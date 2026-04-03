package client

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- privateKeyFromChallenge / loginPasswordFromChallenge ---

func TestPrivateKeyFromChallenge(t *testing.T) {
	result := privateKeyFromChallenge("pubkey", "password", "challenge")
	if len(result) != 64 {
		t.Errorf("expected 64-char hex, got %d chars", len(result))
	}
	if result != privateKeyFromChallenge("pubkey", "password", "challenge") {
		t.Error("not deterministic")
	}
	if result == privateKeyFromChallenge("pubkey", "other", "challenge") {
		t.Error("different passwords should produce different keys")
	}
	if result == privateKeyFromChallenge("pubkey", "password", "other") {
		t.Error("different challenges should produce different keys")
	}
}

func TestLoginPasswordFromChallenge(t *testing.T) {
	privKey := privateKeyFromChallenge("pub", "pass", "ch")
	lp := loginPasswordFromChallenge(privKey, "ch")
	if len(lp) != 64 {
		t.Errorf("expected 64-char hex, got %d", len(lp))
	}
	if lp == privKey {
		t.Error("login password should differ from private key")
	}
}

// --- apiAuth ---

func TestApiAuth_Format(t *testing.T) {
	auth := apiAuth("withoutloginkey", "Login")
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 {
		t.Fatalf("expected 'TOKEN TS', got: %q", auth)
	}
	if len(parts[0]) != 64 {
		t.Errorf("token should be 64 hex chars, got %d", len(parts[0]))
	}
	if len(parts[1]) < 10 {
		t.Errorf("timestamp too short: %q", parts[1])
	}
}

func TestApiAuth_DifferentActionsProduceDifferentTokens(t *testing.T) {
	// Same key, same timestamp not possible to force, but different actions → different tokens
	a1 := apiAuth("key", "Login")
	a2 := apiAuth("key", "GetDeviceSettings")
	if strings.Split(a1, " ")[0] == strings.Split(a2, " ")[0] {
		t.Error("different actions should produce different tokens")
	}
}

func TestApiAuth_DifferentKeysProduceDifferentTokens(t *testing.T) {
	a1 := apiAuth("key1", "Login")
	a2 := apiAuth("key2", "Login")
	if strings.Split(a1, " ")[0] == strings.Split(a2, " ")[0] {
		t.Error("different keys should produce different tokens")
	}
}

// --- Login flow ---

func TestLogin_Success(t *testing.T) {
	const (
		testChallenge = "testchallenge"
		testCookie    = "sessionkey123"
		testPublicKey = "pubkey"
	)
	// Pre-compute expected privateKey: HMAC-SHA256(key=pubkey+password, msg=testchallenge)
	expectedPrivKey := privateKeyFromChallenge(testPublicKey, "password", testChallenge)

	callCount := 0
	var call2Cookie string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		action := r.Header.Get("API-ACTION")
		switch callCount {
		case 1:
			if action != "Login" {
				t.Errorf("call 1: expected API-ACTION=Login, got %q", action)
			}
			fmt.Fprint(w, soapWrap(`<LoginResponse><Challenge>`+testChallenge+`</Challenge><Cookie>`+testCookie+`</Cookie><PublicKey>`+testPublicKey+`</PublicKey><LoginResult>OK</LoginResult></LoginResponse>`))
		case 2:
			if action != "Login" {
				t.Errorf("call 2: expected API-ACTION=Login, got %q", action)
			}
			call2Cookie = r.Header.Get("Cookie")
			fmt.Fprint(w, soapWrap(`<LoginResponse><LoginResult>OK</LoginResult></LoginResponse>`))
		}
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	if err := c.Login(); err != nil {
		t.Fatalf("Login() error: %v", err)
	}
	if c.uid != testCookie {
		t.Errorf("expected uid=%q, got %q", testCookie, c.uid)
	}
	if c.privateKey != expectedPrivKey {
		t.Errorf("expected privateKey=%q, got %q", expectedPrivKey, c.privateKey)
	}
	if call2Cookie != "uid="+testCookie {
		t.Errorf("expected Cookie header 'uid=%s' on login request, got %q", testCookie, call2Cookie)
	}
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls, got %d", callCount)
	}
}

