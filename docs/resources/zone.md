---
page_title: "easydns_zone Resource - EasyDNS"
subcategory: ""
description: |-
  Manages a DNS zone in EasyDNS (import-only).
---

# easydns_zone (Resource)

Manages a DNS zone in EasyDNS. This resource is **import-only** - zones must be created in the EasyDNS dashboard first, then imported into Terraform for tracking.

~> **Note:** This resource cannot create new zones. Zones must exist in EasyDNS before they can be managed by Terraform.

## Example Usage

```terraform
# Import an existing zone
resource "easydns_zone" "main" {
  domain = "example.com"
}

# Use the zone with records
resource "easydns_record" "www" {
  domain = easydns_zone.main.domain
  host   = "www"
  type   = "A"
  rdata  = "192.0.2.1"
}
```

## Schema

### Required

- `domain` (String) The domain name of the zone.

### Read-Only

- `id` (String) Zone identifier (same as domain).
- `exists` (Boolean) Whether the zone exists in EasyDNS.
- `on_system` (Boolean) Whether the zone is active on EasyDNS nameservers.
- `expiry` (String) Domain registration expiry date.
- `next_due` (String) Next billing due date.
- `service` (String) Service ID associated with the zone.

## Import

```shell
terraform import easydns_zone.main example.com
```

## Behavior

| Action | Behavior |
|--------|----------|
| Create | Adopts an existing zone. Fails if the zone doesn't exist in EasyDNS. |
| Read | Fetches current zone information from the API. |
| Update | Not applicable - zone properties are read-only. |
| Delete | Removes from Terraform state only. Does **not** delete the zone from EasyDNS. |
