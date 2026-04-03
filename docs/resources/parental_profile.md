---
page_title: "dlink_parental_profile"
---

# dlink_parental_profile

Manages a parental control profile on the D-Link router. Each profile can block specific domains and be assigned to devices by MAC address.

## Example Usage

```hcl
resource "dlink_parental_profile" "kids" {
  name           = "Kids"
  filter_enabled = true

  blocked_domains = [
    { title = "YouTube", domain = "youtube.com" },
    { title = "TikTok",  domain = "tiktok.com"  },
  ]

  devices = [
    "8a:5b:39:45:2c:19",
    "8a:5b:39:45:2c:10",
  ]
}
```

## Argument Reference

- `name` (required) - Display name for the profile.
- `filter_enabled` (optional) - Whether domain filtering is active for this profile. Defaults to `true`.
- `allow_slow_access` (optional) - Whether to allow throttled (slow) access. Defaults to `false`.
- `blocked_domains` (optional) - List of domains to block. Each entry supports:
  - `title` (required) - Human-readable label for the entry.
  - `domain` (required) - Domain name to block, e.g. `"youtube.com"`.
- `devices` (optional) - List of MAC addresses of devices assigned to this profile, e.g. `["8a:5b:39:45:2c:19"]`.

## Attribute Reference

- `id` - UUID assigned to the profile by the router.
