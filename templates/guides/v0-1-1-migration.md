---
page_title: "Upgrade from v0.1.1"
subcategory: "Guides"
description: |-
  Upgrade existing record and zone configurations while preserving state.
---

# Upgrade from v0.1.1

The v1 provider preserves the v0.1.1 `easydns_record` and `easydns_zone`
addresses. Upgrade state in place; do not remove and recreate DNS records.

## Before upgrading

1. Back up Terraform state using your backend's supported mechanism.
2. Confirm `terraform plan` is empty with v0.1.1.
3. Upgrade Terraform CLI to 1.14 or newer.
4. Remove any saved production credentials from the shell while rehearsing the
   upgrade against sandbox state.

## Update the version constraint

```terraform
terraform {
  required_version = ">= 1.14.0"

  required_providers {
    easydns = {
      source  = "CMDennis/easydns"
      version = "~> 1.0"
    }
  }
}
```

Then run:

```shell
terraform init -upgrade
terraform plan
```

Existing record IDs remain unchanged. New computed/defaulted fields such as
`geozone_id` are populated during refresh.

## Replace the legacy record-mode setting

`use_async_api` still works through v1.x, so this change can be separate from
the provider upgrade:

```terraform
# before
provider "easydns" {
  use_async_api = true
}

# after
provider "easydns" {
  record_write_mode = "asynchronous"
}
```

Do not configure both. A mode-only change must not replace a record.

## Move from `easydns_zone`

The zone resource and data source are deprecated, not removed. You can upgrade
first and migrate them later. Follow [Migrating from easydns_zone](zone-migration.md)
to move state to `easydns_domain` safely.

## Verification

Review a refresh-only plan, then a normal plan:

```shell
terraform plan -refresh-only
terraform plan
```

The normal plan should be empty. Investigate any record replacement, domain
deletion, or unexplained value rewrite before applying.

The repository's gated migration acceptance test creates a disposable sandbox
record with provider v0.1.1, switches to the current Protocol 6 provider,
expects an empty plan, verifies import, and destroys the fixture. It cannot run
against production and is never scheduled automatically.
