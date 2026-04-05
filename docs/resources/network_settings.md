---
page_title: "dlink_network_settings"
---

# dlink_network_settings

Manages DHCP and LAN network settings on the D-Link router.

-> This is a singleton resource. Destroying it removes the resource from state only.

~> `ip_address` and `subnet_mask` overlap with `dlink_lan`. If both resources are used, keep the values in sync to avoid drift.

## Example Usage

```hcl
resource "dlink_network_settings" "main" {
  ip_address     = "192.168.0.1"
  subnet_mask    = "255.255.255.0"
  device_name    = "MyRouter"
  ip_range_start = 100
  ip_range_end   = 200
  lease_time     = 10080
  dns_relay      = true
}
```

## Argument Reference

- `ip_address` (required) - Router LAN IP address.
- `subnet_mask` (optional) - LAN subnet mask. Defaults to `"255.255.255.0"`.
- `device_name` (optional) - Router hostname (e.g. `"R15-D105"`).
- `local_domain_name` (optional) - Local domain name suffix for DHCP clients.
- `ip_range_start` (optional) - Last octet of the DHCP range start. Defaults to `1`.
- `ip_range_end` (optional) - Last octet of the DHCP range end. Defaults to `254`.
- `lease_time` (optional) - DHCP lease time in minutes. Defaults to `10080` (7 days).
- `broadcast` (optional) - Whether broadcast is enabled. Defaults to `false`.
- `dns_relay` (optional) - Whether DNS relay is enabled. Defaults to `true`.

## Attribute Reference

- `id` - Always `"network_settings"`.
- `mac_address` - Router LAN MAC address (read-only).
