---
page_title: "dlink_port_forward"
---

# dlink_port_forward

Manages a single port forwarding rule on the D-Link router.

-> On this router model the WAN and LAN port are always the same — there is no separate internal/external port mapping.

## Example Usage

```hcl
resource "dlink_port_forward" "http" {
  name     = "HTTP"
  protocol = "TCP"
  port     = 80
  local_ip = "192.168.0.100"
}

resource "dlink_port_forward" "ssh" {
  name     = "SSH"
  protocol = "TCP"
  port     = 2222
  local_ip = "192.168.0.50"
  enabled  = true
}
```

## Argument Reference

- `name` (required) - Rule description. Must be unique on the router.
- `port` (required) - Port number, used on both WAN and LAN sides.
- `local_ip` (required) - LAN IP address of the destination host.
- `protocol` (optional) - Protocol: `"TCP"`, `"UDP"`, or `"TCP/UDP"`. Defaults to `"TCP"`.
- `schedule` (optional) - Schedule name. Defaults to `"Always"`.
- `enabled` (optional) - Whether the rule is active. Defaults to `true`.

## Attribute Reference

- `id` - Composite ID in the format `<name>/<port>`.
