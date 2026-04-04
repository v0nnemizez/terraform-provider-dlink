---
page_title: "dlink_lan"
---

# dlink_lan

Manages LAN settings on the D-Link router (IP address, subnet mask, DHCP server).

!> Changing `router_ip` will immediately disconnect the current session as the router moves to a new address. Update the `host` attribute in the provider configuration before running `tofu plan` again.

-> This is a singleton resource — there can only be one `dlink_lan` per provider instance. Destroying it removes the resource from state only; LAN settings cannot be deleted from the router.

## Example Usage

```hcl
resource "dlink_lan" "main" {
  router_ip    = "192.168.0.1"
  subnet_mask  = "255.255.255.0"
  dhcp_enabled = true
}
```

## Argument Reference

- `router_ip` (required) - Router LAN IP address (e.g. `"192.168.0.1"`).
- `subnet_mask` (optional) - LAN subnet mask. Defaults to `"255.255.255.0"`.
- `dhcp_enabled` (optional) - Whether the DHCP server is enabled. Defaults to `true`.

## Attribute Reference

- `id` - Always `"lan"`.
- `mac_address` - Router LAN MAC address (read-only, assigned by hardware).
