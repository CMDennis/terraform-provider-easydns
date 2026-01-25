---
page_title: "easydns_record Data Source - EasyDNS"
subcategory: ""
description: |-
  Fetches a specific DNS record from EasyDNS.
---

# easydns_record (Data Source)

Use this data source to retrieve information about a specific DNS record by domain, host, and type.

## Example Usage

```terraform
data "easydns_record" "www" {
  domain = "example.com"
  host   = "www"
  type   = "A"
}

output "www_ip" {
  value = data.easydns_record.www.rdata
}

output "www_ttl" {
  value = data.easydns_record.www.ttl
}
```

## Schema

### Required

- `domain` (String) The domain/zone name.
- `host` (String) The hostname/subdomain to look up.
- `type` (String) The record type (A, AAAA, CNAME, MX, TXT, etc.).

### Read-Only

- `id` (String) The unique record ID.
- `rdata` (String) Record data (IP address, hostname, or text value).
- `ttl` (Number) Time to live in seconds.
- `prio` (Number) Priority (for MX and SRV records).
- `last_mod` (String) Last modification timestamp.
