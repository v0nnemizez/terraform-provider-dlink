---
page_title: "dlink_adv_network_settings"
---

# dlink_adv_network_settings

Manages advanced network settings on the D-Link router (UPnP, multicast, WAN port speed).

-> This is a singleton resource. Destroying it removes the resource from state only.

## Example Usage

```hcl
resource "dlink_adv_network_settings" "main" {
  upnp           = true
  multicast_ipv4 = true
  multicast_ipv6 = true
  wan_port_speed = "Auto"
}
```

## Argument Reference

- `upnp` (optional) - Whether UPnP is enabled. Defaults to `true`.
- `multicast_ipv4` (optional) - Whether IPv4 multicast (IGMP proxy) is enabled. Defaults to `true`.
- `multicast_ipv6` (optional) - Whether IPv6 multicast (MLD proxy) is enabled. Defaults to `true`.
- `wan_port_speed` (optional) - WAN port speed. Defaults to `"Auto"`.

## Attribute Reference

- `id` - Always `"adv_network_settings"`.
