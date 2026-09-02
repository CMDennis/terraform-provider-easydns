---
page_title: "easydns_mailmaps Data Source - EasyDNS"
description: |-
  Lists EasyDNS mail-forwarding maps for a domain.
---


# easydns_mailmaps (Data Source)

```terraform
data "easydns_mailmaps" "example" {
  domain = "example.com"
}
```

Mailmaps are sorted by numeric ID. Each returned object contains:

- `id` — immutable numeric mailmap ID.
- `alias` — source-address local part.
- `host` — relative host; `@` means the domain apex.
- `email` — fully-qualified source address.
- `destinations` — unordered set of forwarding destinations.
- `active` — whether forwarding is active.
- `last_modified` — timestamp last reported by EasyDNS.

The only argument is required `domain` (String), the domain to list.
