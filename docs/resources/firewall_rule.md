---
page_title: "dlink_firewall_rule"
---

# dlink_firewall_rule

Manages a single IPv4 firewall rule on the D-Link router.

-> The global firewall enable/disable switch is preserved as-is when rules are modified. To toggle it, use the router web UI.

## Example Usage

```hcl
resource "dlink_firewall_rule" "block_inbound" {
  name           = "BlockInbound"
  src_interface  = "WAN"
  src_ip_start   = "80.150.200.102"
  dest_interface = "LAN"
  dest_ip_start  = "192.168.0.100"
  protocol       = "TCP"
  port_start     = 4000
  port_end       = 4500
}
```

## Argument Reference

- `name` (required) - Rule name. Must be unique on the router.
- `enabled` (optional) - Whether the rule is active. Defaults to `true`.
- `schedule` (optional) - Schedule name. Defaults to `"Always"`.
- `src_interface` (optional) - Source interface: `"WAN"` or `"LAN"`. Defaults to `"WAN"`.
- `src_ip_start` (optional) - Start of the source IP address range.
- `src_ip_end` (optional) - End of the source IP address range. Leave empty for a single address.
- `dest_interface` (optional) - Destination interface: `"LAN"` or `"WAN"`. Defaults to `"LAN"`.
- `dest_ip_start` (optional) - Start of the destination IP address range.
- `dest_ip_end` (optional) - End of the destination IP address range. Leave empty for a single address.
- `protocol` (optional) - Protocol: `"TCP"`, `"UDP"`, `"TCP/UDP"`, or `"ICMP"`. Defaults to `"TCP"`.
- `port_start` (optional) - Start of the destination port range. Use `0` for any port. Defaults to `0`.
- `port_end` (optional) - End of the destination port range. Use `0` for any port. Defaults to `0`.

## Attribute Reference

- `id` - Same as `name`.
