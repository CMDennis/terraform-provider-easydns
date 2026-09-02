---
page_title: "DNS record patterns"
subcategory: "Examples"
description: |-
  Examples for wildcard, geo, mail, service, and parked records.
---

# DNS record patterns

These examples use documentation-only addresses and placeholder names. Replace
them with values for your zone.

## Wildcard

```terraform
resource "easydns_record" "wildcard" {
  domain = "example.com"
  host   = "*"
  type   = "A"
  rdata  = "192.0.2.30"
  ttl    = 300
}
```

## Geo-targeted record

Look up EasyDNS region IDs instead of hard-coding an undocumented number:

```terraform
data "easydns_geo_regions" "available" {}

resource "easydns_record" "regional" {
  domain     = "example.com"
  host       = "www"
  type       = "A"
  rdata      = "192.0.2.31"
  geozone_id = 12 # choose an ID returned by the data source
}
```

`geozone_id = 0` means globally applicable.

## DKIM and DMARC

TXT content is opaque and retains its exact case and spacing:

```terraform
resource "easydns_record" "dkim" {
  domain = "example.com"
  host   = "selector1._domainkey"
  type   = "TXT"
  rdata  = "v=DKIM1; k=rsa; p=REPLACE_WITH_PUBLIC_KEY"
  ttl    = 3600
}

resource "easydns_record" "dmarc" {
  domain = "example.com"
  host   = "_dmarc"
  type   = "TXT"
  rdata  = "v=DMARC1; p=none; rua=mailto:dmarc@example.com"
  ttl    = 3600
}
```

Do not put a private DKIM key in Terraform. Only the public DNS value belongs
in the record.

## MX

```terraform
resource "easydns_record" "mail" {
  domain = "example.com"
  host   = "@"
  type   = "MX"
  rdata  = "mail.example.com."
  prio   = 10
  ttl    = 3600
}
```

## SRV

EasyDNS stores the record priority separately in `prio`; the remaining weight,
port, and target form `rdata`:

```terraform
resource "easydns_record" "sip" {
  domain = "example.com"
  host   = "_sip._tcp"
  type   = "SRV"
  prio   = 10
  rdata  = "5 5060 sip.example.com."
  ttl    = 3600
}
```

## Parked redirect

Use EasyDNS `URL` or `URLHTTPS` records only when that service is enabled for
the domain:

```terraform
resource "easydns_record" "parked" {
  domain = "example.com"
  host   = "@"
  type   = "URLHTTPS"
  rdata  = "https://www.example.com/"
  ttl    = 600
}
```

## Root and DNS targets

- Use `@` for the zone apex.
- A trailing dot on DNS-name targets is optional for comparison.
- TXT, URL, and other opaque values compare exactly.
- `A` and `AAAA` values are normalized to canonical IP text.
