# terraform-provider-dlink

Terraform/OpenTofu provider for managing D-Link routers via the built-in SOAP API.

> **⚠️ Compatibility warning**
>
> This provider has only been tested on the following exact hardware and firmware combination:
>
> | Field | Value |
> |---|---|
> | Model | D-Link DIR-X1530 (R15) |
> | Hardware revision | A1 |
> | Firmware version | 1.08.05 |
>
> It may not work on other D-Link models, hardware revisions, or firmware versions. The SOAP API, authentication protocol, and XML structure are all specific to this firmware and are subject to change in future updates.

## Resources

| Resource | Description |
|---|---|
| `dlink_wifi` | WiFi settings for a radio band (2.4GHz / 5GHz) |
| `dlink_port_forward` | Port forwarding rules |
| `dlink_parental_profile` | Parental control profiles with domain blocking and device assignment |
| `dlink_firewall_rule` | IPv4 firewall rules |

## Usage

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

See the [docs/](docs/) directory for full resource documentation.

## Requirements

- [OpenTofu](https://opentofu.org/) >= 1.6 or [Terraform](https://www.terraform.io/) >= 1.5
- Go >= 1.22 (for building from source)

## Building from source

```bash
go install .
```

## Running tests

```bash
go test ./internal/client/...
```

## Local development

Add a dev override to `~/.tofurc`:

```hcl
provider_installation {
  dev_overrides {
    "registry.opentofu.org/v0nNemizez/dlink" = "/Users/<you>/go/bin"
  }
  direct {}
}
```
