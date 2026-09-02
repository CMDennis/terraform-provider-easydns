---
page_title: "Registration, premium pricing, and deletion safety"
subcategory: "Guides"
description: |-
  The opt-ins that guard billable and irreversible registrar operations.
---

# Registration, premium pricing, and deletion safety

Registrar operations spend money and change things Terraform cannot undo. The
provider guards them with opt-ins that are off by default. Nothing here
restricts DNS record management or DNS-only domains.

## Registering a domain

Two things must both be true before a domain is registered:

1. The provider has `enable_domain_registration = true` (or
   `EASYDNS_ENABLE_DOMAIN_REGISTRATION=true`).
2. The resource has `dns_only = false`.

Without the provider opt-in, a registering plan fails during `terraform plan`,
before any request is sent. The client enforces the same rule independently, so
a registration cannot be issued through a client that was not configured to
allow it.

```terraform
provider "easydns" {
  enable_domain_registration = true
}
```

Leave the opt-in off in any workspace that only manages DNS.

## Premium pricing

Registries price some domains far above the standard rate. A premium
registration requires:

- `premium = true`, acknowledging that the registry prices this domain specially
- `premium_price`, the exact price returned by the EasyDNS pricing endpoint
- `max_premium_price`, the highest price this configuration will accept

If `premium_price` exceeds `max_premium_price`, the plan fails. Prices are
compared as exact decimals, so no amount is ever accepted through
floating-point rounding.

Set `max_premium_price` to what you are actually willing to pay, not to the
quoted price plus headroom.

## Deleting a domain

`easydns_domain.deletion_protection` defaults to `true`. While it is on,
`terraform destroy` fails with a diagnostic.

The provider does **not** offer a "remove from state only" behavior on destroy.
Silently dropping a domain from state while leaving it registered would make
Terraform's view of the world quietly wrong. To delete a domain:

```terraform
resource "easydns_domain" "example" {
  # ...
  deletion_protection = false
}
```

Apply that change, then destroy. An imported domain always starts protected,
regardless of what the configuration says, until the first apply.

## Operations that never write on destroy

Three resources adopt existing remote objects. Destroying them stops Terraform
managing the object and changes nothing remotely:

| Resource | Destroy behavior |
|---|---|
| `easydns_domain_registration_settings` | Reglock and renewal policy left as-is |
| `easydns_domain_nameservers` | Delegation left in place |
| `easydns_zone` | Import-only; nothing to delete |

Each emits a warning on destroy so the behavior is visible in the plan output.

`easydns_glue_record` is the exception: destroying it does delete the glue
record, because glue exists only to serve a delegation you control. A registry
refuses that deletion while any domain in the same TLD still points at the
host, and the refusal is surfaced rather than retried.

## Contact data in state

Registrant contacts and TLD-specific registration fields are personal data.
They are marked sensitive so they do not appear in plan output, but Terraform
still writes them to state in plain text. Any workspace that registers domains
needs an encrypted, access-controlled state backend.

## Testing

Acceptance tests refuse to run against any host other than the official EasyDNS
sandbox. Registration and deletion tests need a further explicit environment
gate and a dedicated disposable sandbox account.
