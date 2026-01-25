---
page_title: "easydns_records Data Source - EasyDNS"
subcategory: ""
description: |-
  Fetches all DNS records for a domain from EasyDNS.
---

# easydns_records (Data Source)

Use this data source to retrieve all DNS records for a domain. Useful for auditing, finding record IDs for import, or filtering records by type.

## Example Usage

### List All Records

```terraform
data "easydns_records" "all" {
  domain = "example.com"
}

output "total_records" {
  value = length(data.easydns_records.all.records)
}

output "all_records" {
  value = [for r in data.easydns_records.all.records : "${r.type} ${r.host} -> ${r.rdata}"]
}
```

### Filter by Record Type

```terraform
data "easydns_records" "all" {
  domain = "example.com"
}

output "a_records" {
  value = [for r in data.easydns_records.all.records : r if r.type == "A"]
}

output "mx_records" {
  value = [for r in data.easydns_records.all.records : r if r.type == "MX"]
}
```

### Find Record IDs for Import

```terraform
data "easydns_records" "all" {
  domain = "example.com"
}

output "record_ids" {
  value = { for r in data.easydns_records.all.records : "${r.host}.${r.type}" => r.id }
}
```

## Schema

### Required

- `domain` (String) The domain/zone name.

### Read-Only

- `id` (String) Identifier for this data source.
- `records` (List of Object) List of all records in the domain.

Each object in `records` contains:

| Attribute | Type | Description |
|-----------|------|-------------|
| `id` | String | The unique record ID |
| `host` | String | Hostname/subdomain |
| `type` | String | Record type |
| `rdata` | String | Record data |
| `ttl` | Number | Time to live in seconds |
| `prio` | Number | Priority (for MX/SRV records) |
| `last_mod` | String | Last modification timestamp |
