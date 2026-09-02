---
page_title: "Migrating from easydns_zone"
subcategory: "Guides"
description: |-
  How to move from the deprecated easydns_zone resource and data source to easydns_domain.
---

# Migrating from `easydns_zone`

`easydns_zone` and the `easydns_zone` data source are deprecated as of v0.3.0
and will be removed in v2.0.0. `easydns_domain` replaces both.

## Why replace it

`easydns_zone` is import-only: it can adopt a domain that already exists, but
it cannot add one, cannot register one, and cannot delete one. It also exposes
only a subset of the documented domain response. `easydns_domain` covers the
full lifecycle and adds `cloned_to` and `subscription_id`.

Nothing breaks yet. Both surfaces still work for the whole v0.x and v1.x line;
they only emit a deprecation warning.

## Migrating the data source

Rename the type. Every attribute keeps its name.

```terraform
# Before
data "easydns_zone" "example" {
  domain = "example.com"
}

# After
data "easydns_domain" "example" {
  domain = "example.com"
}
```

Update references from `data.easydns_zone.example.*` to
`data.easydns_domain.example.*`. The one behavioral difference: a domain that
is not on the EasyDNS system is now an error rather than a result with
`on_system = false`.

## Migrating the resource

`easydns_domain` needs four attributes that `easydns_zone` did not have:
`service`, `term`, `currency`, and `dns_only`. Set them to match the domain's
existing EasyDNS configuration. `service_id` on the old resource, or the
`easydns_domain` data source, tells you which service the domain uses.

Because the resource type changes, move the domain through state rather than
letting Terraform destroy and recreate it.

```terraform
# Before
resource "easydns_zone" "example" {
  domain = "example.com"
}

# After
resource "easydns_domain" "example" {
  domain   = "example.com"
  service  = "dns"
  term     = 1
  currency = "USD"
  dns_only = true

  # Keep the safe default until you deliberately want to allow deletion.
  deletion_protection = true
}
```

Then:

```shell
terraform state rm easydns_zone.example
terraform import easydns_domain.example example.com
terraform plan
```

The plan must be empty before you apply. If it is not, the `service`, `term`,
`currency`, or `dns_only` value you wrote does not match the domain's actual
configuration — correct the configuration rather than applying, because those
attributes are immutable and a mismatch cannot be fixed by an update.

## What does not change

- Record management through `easydns_record` is unaffected.
- Import identifiers stay the plain domain name for both resources.
- No EasyDNS request is made by `terraform state rm`, so the domain itself is
  never touched during the migration.

## Deletion safety

`easydns_zone` had no delete path at all. `easydns_domain` does, so it defaults
`deletion_protection = true`. A `terraform destroy` fails while protection is
on. This is deliberate: after migrating, a `destroy` that used to be a no-op
for the domain could otherwise remove real DNS and registrar service. Set
`deletion_protection = false` and apply before you intend to delete a domain.
