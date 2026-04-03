---
page_title: "dlink_wifi"
---

# dlink_wifi

Manages WiFi settings for a single radio band on the D-Link router.

~> Deleting this resource disables the radio band rather than removing configuration, since WiFi cannot be deleted from the router.

~> This router firmware does not support reading back WiFi settings. The Terraform state is used as the source of truth after the initial apply.

## Example Usage

```hcl
resource "dlink_wifi" "wifi_24" {
  band          = "2.4GHz"
  ssid          = "MyNetwork"
  password      = var.wifi_password
  channel       = 6
  security_mode = "WPA2-PSK"
}

resource "dlink_wifi" "wifi_5" {
  band          = "5GHz"
  ssid          = "MyNetwork_5G"
  password      = var.wifi_password
  channel       = 36
  security_mode = "WPA3"
}
```

## Argument Reference

- `band` (required) - Radio band to configure. Must be `"2.4GHz"` or `"5GHz"`.
- `ssid` (required) - WiFi network name (SSID).
- `password` (required, sensitive) - WiFi password (pre-shared key).
- `channel` (optional) - WiFi channel. Use `0` for auto. Defaults to `0`.
- `enabled` (optional) - Whether the radio is enabled. Defaults to `true`.
- `security_mode` (optional) - Security mode, e.g. `"WPA2-PSK"` or `"WPA3"`. Defaults to `"WPA2-PSK"`.

## Attribute Reference

- `id` - Same as `band`.
