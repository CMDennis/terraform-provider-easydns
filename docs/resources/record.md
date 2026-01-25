---
page_title: "easydns_record Resource - EasyDNS"
subcategory: ""
description: |-
  Manages a DNS record in EasyDNS.
---

# easydns_record (Resource)

Manages a DNS record in EasyDNS. Supports A, AAAA, CNAME, MX, TXT, NS, SRV, CAA, and other record types.

## Example Usage

### A Record

```terraform
resource "easydns_record" "web" {
  domain = "example.com"
  host   = "www"
  type   = "A"
  rdata  = "192.0.2.1"
  ttl    = 3600
}
```

### CNAME Record

```terraform
resource "easydns_record" "blog" {
  domain = "example.com"
  host   = "blog"
  type   = "CNAME"
  rdata  = "www.example.com."
  ttl    = 3600
}
```

### MX Record

```terraform
resource "easydns_record" "mail" {
  domain = "example.com"
  host   = "@"
  type   = "MX"
  rdata  = "mail.example.com."
  ttl    = 3600
  prio   = 10
}
```

### TXT Record (SPF)

```terraform
resource "easydns_record" "spf" {
  domain = "example.com"
  host   = "@"
  type   = "TXT"
  rdata  = "v=spf1 include:_spf.google.com ~all"
  ttl    = 3600
}
```

### AAAA Record (IPv6)

```terraform
resource "easydns_record" "ipv6" {
  domain = "example.com"
  host   = "www"
  type   = "AAAA"
  rdata  = "2001:db8::1"
  ttl    = 3600
}
```

## Schema

### Required

- `domain` (String) The domain/zone name (e.g., `example.com`).
- `host` (String) Hostname/subdomain. Use `@` for the root domain, `*` for wildcard.
- `type` (String) Record type (A, AAAA, CNAME, MX, TXT, NS, SRV, CAA, etc.).
- `rdata` (String) Record data (IP address, hostname, or text value depending on record type).

### Optional

- `ttl` (Number) Time to live in seconds. Defaults to `600`.
- `prio` (Number) Priority for MX and SRV records (0-100). Defaults to `0`.

### Read-Only

- `id` (String) The unique record ID assigned by EasyDNS.
- `last_mod` (String) Last modification timestamp.

## Import

Import existing records using the format `domain:record_id`:

```shell
terraform import easydns_record.web "example.com:12345678"
```

To find the record ID, use the `easydns_records` data source to list all records in a domain.

## Validation

The provider validates inputs:

- **Hostname**: Must contain only valid DNS characters (letters, digits, hyphens, underscores, dots, `@`, `*`)
- **IPv4 (A records)**: Must be a valid IPv4 address
- **IPv6 (AAAA records)**: Must be a valid IPv6 address
- **Priority**: Must be between 0 and 100
