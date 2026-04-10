---
page_title: "dlink_quick_vpn"
---

# dlink_quick_vpn

Manages the D-Link QuickVPN (L2TP/IPsec) configuration.

-> This is a singleton resource — there can only be one `dlink_quick_vpn` per provider instance. Destroying it disables QuickVPN on the router (`enabled = false`).

~> **Password handling:** `password` and `psk` are encrypted by the provider before being sent to the router, matching the browser's behaviour. The router never returns these values in plaintext, so they are write-only from a drift-detection perspective — changes outside Terraform will not be detected on the password/PSK fields.

## Example Usage

```hcl
resource "dlink_quick_vpn" "main" {
  enabled  = true
  username = "vpnuser"
  password = "s3cr3t!"
  psk      = "MyPreSharedKey!"
}
```

## Argument Reference

- `enabled` (optional) - Whether QuickVPN is enabled. Defaults to `false`.
- `username` (required) - VPN username.
- `password` (required, sensitive) - VPN password. Encrypted before transmission.
- `psk` (required, sensitive) - Pre-Shared Key for L2TP/IPsec authentication. Encrypted before transmission.
- `auth_protocol` (optional) - Authentication protocol. Defaults to `"MSCHAPv2"`.
- `mppe` (optional) - MPPE encryption setting. Defaults to `"None"`.

## Attribute Reference

- `id` - Always `"quick_vpn"`.