func TestLogin_EmptyChallenge(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, soapWrap(`<LoginResponse><Challenge></Challenge><LoginResult>OK</LoginResult></LoginResponse>`))
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Login()
	if err == nil || !strings.Contains(err.Error(), "empty challenge") {
		t.Errorf("expected 'empty challenge' error, got: %v", err)
	}
}

func TestLogin_Rejected(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch callCount {
		case 1:
			fmt.Fprint(w, soapWrap(`<LoginResponse><Challenge>ch</Challenge><Cookie>key</Cookie><LoginResult>OK</LoginResult></LoginResponse>`))
		case 2:
			fmt.Fprint(w, soapWrap(`<LoginResponse><LoginResult>FAILED</LoginResult></LoginResponse>`))
		}
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Login()
	if err == nil || !strings.Contains(err.Error(), "FAILED") {
		t.Errorf("expected FAILED error, got: %v", err)
	}
}

// --- Request headers ---

func TestRequestHeaders(t *testing.T) {
	var gotAction, gotAuth, gotContent string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAction = r.Header.Get("API-ACTION")
		gotAuth = r.Header.Get("API-AUTH")
		gotContent = r.Header.Get("API-CONTENT")
		fmt.Fprint(w, soapWrap(`<GetWLanRadioSettingsResponse></GetWLanRadioSettingsResponse>`))
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	c.privateKey = "testkey"
	_, _ = c.GetAction("GetWLanRadioSettings")

	if gotAction != "GetWLanRadioSettings" {
		t.Errorf("API-ACTION: got %q, want GetWLanRadioSettings", gotAction)
	}
	if gotAuth == "" {
		t.Error("API-AUTH header missing")
	}
	if !strings.Contains(gotAuth, " ") {
		t.Errorf("API-AUTH should be 'TOKEN TS', got %q", gotAuth)
	}
	if gotContent != "null" {
		t.Errorf("API-CONTENT: got %q, want null", gotContent)
	}
}

// --- parseWifiSettings ---

func TestParseWifiSettings(t *testing.T) {
	radioXML := soapWrap(`<GetWLanRadioSettingsResponse>
		<WLanRadioSettings>
			<Radio><Band>2.4GHz</Band><SSID>MyNet</SSID><Channel>6</Channel><Enabled>true</Enabled></Radio>
			<Radio><Band>5GHz</Band><SSID>MyNet5</SSID><Channel>36</Channel><Enabled>true</Enabled></Radio>
		</WLanRadioSettings>
	</GetWLanRadioSettingsResponse>`)

	secXML := soapWrap(`<GetWLanRadioSecurityResponse>
		<WLanRadioSecurity>
			<Radio><Band>2.4GHz</Band><SecurityMode>WPA2-PSK</SecurityMode><PreSharedKey>secret123</PreSharedKey></Radio>
			<Radio><Band>5GHz</Band><SecurityMode>WPA3</SecurityMode><PreSharedKey>secret5g</PreSharedKey></Radio>
		</WLanRadioSecurity>
	</GetWLanRadioSecurityResponse>`)

	s, err := parseWifiSettings("2.4GHz", []byte(radioXML), []byte(secXML))
	if err != nil {
		t.Fatalf("parseWifiSettings error: %v", err)
	}
	if s.SSID != "MyNet" {
		t.Errorf("SSID: got %q, want MyNet", s.SSID)
	}
	if s.Channel != 6 {
		t.Errorf("Channel: got %d, want 6", s.Channel)
	}
	if !s.Enabled {
		t.Error("expected Enabled=true")
	}
	if s.SecurityMode != "WPA2-PSK" {
		t.Errorf("SecurityMode: got %q", s.SecurityMode)
	}
	if s.Password != "secret123" {
		t.Errorf("Password: got %q", s.Password)
	}

	s5, err := parseWifiSettings("5GHz", []byte(radioXML), []byte(secXML))
	if err != nil {
		t.Fatalf("5GHz error: %v", err)
	}
	if s5.SSID != "MyNet5" || s5.Password != "secret5g" {
		t.Errorf("5GHz mismatch: %+v", s5)
	}
}

// --- GetPortForwardingRules ---

