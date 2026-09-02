---
page_title: "Verification status"
subcategory: "Guides"
description: |-
  Which parts of the provider have been exercised against a live EasyDNS API, and what the rest returned.
---

# Verification status

Every construct in this provider is covered by local contract tests against
recorded response shapes. That is not the same as having been run against
EasyDNS. This page records exactly which operations have been exercised
against a live sandbox account, and what the ones that failed returned.

Read it as **unverified**, not **broken**. The sandbox used for this work has
several non-functional backend integrations, and the same operations may
behave correctly in production. Nothing here has been shown to be defective in
production.

Last updated from a sandbox run on 2026-09-02.

## Verified against a live API

These completed a full Terraform lifecycle, including an empty second plan and
import verification where applicable.

| Construct | What ran |
|---|---|
| `easydns_record` | Create, update, delete, import, empty plan, in both write modes |
| `easydns_domain` | Create, read, import, empty plan. **Delete did not take effect**; see below |
| `easydns_domain_registration_settings` | Apply, empty plan, import |
| `easydns_zone` | Adoption, empty plan, import |
| `easydns_domain` (data source) | Read |
| Migration from v0.1.1 | State written by the published v0.1.1, then read, planned, applied and imported by this provider |

These were exercised directly against the sandbox, though not through a full
Terraform plan:

| Construct | What ran |
|---|---|
| `easydns_domains` | Read, including the username resolution an omitted `user` triggers |
| `easydns_domain_registration_statuses` | Read of the whole account |
| `easydns_glue_records` | Read |
| `easydns_domain_pricing` | Read across several TLDs and service levels |
| `easydns_current_user` | Read |
| `easydns_parsed_records` | Read |
| `easydns_records` | Read, and the server-side `search_keyword` path |
| `easydns_mailmaps` | Read |
| `easydns_geo_regions` | Read of all 255 regions, exercising all-page traversal |
| `easydns_service` | Read |
| `easydns_force_zone_reload` | Invoked and accepted |
| `easydns_set_primary_nameserver` | Invoked and accepted, against a disposable domain |

## Verified except for one operation

### `easydns_mailmap` — update is refused

Create, read, and delete all work. Update answers:

```
HTTP 406: Access to resource denied due to context restrictions
```

Both the bare alias and the fully-qualified `alias@domain` path forms were
tried, and it was reproduced on more than one mailmap. Until this is
confirmed in production, expect an in-place change to a mailmap to fail. A
destroy followed by a create uses only the operations that work.

### `easydns_glue_record` — update and delete are both refused

Create and read work. Update answers:

```
HTTP 400: Error encountered attempting to modify glue record
```

Delete answers:

```
HTTP 400: Error attempting to delete glue record.
          Please contact support for assistance (no result)
```

Both path forms were tried for the delete. **A glue record created through
this provider could be neither changed nor removed through it**, so any
change to one fails and `terraform destroy` leaves it at the registry. The
refused update did not partially apply: the addresses were unchanged
afterwards.

### `easydns_domain` — delete is accepted without taking effect

Delete answers `HTTP 200` with the domain name, and the domain then continues
to report `onsystem: "Y"` indefinitely with its next-due date unchanged.

The provider reports this rather than treating the accepted delete as done,
and **leaves the resource in state**. Removing it would abandon a domain that
may still bill. `terraform destroy` therefore fails rather than silently
succeeding.

## Not verified at all

### `easydns_domain_nameservers` (resource and data source)

Reading a domain's delegation failed for every domain tried:

```
HTTP 400: Error encountered attempting to retrieve name servers for domain:
          Authentication Error.
```

A DNS-only domain returned an empty body instead. Because the read is the
foundation of both the resource and the data source, neither has been
exercised in any respect, and the update operation has never run.

### `easydns_zone_soa`

Every read of a zone's SOA serial failed, for every domain tried and on three
consecutive attempts:

```
HTTP 400: timeout on read select()
```

This is an endpoint-wide failure on the sandbox rather than something specific
to one zone, and it looks like a backend timeout rather than a rejection.

### `easydns_subscription_service`

No domain on the account carries a subscription block, so there is no valid
identifier to read. A guessed identifier is refused with `HTTP 401: Invalid or
unauthorized subscription block ID provided for this install`, which shows the
endpoint responds but says nothing about whether the provider decodes a real
subscription correctly.

## The unverified endpoints

Every operation below is implemented and covered by local contract tests, and
none has been confirmed against a live EasyDNS API. Nine of the thirty-four
implemented operations are in this list.

| Method | Endpoint | Operation | What happened |
|---|---|---|---|
| `GET` | `/domains/ns/{domain}` | `getDomainNameservers` | `HTTP 400 Authentication Error` for every registered domain; empty body for a DNS-only one |
| `POST` | `/domains/ns/{domain}` | `updateDomainNameservers` | Never reached, because the read above never succeeded |
| `GET` | `/zones/records/soa/{domain}` | `getZoneSOA` | `HTTP 400 timeout on read select()` for every domain, on three consecutive attempts |
| `POST` | `/domains/glue/{domain}` | `updateGlue` | `HTTP 400 Error encountered attempting to modify glue record` |
| `DELETE` | `/domains/glue/{domain}/{host}` | `deleteGlue` | `HTTP 400 Error attempting to delete glue record. Please contact support for assistance (no result)` |
| `POST` | `/mail/maps/{domain}/{email}` | `updateMailmap` | `HTTP 406 Access to resource denied due to context restrictions`, for both the bare alias and the fully-qualified path |
| `DELETE` | `/domain/{domain}` | `deleteDomain` | `HTTP 200` accepted, then the domain keeps reporting `onsystem: "Y"` indefinitely |
| `GET` | `/services/subscription/{subscription_id}/description` | `getSubscriptionServiceDescription` | No account domain carries a subscription block, so there is no valid identifier to read |
| `GET` | `/domains/glue/{domain}/{host}/status` | `checkRegistryGlue` | Exercised only through a glue record the registry had not yet published, so a positive result has never been observed |

Seven of the nine returned an error from the sandbox rather than being merely
untried. That, together with delegation reads failing for every domain, points
at partially wired sandbox infrastructure rather than nine separate API
defects, which is why none of them is described here as broken.

## Why this matters

Local fixtures agreed with the code and disagreed with the API in seven places
found so far, including a domain read that failed for any domain without a
subscription block and a registration status change that was silently reported
as successful without being applied. Each was found only by making a real
request.

Treat any row above that is not in the first table as carrying that same risk.
If you exercise one against production, a report of what it returned is
genuinely useful.
