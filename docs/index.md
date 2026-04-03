---
page_title: "Provider: D-Link"
---

# D-Link Provider

Manages configuration on D-Link DIR-X1530 (model R15, firmware 1.08.x) routers via the router's built-in SOAP API (`/DHMAPI/`).

## Example Usage

```hcl
terraform {
  required_providers {
    dlink = {
      source  = "v0nNemizez/dlink"
      version = "~> 0.1"
    }
  }
}

provider "dlink" {
  host     = "192.168.0.1"
  username = "Admin"
  password = var.router_password
}
```

## Argument Reference

- `host` (required) - Router hostname or IP address (e.g. `192.168.0.1`).
- `username` (required) - Router admin username.
- `password` (required, sensitive) - Router admin password.
- `endpoint` (optional) - Override the full SOAP API base URL. Defaults to `http://<host>/DHMAPI/`.