func TestGetPortForwardingRules_EmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Router returns HTTP 200 with empty body when no rules exist.
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	rules, err := c.GetPortForwardingRules()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected empty slice, got %d rules", len(rules))
	}
}

func TestGetPortForwardingRules(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, soapWrap(`<GetPortForwardingSettingsResponse>
			<PortForwardingList>
				<PortForwardingInfo>
					<Enabled>true</Enabled>
					<PortForwardingDescription>HTTP</PortForwardingDescription>
					<TCPPorts>80</TCPPorts>
					<UDPPorts></UDPPorts>
					<LocalIPAddress>192.168.0.100</LocalIPAddress>
					<ScheduleName>Always</ScheduleName>
				</PortForwardingInfo>
				<PortForwardingInfo>
					<Enabled>false</Enabled>
					<PortForwardingDescription>SSH</PortForwardingDescription>
					<TCPPorts>22</TCPPorts>
					<UDPPorts></UDPPorts>
					<LocalIPAddress>192.168.0.50</LocalIPAddress>
					<ScheduleName>Always</ScheduleName>
				</PortForwardingInfo>
			</PortForwardingList>
		</GetPortForwardingSettingsResponse>`))
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	rules, err := c.GetPortForwardingRules()
	if err != nil {
		t.Fatalf("GetPortForwardingRules error: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	r := rules[0]
	if r.Name != "HTTP" || r.Protocol != "TCP" || r.Port != 80 ||
		r.LocalIP != "192.168.0.100" || !r.Enabled {
		t.Errorf("unexpected rule[0]: %+v", r)
	}
	if rules[1].Enabled {
		t.Error("rule[1] should be disabled")
	}
}

// --- SetPortForwardingRules ---

func TestSetPortForwardingRules_SendsCorrectXML(t *testing.T) {
	var receivedBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		receivedBody = string(b)
		fmt.Fprint(w, soapWrap(`<SetPortForwardingSettingsResponse><SetPortForwardingSettingsResult>OK</SetPortForwardingSettingsResult></SetPortForwardingSettingsResponse>`))
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.SetPortForwardingRules([]PortForwardRule{
		{Name: "Test", Protocol: "TCP", Port: 443, LocalIP: "10.0.0.1", Enabled: true},
	})
	if err != nil {
		t.Fatalf("SetPortForwardingRules error: %v", err)
	}
	for _, want := range []string{"Test", "443", "10.0.0.1", "true", "PortForwardingInfo", "TCPPorts", "Always"} {
		if !strings.Contains(receivedBody, want) {
			t.Errorf("request body missing %q", want)
		}
	}
}

// --- SetWifiSettings ---

func TestSetWifiSettings_SendsCorrectXML(t *testing.T) {
	var receivedBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		receivedBody = string(b)
		fmt.Fprint(w, soapWrap(`<SetMultipleActionsResponse><SetMultipleActionsResult>OK</SetMultipleActionsResult></SetMultipleActionsResponse>`))
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.SetWifiSettings(&WifiSettings{
		Band: "2.4GHz", SSID: "NewSSID", Channel: 11, Enabled: true,
		SecurityMode: "WPA2-PSK", Password: "newpassword",
	})
	if err != nil {
		t.Fatalf("SetWifiSettings error: %v", err)
	}
	for _, want := range []string{"NewSSID", "2.4GHz", "11", "WPA2-PSK", "newpassword"} {
		if !strings.Contains(receivedBody, want) {
			t.Errorf("request body missing %q", want)
		}
	}
}

// --- HTTP error ---

func TestPostSOAP_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.GetAction("GetWLanRadioSettings")
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 error, got: %v", err)
	}
}

// --- helpers ---

func newTestClient(serverURL string) *Client {
	c := NewClientWithEndpoint(serverURL+"/", "Admin", "password")
	return c
}

func soapWrap(inner string) string {
	env := struct {
		XMLName xml.Name `xml:"Envelope"`
		SOAP    string   `xml:"xmlns:soap,attr"`
		Body    struct {
			Inner string `xml:",innerxml"`
		} `xml:"Body"`
	}{SOAP: "http://schemas.xmlsoap.org/soap/envelope/"}
	env.Body.Inner = inner
	b, _ := xml.Marshal(env)
	return string(b)
}
