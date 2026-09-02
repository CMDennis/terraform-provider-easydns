---
page_title: "Architecture and API coverage"
subcategory: "Development"
description: |-
  Understand the provider layers, lifecycle boundaries, and OpenAPI coverage.
---

# Architecture and API coverage

The coverage matrix records whether an operation is *implemented*. Whether it
has been *exercised against a live EasyDNS API* is a separate question,
answered in the [verification status guide](verification-status). Several
implemented operations have never run outside a local contract test.


## Layers

```text
Terraform 1.14+ / Protocol 6
        |
        v
resources | data sources | actions
        |
        v
typed EasyDNS client
  timeout | limiter | read retries | redacted errors
        |
        +--> synchronous record route
        +--> asynchronous record route
        +--> domains, registry, mail, and metadata routes
```

Framework code translates Terraform configuration, plans, state, and
diagnostics. `internal/client` owns HTTP paths, wire coercion, pagination,
normalization, retry classification, and write reconciliation. Keeping those
roles separate makes the API behavior testable with local HTTP fixtures.

## Mutation rule

A mutation is issued exactly once. Transport, response-read, empty-body, and
decode failures after a write are treated as ambiguous. Resource code then
uses safe reads to discover whether the intended result exists. Imperative
actions cannot be reconciled and tell the operator to verify before replay.

## OpenAPI coverage

The pinned EasyDNS v1.1.1 contract contains 36 operations:

- 34 are implemented by 7 resources, 16 data sources, 2 actions, or shared
  lifecycle client calls;
- 2 arbitrary-user mutation operations are deliberately excluded.

User creation/update is excluded because the API lacks a complete
read/delete lifecycle for the created user and would require returning new
credentials through durable Terraform state. The read-only current-user data
source is implemented.

The source completion repository retains the pinned OpenAPI file, checksum,
machine-readable disposition, and generated operation-by-operation coverage
table. A release must not change the contract snapshot without reviewing every
operation's disposition.

## Compatibility surfaces

`easydns_zone` remains as a deprecated v1 compatibility resource and data
source. `easydns_domain` is the replacement. `use_async_api` remains a
deprecated compatibility alias for `record_write_mode` through v1.x.
