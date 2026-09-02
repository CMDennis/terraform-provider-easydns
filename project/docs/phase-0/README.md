# Phase 0 Baseline and Handoff

Phase 0 is complete as a design and public-contract baseline. It intentionally
does not modify the provider source tree or make authenticated EasyDNS calls.

## Deliverables

- Approved completion plan in `PROJECT_PLAN.md`
- Pinned EasyDNS OpenAPI v1.1.1 snapshot and checksum
- Machine-readable disposition for all 36 documented operations
- Generated human-readable API coverage matrix
- Accepted decisions for platform support, compatibility, record write modes,
  registrar safety, contracts, identities, and state
- Stable import and state conventions
- Layered testing and sandbox-safety strategy
- Sanitized schema-derived contract fixtures and validation tooling

## Architecture baseline

```text
Terraform Core 1.14+
        |
        v
Provider Framework / Protocol 6
  | resources | data sources | actions |
        |
        v
Typed EasyDNS client
  | context | timeout | redaction | 1 req/s limiter |
  | read retry/backoff | write reconciliation        |
        |
        +-----------------------------+
        |                             |
        v                             v
Synchronous record API        Asynchronous record API
        |                             |
        +----------> read waiters <---+
        |
        v
Other EasyDNS domain, registry, mail, and metadata APIs
```

## Fixed decisions for Phase 1

- Terraform 1.14+ and a framework version with action support are the v1 target.
- Protocol version remains 6.
- Only record mutations have a synchronous/asynchronous choice.
- Writes are reconciled, not blindly retried.
- The provider applies a default one-request-per-second client limiter.
- Domain registration and deletion are disabled/protected by default.
- Existing public names remain compatible through v1.x and are not removed
  before v2.0.
- User create/update operations are explicitly excluded from v1.

## Contract risks discovered

1. Record create responses have no defined content in OpenAPI, but the current
   client assumes a record body.
2. The API uses inconsistent scalar representations, including numeric values
   encoded as strings or null.
3. Several response `data` objects are loosely typed or incomplete.
4. Registration-status request examples and declared array shape conflict.
5. The mailmap update path uses `{email}` while deletion uses `mailmap_id`; the
   exact update identity must be verified.
6. Async success means queued zone regeneration, so Terraform needs read
   waiters to produce stable state.
7. The documented rate limit makes uncoordinated polling unsafe.

## Required sandbox observations before Phase 1 models freeze

Use only a dedicated EasyDNS sandbox account and sanitize results before
committing them:

- Record list with string, number, zero, and null numeric fields
- Synchronous and asynchronous create success body and headers
- Synchronous and asynchronous update success body
- Delete success body for both modes
- 420 response status, body, and retry-related headers
- Registration-status list and update shapes
- Nameserver get/update shapes
- Glue list/create/update/status shapes
- Mailmap list/create/update/delete identity behavior
- Service/subscription description `data` shapes
- Pricing response including premium and non-premium examples

## Provider source handoff

Before Phase 1 begins:

1. Decide whether this planning repository will be merged into the provider
   repository or whether the provider will be copied into this writable
   workspace.
2. Preserve and separately review the existing uncommitted validator edits in
   the provider tree.
3. Establish a branch from v0.1.1 for the client refactor.
4. Do not import the sibling real Terraform state, `tfvars`, token file, or real
   domain configurations into tests or fixtures.

## Phase 0 validation

Run:

```shell
./scripts/validate-phase-0.sh
```

Successful validation proves that the public OpenAPI snapshot is intact, every
documented operation has one disposition, generated coverage docs are current,
and fixture JSON is syntactically valid. It does not prove that illustrative
fixtures match the live sandbox API.
