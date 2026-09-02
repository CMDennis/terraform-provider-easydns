---
page_title: "Troubleshooting API behavior"
subcategory: "Guides"
description: |-
  Diagnose rate limits, timeouts, ambiguous writes, and eventual consistency.
---

# Troubleshooting API behavior

## Rate-limit responses (`420` or `429`)

The provider limits each configured client to one request per second. Bounded
read retries honor `Retry-After` when EasyDNS sends it and otherwise use
exponential backoff with jitter.

If rate limits persist:

1. Reduce Terraform parallelism, for example `terraform apply -parallelism=1`.
2. Avoid many separately configured provider aliases; each has its own limiter.
3. Prefer asynchronous record writes for a large record batch.
4. Wait for the account's limit window before retrying.

Writes are not automatically retried, even for a rate-limit response whose
delivery point is unclear.

### The daily request budget

EasyDNS documents a second, slower limit alongside the per-second one: **500
requests per day**, with the counter resetting at midnight EST. The provider
paces requests but does not track this budget, so exhausting it looks like
sustained `420` responses that backoff cannot clear.

Reconciliation is the usual cause. A mutation that does not become observable
polls every two seconds for up to two minutes, so a single write that never
settles can spend up to 60 reads. Eight of those exhaust a day.

If you are close to the cap:

- Raise `record_poll_interval` on the provider. At the 2s default a stuck
  write spends up to 60 reads; at `5s` it spends 24, and at `10s` it spends 12.
- Shorten `record_reconcile_timeout` where the API settles quickly, so a stuck
  write gives up sooner instead of polling for the full two minutes.

```terraform
provider "easydns" {
  record_poll_interval     = "5s"
  record_reconcile_timeout = "1m"
}
```
- Run acceptance suites one at a time rather than as a full sweep, and prefer
  the read-only suite for routine checks.
- Remember that `terraform plan` and `terraform refresh` read every managed
  object, so a large configuration costs requests even when nothing changes.

## Timeouts and cancellation

HTTP requests default to a 30-second timeout. Record and mailmap reconciliation
can continue for up to two minutes, polling every two seconds. Cancelling
Terraform cancels rate-limit waits, retries, HTTP requests, and reconciliation.

An operation that times out after sending a mutation may have succeeded. Read
the object in EasyDNS before deciding whether to import or retry it.

## Eventual consistency

Asynchronous record writes and registry glue can lag the successful request.
Records and mailmaps are polled to a stable lifecycle result. Glue exposes the
separate computed `registry_configured` flag because publication can take
longer; a later `terraform refresh` may change it from false to true.

## A second plan is not empty

- Check normalization: domains become lower-case IDNA ASCII without a trailing
  dot; IP addresses use canonical text.
- Treat nameserver and destination collections as sets; their order is not
  meaningful.
- Keep exact TXT and URL content, including spaces.
- If EasyDNS imposed a minimum TTL or rewrote a value, update configuration to
  the accepted remote value rather than applying repeatedly.

## Ambiguous write diagnostic

Stop and inspect the sandbox or production dashboard. For records, use
`easydns_records` to find a newly created ID, then import it as
`domain:record_id`. For mailmaps, use `easydns_mailmaps` and import
`domain:mailmap_id`. Repeating a create before inspection can make duplicates.

## Safe diagnostics

Do not paste authorization headers, full API responses, registrant contacts, or
Terraform state into an issue. Include the provider version, Terraform version,
resource type, sanitized configuration, HTTP status, and redacted diagnostic.
