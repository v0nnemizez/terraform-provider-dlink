package client

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const withoutLoginKey = "withoutloginkey"

// Client holds the HTTP client and session state for the D-Link DHMAPI SOAP API.
type Client struct {
	BaseURL    string
	Username   string
	password   string
	httpClient *http.Client
	privateKey string     // HMAC key for API-AUTH; set from Login.js PrivateKey = HMAC(PublicKey+pass, challenge)
	uid        string     // uid cookie value; set from Cookie field in challenge response
	pfMu       sync.Mutex // serialises Get+Set on the shared port-forwarding list
	fwMu       sync.Mutex // serialises Get+Set on the shared firewall rule list
}

// NewClient creates a new D-Link API client using the default /DHMAPI/ path.
func NewClient(host, username, password string) *Client {
	return NewClientWithEndpoint(
		fmt.Sprintf("http://%s/DHMAPI/", host),
		username,
		password,
	)
}

// NewClientWithEndpoint creates a client with a fully specified base URL.
func NewClientWithEndpoint(baseURL, username, password string) *Client {
	return &Client{
		BaseURL:    baseURL,
		Username:   username,
		password:   password,
		privateKey: withoutLoginKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// --- SOAP envelope helpers ---

type soapEnvelope struct {
	XMLName xml.Name `xml:"soap:Envelope"`
	XSI     string   `xml:"xmlns:xsi,attr"`
	XSD     string   `xml:"xmlns:xsd,attr"`
	SOAP    string   `xml:"xmlns:soap,attr"`
	Body    soapBody
}

type soapBody struct {
	XMLName xml.Name `xml:"soap:Body"`
	Content interface{}
}

type rawEnvelope struct {
	XMLName xml.Name `xml:"soap:Envelope"`
	XSI     string   `xml:"xmlns:xsi,attr"`
	XSD     string   `xml:"xmlns:xsd,attr"`
	SOAP    string   `xml:"xmlns:soap,attr"`
	Body    rawBody
}

type rawBody struct {
	XMLName xml.Name `xml:"soap:Body"`
	Content []byte   `xml:",innerxml"`
}

// --- Auth helpers ---

// apiAuth computes: HMAC-SHA256(privateKey, timestampMs+action) + " " + timestampMs
func apiAuth(privateKey, action string) string {
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	mac := hmac.New(sha256.New, []byte(privateKey))
	mac.Write([]byte(ts + action))
	token := strings.ToUpper(fmt.Sprintf("%x", mac.Sum(nil)))
	return token + " " + ts
}

// apiContent computes the API-CONTENT header per comm.js / SOAPAction.js.
// For authenticated calls that are not Login or GetDeviceSettings, it returns
// AES-256-CTR(MD5(body)) formatted as "CIPHERTEXT_HEX IV_HEX" (both uppercase).
// Returns "null" for pre-login, Login, and GetDeviceSettings calls.
func apiContent(body []byte, privateKey, action string) string {
	if privateKey == withoutLoginKey || action == "Login" || action == "GetDeviceSettings" {
		return "null"
	}

	// MD5 of the raw request body, uppercase hex — matches CryptoJS.MD5(f).toString().toUpperCase()
	sum := md5.Sum(body)
	md5Hex := strings.ToUpper(hex.EncodeToString(sum[:]))

	// PKCS7-pad the MD5 hex string (32 ASCII bytes → padded to 48 bytes, one full extra block)
	plaintext := []byte(md5Hex)
	padLen := 16 - (len(plaintext) % 16)
	plaintext = append(plaintext, bytes.Repeat([]byte{byte(padLen)}, padLen)...)

	// Decode PrivateKey (64 hex chars → 32 bytes AES-256 key)
	key, err := hex.DecodeString(privateKey)
	if err != nil || len(key) != 32 {
		return "null"
	}

	// Random 16-byte IV
	iv := make([]byte, 16)
	if _, err := rand.Read(iv); err != nil {
		return "null"
	}

	// AES-256-CTR encrypt
	block, err := aes.NewCipher(key)
	if err != nil {
		return "null"
	}
	ciphertext := make([]byte, len(plaintext))
	cipher.NewCTR(block, iv).XORKeyStream(ciphertext, plaintext)

	return strings.ToUpper(hex.EncodeToString(ciphertext)) + " " + strings.ToUpper(hex.EncodeToString(iv))
}

// privateKeyFromChallenge computes the session private key per Login.js:
//   PrivateKey = HMAC-SHA256(key=PublicKey+password, msg=Challenge).toUpperCase()
func privateKeyFromChallenge(publicKey, password, challenge string) string {
	key := publicKey + password
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(challenge))
	return strings.ToUpper(fmt.Sprintf("%x", mac.Sum(nil)))
}

// loginPasswordFromChallenge computes the login password per Login.js:
//   LoginPassword = HMAC-SHA256(key=PrivateKey, msg=Challenge).toUpperCase()
func loginPasswordFromChallenge(privateKey, challenge string) string {
	mac := hmac.New(sha256.New, []byte(privateKey))
	mac.Write([]byte(challenge))
	return strings.ToUpper(fmt.Sprintf("%x", mac.Sum(nil)))
}

// --- Login ---

type loginRequest struct {
	XMLName       xml.Name `xml:"Login"`
	Action        string   `xml:"Action"`
	Username      string   `xml:"Username"`
	LoginPassword string   `xml:"LoginPassword"`
	Captcha       string   `xml:"Captcha"`
}

type challengeResponse struct {
	Challenge string `xml:"Challenge"`
	Cookie    string `xml:"Cookie"`
	PublicKey string `xml:"PublicKey"`
	Result    string `xml:"Result"`
}

type loginResponse struct {
	Result string `xml:"LoginResult"`
	Cookie string `xml:"Cookie"` // some firmware versions include this
}

// Login performs the two-step DHMAPI authentication per Login.js:
//  1. Request challenge → get Challenge, Cookie, PublicKey
//  2. PrivateKey = HMAC-SHA256(key=PublicKey+password, msg=Challenge)
//  3. LoginPassword = HMAC-SHA256(key=PrivateKey, msg=Challenge)
//  4. Send login request with LoginPassword; use uid cookie=Cookie from step 1
func (c *Client) Login() error {
	challenge, cookie, publicKey, err := c.requestChallenge()
	if err != nil {
		return fmt.Errorf("challenge request failed: %w", err)
	}

	// Store uid cookie (Cookie field) — sent with all subsequent requests per SOAPAction.js.
	c.uid = cookie

	// Derive session private key per Login.js.
	privKey := privateKeyFromChallenge(publicKey, c.password, challenge)
	c.privateKey = privKey

	loginPwd := loginPasswordFromChallenge(privKey, challenge)
	if err := c.doLogin(loginPwd); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	return nil
}

// requestChallenge performs the first login step and returns (challenge, cookie, publicKey).
func (c *Client) requestChallenge() (string, string, string, error) {
	req := loginRequest{
		Action:   "request",
		Username: c.Username,
	}

	env := soapEnvelope{
		XSI:  "http://www.w3.org/2001/XMLSchema-instance",
		XSD:  "http://www.w3.org/2001/XMLSchema",
		SOAP: "http://schemas.xmlsoap.org/soap/envelope/",
		Body: soapBody{Content: req},
	}

	respBody, err := c.post("Login", env)
	if err != nil {
		return "", "", "", err
	}

	var env2 struct {
		Body struct {
			Login challengeResponse `xml:"LoginResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(respBody, &env2); err != nil {
		return "", "", "", fmt.Errorf("parse challenge response: %w (body: %s)", err, string(respBody))
	}

	ch := env2.Body.Login
	if ch.Challenge == "" {
		return "", "", "", fmt.Errorf("empty challenge in response (body: %s)", string(respBody))
	}
	return ch.Challenge, ch.Cookie, ch.PublicKey, nil
}

// doLogin performs the second login step.
func (c *Client) doLogin(hashedPassword string) error {
	req := loginRequest{
		Action:        "login",
		Username:      c.Username,
		LoginPassword: strings.ToUpper(hashedPassword),
	}

	env := soapEnvelope{
		XSI:  "http://www.w3.org/2001/XMLSchema-instance",
		XSD:  "http://www.w3.org/2001/XMLSchema",
		SOAP: "http://schemas.xmlsoap.org/soap/envelope/",
		Body: soapBody{Content: req},
	}

	respBody, err := c.post("Login", env)
	if err != nil {
		return err
	}

	var env2 struct {
		Body struct {
			Login loginResponse `xml:"LoginResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(respBody, &env2); err != nil {
		return fmt.Errorf("parse login response: %w (body: %s)", err, string(respBody))
	}

	result := env2.Body.Login.Result
	if !strings.EqualFold(result, "ok") && !strings.EqualFold(result, "success") {
		return fmt.Errorf("login rejected by router: result=%q (body: %s)", result, string(respBody))
	}

	// Some firmware variants return a new cookie/private key in the login response.
	if env2.Body.Login.Cookie != "" {
		c.privateKey = env2.Body.Login.Cookie
	}

	return nil
}

// --- HTTP transport ---

// post marshals the envelope, adds DHMAPI auth headers, and returns the raw response body.
func (c *Client) post(action string, body interface{}) ([]byte, error) {
	var xmlBytes []byte
	var err error

	switch v := body.(type) {
	case rawEnvelope:
		xmlBytes, err = xml.Marshal(v)
	default:
		xmlBytes, err = xml.Marshal(body)
	}
	if err != nil {
		return nil, fmt.Errorf("marshal SOAP: %w", err)
	}

	payload := append([]byte(`<?xml version="1.0" encoding="utf-8"?>`), xmlBytes...)

	req, err := http.NewRequest("POST", c.BaseURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml; charset=UTF-8")
	req.Header.Set("API-ACTION", action)
	req.Header.Set("API-AUTH", apiAuth(c.privateKey, action))
	req.Header.Set("API-CONTENT", apiContent(payload, c.privateKey, action))
	if c.uid != "" {
		req.Header.Set("Cookie", "uid="+c.uid)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP POST to %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d for %s (action=%s): %s",
			resp.StatusCode, c.BaseURL, action, string(respBytes))
	}

	return respBytes, nil
}

// --- Generic Get/Set ---

// GetAction sends a SOAP Get request and returns the raw response XML.
func (c *Client) GetAction(action string) ([]byte, error) {
	innerXML := []byte(fmt.Sprintf(`<%s></%s>`, action, action))
	env := rawEnvelope{
		XSI:  "http://www.w3.org/2001/XMLSchema-instance",
		XSD:  "http://www.w3.org/2001/XMLSchema",
		SOAP: "http://schemas.xmlsoap.org/soap/envelope/",
		Body: rawBody{Content: innerXML},
	}
	return c.post(action, env)
}

// SetMultipleActions sends a SetMultipleActions SOAP request with the given inner XML.
func (c *Client) SetMultipleActions(actionsInnerXML string) ([]byte, error) {
	return c.postRaw("SetMultipleActions", []byte(actionsInnerXML))
}

func (c *Client) postRaw(action string, innerXML []byte) ([]byte, error) {
	env := rawEnvelope{
		XSI:  "http://www.w3.org/2001/XMLSchema-instance",
		XSD:  "http://www.w3.org/2001/XMLSchema",
		SOAP: "http://schemas.xmlsoap.org/soap/envelope/",
		Body: rawBody{Content: innerXML},
	}
	return c.post(action, env)
}

// --- WiFi ---

// WifiSettings represents WiFi radio configuration.
type WifiSettings struct {
	Band         string
	SSID         string
	Channel      int
	Enabled      bool
	SecurityMode string
	Password     string
}

// GetWifiSettings fetches radio settings and security for the given band.
// Returns (nil, nil) if the router returns no data (firmware does not support read-back).
func (c *Client) GetWifiSettings(band string) (*WifiSettings, error) {
	radioResp, err := c.GetAction("GetWLanRadioSettings")
	if err != nil {
		return nil, fmt.Errorf("GetWLanRadioSettings: %w", err)
	}
	if len(bytes.TrimSpace(radioResp)) == 0 {
		return nil, nil
	}

	secResp, err := c.GetAction("GetWLanRadioSecurity")
	if err != nil {
		return nil, fmt.Errorf("GetWLanRadioSecurity: %w", err)
	}

	return parseWifiSettings(band, radioResp, secResp)
}

type wlanRadioSettingsResponse struct {
	Radios []wlanRadio `xml:"WLanRadioSettings>Radio"`
}

type wlanRadio struct {
	Band    string `xml:"Band"`
	SSID    string `xml:"SSID"`
	Channel int    `xml:"Channel"`
	Enabled string `xml:"Enabled"`
}

type wlanSecurityResponse struct {
	Radios []wlanRadioSecurity `xml:"WLanRadioSecurity>Radio"`
}

type wlanRadioSecurity struct {
	Band         string `xml:"Band"`
	SecurityMode string `xml:"SecurityMode"`
	PreSharedKey string `xml:"PreSharedKey"`
}

func parseWifiSettings(band string, radioXML, secXML []byte) (*WifiSettings, error) {
	var radioEnv struct {
		Body struct {
			Inner wlanRadioSettingsResponse `xml:"GetWLanRadioSettingsResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(radioXML, &radioEnv); err != nil {
		return nil, fmt.Errorf("parse radio settings: %w", err)
	}

	var secEnv struct {
		Body struct {
			Inner wlanSecurityResponse `xml:"GetWLanRadioSecurityResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(secXML, &secEnv); err != nil {
		return nil, fmt.Errorf("parse security settings: %w", err)
	}

	settings := &WifiSettings{Band: band}

	for _, r := range radioEnv.Body.Inner.Radios {
		if strings.EqualFold(r.Band, band) {
			settings.SSID = r.SSID
			settings.Channel = r.Channel
			settings.Enabled = strings.EqualFold(r.Enabled, "true") || r.Enabled == "1"
		}
	}
	for _, s := range secEnv.Body.Inner.Radios {
		if strings.EqualFold(s.Band, band) {
			settings.SecurityMode = s.SecurityMode
			settings.Password = s.PreSharedKey
		}
	}

	return settings, nil
}

// SetWifiSettings applies WiFi radio and security settings for one band.
func (c *Client) SetWifiSettings(s *WifiSettings) error {
	enabled := "false"
	if s.Enabled {
		enabled = "true"
	}

	actionsXML := fmt.Sprintf(`<SetMultipleActions>
  <Actions>
    <Action>
      <Name>SetWLanRadioSettings</Name>
      <Parameters>
        <Radio>
          <Band>%s</Band>
          <SSID>%s</SSID>
          <Channel>%d</Channel>
          <Enabled>%s</Enabled>
        </Radio>
      </Parameters>
    </Action>
    <Action>
      <Name>SetWLanRadioSecurity</Name>
      <Parameters>
        <Radio>
          <Band>%s</Band>
          <SecurityMode>%s</SecurityMode>
          <PreSharedKey>%s</PreSharedKey>
        </Radio>
      </Parameters>
    </Action>
  </Actions>
</SetMultipleActions>`,
		s.Band, s.SSID, s.Channel, enabled,
		s.Band, s.SecurityMode, s.Password)

	_, err := c.SetMultipleActions(actionsXML)
	return err
}

// --- Port Forwarding ---

// PortForwardRule represents a single port forwarding rule.
// Note: on this router, external and internal ports are always the same (Port field).
// Protocol "TCP" sets TCPPorts, "UDP" sets UDPPorts, "TCP/UDP" sets both.
type PortForwardRule struct {
	Name     string
	Protocol string // "TCP", "UDP", or "TCP/UDP"
	Port     int    // same port on WAN and LAN sides
	LocalIP  string
	Schedule string // default "Always"
	Enabled  bool
}

type portForwardingResponse struct {
	Rules []portForwardXML `xml:"PortForwardingList>PortForwardingInfo"`
}

type portForwardXML struct {
	Enabled     string `xml:"Enabled"`
	Description string `xml:"PortForwardingDescription"`
	TCPPorts    string `xml:"TCPPorts"`
	UDPPorts    string `xml:"UDPPorts"`
	LocalIP     string `xml:"LocalIPAddress"`
	Schedule    string `xml:"ScheduleName"`
}

// GetPortForwardingRules returns all port forwarding rules.
func (c *Client) GetPortForwardingRules() ([]PortForwardRule, error) {
	respXML, err := c.GetAction("GetPortForwardingSettings")
	if err != nil {
		return nil, err
	}

	if len(bytes.TrimSpace(respXML)) == 0 {
		return []PortForwardRule{}, nil
	}

	var env struct {
		Body struct {
			Inner portForwardingResponse `xml:"GetPortForwardingSettingsResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(respXML, &env); err != nil {
		return nil, fmt.Errorf("parse port forwarding: %w (body: %s)", err, string(respXML))
	}

	rules := make([]PortForwardRule, 0, len(env.Body.Inner.Rules))
	for _, r := range env.Body.Inner.Rules {
		protocol, port := protocolAndPortFromXML(r.TCPPorts, r.UDPPorts)
		schedule := r.Schedule
		if schedule == "" {
			schedule = "Always"
		}
		rules = append(rules, PortForwardRule{
			Name:     r.Description,
			Protocol: protocol,
			Port:     port,
			LocalIP:  r.LocalIP,
			Schedule: schedule,
			Enabled:  strings.EqualFold(r.Enabled, "true") || r.Enabled == "1",
		})
	}
	return rules, nil
}

// protocolAndPortFromXML derives Protocol and Port from TCPPorts/UDPPorts XML fields.
func protocolAndPortFromXML(tcpPorts, udpPorts string) (protocol string, port int) {
	tcp := strings.TrimSpace(tcpPorts)
	udp := strings.TrimSpace(udpPorts)
	portStr := tcp
	if portStr == "" {
		portStr = udp
	}
	fmt.Sscanf(portStr, "%d", &port)
	switch {
	case tcp != "" && udp != "":
		protocol = "TCP/UDP"
	case udp != "":
		protocol = "UDP"
	default:
		protocol = "TCP"
	}
	return
}

// --- Parental Control ---

// ParentalProfile represents a parental control profile.
type ParentalProfile struct {
	UUID            string
	Name            string
	FilterEnabled   bool
	BlockedDomains  []BlockedDomain
	AllowSlowAccess bool
	Devices         []string // MAC addresses assigned to this profile
}

// BlockedDomain represents a single blocked domain entry with a display title.
type BlockedDomain struct {
	Title  string
	Domain string
}

type titleValueXML struct {
	Title string `xml:"Title"`
	Value string `xml:"Value"`
}

type parentalProfileXML struct {
	UUID    string   `xml:"UUID"`
	Name    string   `xml:"Name"`
	Devices []string `xml:"Devices>string"`
	ProfileFilter struct {
		Enabled        string          `xml:"Enabled"`
		BlockedDomains []titleValueXML `xml:"BlockedDomains>TitleValue"`
	} `xml:"ProfileFilter"`
	AllowSlowAccess string `xml:"AllowSlowAccess"`
}

type profileListResponse struct {
	Profiles []parentalProfileXML `xml:"ProfileList>ParentalProfile"`
}

// GetParentalProfiles returns all user-defined parental control profiles.
// Profiles with the built-in UUID 00000000-0000-0000-0000-000000000000 are skipped.
func (c *Client) GetParentalProfiles() ([]ParentalProfile, error) {
	respXML, err := c.GetAction("GetProfileListSettingsv1Lite")
	if err != nil {
		return nil, err
	}

	if len(bytes.TrimSpace(respXML)) == 0 {
		return []ParentalProfile{}, nil
	}

	var env struct {
		Body struct {
			Inner profileListResponse `xml:"GetProfileListSettingsv1LiteResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(respXML, &env); err != nil {
		return nil, fmt.Errorf("parse parental profiles: %w (body: %s)", err, string(respXML))
	}

	profiles := make([]ParentalProfile, 0, len(env.Body.Inner.Profiles))
	for _, p := range env.Body.Inner.Profiles {
		if p.UUID == "00000000-0000-0000-0000-000000000000" {
			continue
		}
		domains := make([]BlockedDomain, 0, len(p.ProfileFilter.BlockedDomains))
		for _, tv := range p.ProfileFilter.BlockedDomains {
			domains = append(domains, BlockedDomain{Title: tv.Title, Domain: tv.Value})
		}
		profiles = append(profiles, ParentalProfile{
			UUID:            p.UUID,
			Name:            p.Name,
			FilterEnabled:   strings.EqualFold(p.ProfileFilter.Enabled, "true"),
			BlockedDomains:  domains,
			AllowSlowAccess: strings.EqualFold(p.AllowSlowAccess, "true"),
			Devices:         p.Devices,
		})
	}
	return profiles, nil
}

// FindParentalProfileUUIDByName returns the UUID of the first profile whose
// Name matches (case-insensitive), or "" if not found.
func (c *Client) FindParentalProfileUUIDByName(name string) (string, error) {
	profiles, err := c.GetParentalProfiles()
	if err != nil {
		return "", err
	}
	for _, p := range profiles {
		if strings.EqualFold(p.Name, name) {
			return p.UUID, nil
		}
	}
	return "", nil
}

// SetParentalProfile creates, updates, or deletes a parental control profile.
// Pass delete=true to remove the profile from the router.
func (c *Client) SetParentalProfile(profile ParentalProfile, delete bool) error {
	filterEnabled := "false"
	if profile.FilterEnabled {
		filterEnabled = "true"
	}
	allowSlowAccess := "false"
	if profile.AllowSlowAccess {
		allowSlowAccess = "true"
	}
	deleteStr := "false"
	if delete {
		deleteStr = "true"
	}

	var sb strings.Builder
	sb.WriteString("<SetProfileSettingsv1Lite><ModifiedProfiles><ParentalProfile>")
	fmt.Fprintf(&sb, "<UUID>%s</UUID>", profile.UUID)
	fmt.Fprintf(&sb, "<Name>%s</Name>", profile.Name)
	sb.WriteString("<Devices>")
	for _, mac := range profile.Devices {
		fmt.Fprintf(&sb, "<string>%s</string>", mac)
	}
	sb.WriteString("</Devices>")
	sb.WriteString("<ProfileFilter>")
	fmt.Fprintf(&sb, "<Enabled>%s</Enabled>", filterEnabled)
	sb.WriteString("<BlockedDomains>")
	for _, d := range profile.BlockedDomains {
		fmt.Fprintf(&sb, "<TitleValue><Title>%s</Title><Value>%s</Value></TitleValue>", d.Title, d.Domain)
	}
	sb.WriteString("</BlockedDomains>")
	sb.WriteString("</ProfileFilter>")
	sb.WriteString("<Schedules></Schedules>")
	fmt.Fprintf(&sb, "<Delete>%s</Delete>", deleteStr)
	fmt.Fprintf(&sb, "<AllowSlowAccess>%s</AllowSlowAccess>", allowSlowAccess)
	sb.WriteString("</ParentalProfile></ModifiedProfiles></SetProfileSettingsv1Lite>")

	_, err := c.postRaw("SetProfileSettingsv1Lite", []byte(sb.String()))
	return err
}

// ModifyPortForwardingRules runs fn while holding the port-forwarding mutex,
// giving fn the current rule list and applying whatever it returns.
// Use this for Create/Update/Delete to avoid concurrent Get+Set races.
func (c *Client) ModifyPortForwardingRules(fn func([]PortForwardRule) ([]PortForwardRule, error)) error {
	c.pfMu.Lock()
	defer c.pfMu.Unlock()

	rules, err := c.GetPortForwardingRules()
	if err != nil {
		return err
	}
	updated, err := fn(rules)
	if err != nil {
		return err
	}
	return c.SetPortForwardingRules(updated)
}

// --- Firewall Rules ---

// FirewallRule represents a single IPv4 firewall rule.
type FirewallRule struct {
	Name         string
	Enabled      bool   // Status: "Enable"/"Disable"
	Schedule     string // default "Always"
	SrcInterface string // "WAN" or "LAN"
	SrcIPStart   string
	SrcIPEnd     string
	DstInterface string // "LAN" or "WAN"
	DstIPStart   string
	DstIPEnd     string
	Protocol     string // "TCP", "UDP", "TCP/UDP", "ICMP", etc.
	PortStart    int
	PortEnd      int
}

type firewallResponse struct {
	FirewallStatus string           `xml:"IPv4_FirewallStatus"`
	Rules          []firewallRuleXML `xml:"IPv4FirewallRuleLists>IPv4FirewallRule"`
}

type firewallRuleXML struct {
	Name         string `xml:"Name"`
	Status       string `xml:"Status"`
	Schedule     string `xml:"Schedule"`
	SrcInterface string `xml:"SrcInterface"`
	SrcIPStart   string `xml:"SrcIPv4AddressRangeStart"`
	SrcIPEnd     string `xml:"SrcIPv4AddressRangeEnd"`
	DstInterface string `xml:"DestInterface"`
	DstIPStart   string `xml:"DestIPv4AddressRangeStart"`
	DstIPEnd     string `xml:"DestIPv4AddressRangeEnd"`
	Protocol     string `xml:"Protocol"`
	PortStart    int    `xml:"PortRangeStart"`
	PortEnd      int    `xml:"PortRangeEnd"`
}

// GetFirewallRules returns all IPv4 firewall rules and the global firewall status string.
func (c *Client) GetFirewallRules() ([]FirewallRule, string, error) {
	respXML, err := c.GetAction("GetIPv4FirewallSettings")
	if err != nil {
		return nil, "", err
	}

	if len(bytes.TrimSpace(respXML)) == 0 {
		return []FirewallRule{}, "", nil
	}

	var env struct {
		Body struct {
			Inner firewallResponse `xml:"GetIPv4FirewallSettingsResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(respXML, &env); err != nil {
		return nil, "", fmt.Errorf("parse firewall rules: %w (body: %s)", err, string(respXML))
	}

	inner := env.Body.Inner
	rules := make([]FirewallRule, 0, len(inner.Rules))
	for _, r := range inner.Rules {
		schedule := r.Schedule
		if schedule == "" {
			schedule = "Always"
		}
		rules = append(rules, FirewallRule{
			Name:         r.Name,
			Enabled:      strings.EqualFold(r.Status, "enable"),
			Schedule:     schedule,
			SrcInterface: r.SrcInterface,
			SrcIPStart:   r.SrcIPStart,
			SrcIPEnd:     r.SrcIPEnd,
			DstInterface: r.DstInterface,
			DstIPStart:   r.DstIPStart,
			DstIPEnd:     r.DstIPEnd,
			Protocol:     r.Protocol,
			PortStart:    r.PortStart,
			PortEnd:      r.PortEnd,
		})
	}
	return rules, inner.FirewallStatus, nil
}

// SetFirewallRules replaces all IPv4 firewall rules, preserving the global firewall status.
func (c *Client) SetFirewallRules(rules []FirewallRule, firewallStatus string) error {
	var sb strings.Builder
	sb.WriteString("<SetIPv4FirewallSettings>")
	fmt.Fprintf(&sb, "<IPv4_FirewallStatus>%s</IPv4_FirewallStatus>", firewallStatus)
	sb.WriteString("<IPv4FirewallRuleLists>")
	for _, r := range rules {
		status := "Disable"
		if r.Enabled {
			status = "Enable"
		}
		schedule := r.Schedule
		if schedule == "" {
			schedule = "Always"
		}
		fmt.Fprintf(&sb,
			"<IPv4FirewallRule>"+
				"<Name>%s</Name>"+
				"<Status>%s</Status>"+
				"<Schedule>%s</Schedule>"+
				"<SrcInterface>%s</SrcInterface>"+
				"<SrcIPv4AddressRangeStart>%s</SrcIPv4AddressRangeStart>"+
				"<SrcIPv4AddressRangeEnd>%s</SrcIPv4AddressRangeEnd>"+
				"<DestInterface>%s</DestInterface>"+
				"<DestIPv4AddressRangeStart>%s</DestIPv4AddressRangeStart>"+
				"<DestIPv4AddressRangeEnd>%s</DestIPv4AddressRangeEnd>"+
				"<Protocol>%s</Protocol>"+
				"<PortRangeStart>%d</PortRangeStart>"+
				"<PortRangeEnd>%d</PortRangeEnd>"+
				"</IPv4FirewallRule>",
			r.Name, status, schedule,
			r.SrcInterface, r.SrcIPStart, r.SrcIPEnd,
			r.DstInterface, r.DstIPStart, r.DstIPEnd,
			r.Protocol, r.PortStart, r.PortEnd)
	}
	sb.WriteString("</IPv4FirewallRuleLists>")
	sb.WriteString("</SetIPv4FirewallSettings>")

	_, err := c.postRaw("SetIPv4FirewallSettings", []byte(sb.String()))
	return err
}

// ModifyFirewallRules runs fn while holding the firewall mutex,
// giving fn the current rule list and applying whatever it returns.
// The global firewall status is preserved across the read-modify-write cycle.
func (c *Client) ModifyFirewallRules(fn func([]FirewallRule) ([]FirewallRule, error)) error {
	c.fwMu.Lock()
	defer c.fwMu.Unlock()

	rules, firewallStatus, err := c.GetFirewallRules()
	if err != nil {
		return err
	}
	updated, err := fn(rules)
	if err != nil {
		return err
	}
	return c.SetFirewallRules(updated, firewallStatus)
}

// SetPortForwardingRules replaces all port forwarding rules.
func (c *Client) SetPortForwardingRules(rules []PortForwardRule) error {
	var sb strings.Builder
	sb.WriteString("<SetPortForwardingSettings><PortForwardingList>")
	for _, r := range rules {
		enabled := "false"
		if r.Enabled {
			enabled = "true"
		}
		schedule := r.Schedule
		if schedule == "" {
			schedule = "Always"
		}
		tcpPorts, udpPorts := "", ""
		switch strings.ToUpper(r.Protocol) {
		case "UDP":
			udpPorts = fmt.Sprintf("%d", r.Port)
		case "TCP/UDP", "BOTH":
			tcpPorts = fmt.Sprintf("%d", r.Port)
			udpPorts = fmt.Sprintf("%d", r.Port)
		default: // TCP
			tcpPorts = fmt.Sprintf("%d", r.Port)
		}
		fmt.Fprintf(&sb,
			"<PortForwardingInfo>"+
				"<Enabled>%s</Enabled>"+
				"<PortForwardingDescription>%s</PortForwardingDescription>"+
				"<TCPPorts>%s</TCPPorts>"+
				"<UDPPorts>%s</UDPPorts>"+
				"<LocalIPAddress>%s</LocalIPAddress>"+
				"<ScheduleName>%s</ScheduleName>"+
				"</PortForwardingInfo>",
			enabled, r.Name, tcpPorts, udpPorts, r.LocalIP, schedule)
	}
	sb.WriteString("</PortForwardingList></SetPortForwardingSettings>")

	_, err := c.postRaw("SetPortForwardingSettings", []byte(sb.String()))
	return err
}
