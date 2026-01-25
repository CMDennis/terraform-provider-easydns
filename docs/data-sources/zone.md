---
page_title: "easydns_zone Data Source - EasyDNS"
subcategory: ""
description: |-
  Fetches information about a DNS zone in EasyDNS.
---

# easydns_zone (Data Source)

Use this data source to retrieve information about an existing DNS zone in EasyDNS.

## Example Usage

```terraform
data "easydns_zone" "main" {
  domain = "example.com"
}

output "zone_service" {
  value = data.easydns_zone.main.service
}

output "zone_exists" {
  value = data.easydns_zone.main.exists
}
```

## Schema

### Required

- `domain` (String) The domain name to look up.

### Read-Only

- `id` (String) Zone identifier.
- `exists` (Boolean) Whether the zone exists in EasyDNS.
- `on_system` (Boolean) Whether the zone is active on EasyDNS nameservers.
- `expiry` (String) Domain registration expiry date.
- `next_due` (String) Next billing due date.
- `service` (String) Service ID associated with the zone.
