---
page_title: "Synchronous and asynchronous record writes"
subcategory: "Guides"
description: |-
  Choose a record mutation route without changing Terraform lifecycle behavior.
---

# Synchronous and asynchronous record writes

EasyDNS exposes two routes for record mutations. The choice changes how
EasyDNS schedules zone regeneration, not the Terraform resource lifecycle.

| Mode | EasyDNS behavior | Good fit |
|---|---|---|
| `synchronous` | Requests immediate processing | Interactive changes and small plans |
| `asynchronous` | Queues the zone reload | Larger record batches and less latency per write |

Both modes perform one mutation request and then read until the requested state
is observable. Create discovers the newly assigned record ID, update waits for
the desired values, and delete waits for the known ID to disappear.

Set a provider default:

```terraform
provider "easydns" {
  record_write_mode = "asynchronous"
}
```

Override one record when necessary:

```terraform
resource "easydns_record" "urgent" {
  domain     = "example.com"
  host       = "status"
  type       = "A"
  rdata      = "192.0.2.20"
  write_mode = "synchronous"
}
```

## Ambiguous results

The provider retries bounded read operations on rate limits and transient
server failures. It does not blindly retry creates, updates, deletes, pricing
requests, or actions. If a connection fails after a write was sent, repeating
it could create a duplicate or replay a side effect.

For a record write, the provider first tries to reconcile the result through
reads. If it still cannot prove the outcome, it returns an error that tells you
to inspect EasyDNS and import an object that exists. Verify first; do not simply
run apply again.

## Migrating from `use_async_api`

Replace:

```terraform
provider "easydns" {
  use_async_api = true
}
```

with:

```terraform
provider "easydns" {
  record_write_mode = "asynchronous"
}
```

Run `terraform plan`. The change is provider configuration only and should not
replace records. Do not leave both attributes configured.
