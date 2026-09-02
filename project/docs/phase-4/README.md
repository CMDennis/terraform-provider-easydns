# Phase 4 — Mail, Metadata, and Actions

## Outcome

Phase 4 is implemented in the safe [`provider/`](../../provider/) working copy.
It completes the Terraform surface mapped from the pinned EasyDNS OpenAPI
v1.1.1 contract: 34 operations are implemented and the remaining two are the
documented arbitrary-user exclusions.

No EasyDNS credential was read and no authenticated or live-domain request was
made. Tests use local HTTP servers, synthetic responses, reserved `.invalid`
domains, and documentation-only Terraform examples.

## Terraform surface

| Kind | Name | Purpose |
|---|---|---|
| Resource | `easydns_mailmap` | Mail-forwarding CRUD with immutable ID reconciliation |
| Data source | `easydns_mailmaps` | Stable ID-sorted domain mailmaps |
| Data source | `easydns_current_user` | Authenticated account metadata with PII sensitivity |
| Data source | `easydns_service` | Service description by ID |
| Data source | `easydns_subscription_service` | Subscription block and attached service metadata |
| Data source | `easydns_domain_pricing` | Availability and account-specific exact-decimal pricing |
| Action | `easydns_force_zone_reload` | Immediate zone regeneration |
| Action | `easydns_set_primary_nameserver` | Change a domain to secondary DNS and set its master |

The action surface requires Terraform 1.14 or newer, matching ADR-0001 and the
provider's existing minimum version.

## Mailmap safety and state

EasyDNS create responses identify the domain but do not include the new
`mailmap_id`. The client therefore lists IDs before creation, sends exactly one
`PUT`, and accepts only one matching new object. Multiple candidates produce a
deterministic error rather than arbitrary state selection.

Updates use the prior fully-qualified source address in the API path, then
poll the immutable ID until the complete replacement is visible. Deletes poll
that same ID until absent. Destination addresses are validated, deduplicated,
sorted, and represented by a Terraform set, so response order does not create
perpetual drift. Import uses `domain:mailmap_id`.

## Operation semantics

Phase 4 makes retry classification explicit rather than deriving it solely
from the HTTP verb:

- Domain pricing is a read-only `POST`; bounded read retries apply for 420,
  429, and transient 5xx responses.
- Force reload is a side-effecting `GET`; it is issued once and never retried.
- Primary-nameserver changes and every mailmap mutation are also issued once.
- A transport loss, malformed success body, or transient server failure after
  an imperative action is reported as ambiguous. Documentation tells the
  operator to verify EasyDNS state before trying again.

This closes a subtle safety gap that a verb-only policy would create.

## Data handling

The current-user data source includes every field in the pinned contract.
Username, identity, organization, address, telephone/fax, email, and URL
attributes are marked sensitive. The docs also explain that Terraform still
stores sensitive values in state and that the backend must be protected.

Pricing and tax amounts are decoded from either JSON strings or numbers and
stored as Terraform strings. This preserves exact decimal text and pairs with
the exact premium-price ceiling enforced by `easydns_domain`; a pricing lookup
never registers or bills a domain.

## Contract and framework tests

The local suites cover:

- all Phase 4 paths, methods, bodies, flexible scalar forms, and model mapping;
- mailmap create/update/delete reconciliation and exactly one mutation call;
- duplicate mailmap creation candidates and invalid addresses;
- read-only `POST` retry behavior and exact-decimal preservation;
- no-retry, ambiguous-outcome behavior for both imperative actions;
- provider protocol schemas and registration of every new construct;
- action invocation through a configured client;
- current-user PII sensitivity and strict mailmap import parsing.

## API coverage audit

`api/coverage.csv` remains the source of truth and passes the pinned-contract
validator. The 36 operations now have these dispositions:

- 34 complete;
- 0 partial;
- 0 planned;
- 2 excluded (`createUser` and `updateUserDS`).

The exclusions remain intentional: the API lacks a complete lifecycle for an
arbitrary created user, its encrypted update contract is underspecified, and a
create response would put newly issued credentials into durable Terraform
state.

## Verification

The Phase 4 handoff is accepted only when all of these local, non-destructive
gates pass:

- `make check` (tidy, gofmt, vet, race tests, build);
- integration-tag compile without executing a live test;
- recursive Terraform formatting;
- the 36-operation pinned-contract validator;
- endpoint, schema, import, retry, reconciliation, and documentation checks.

After the Phase 4 tests, statement coverage is 81.5% for `internal/client`,
34.4% for `internal/provider`, and 58.5% overall. Provider lifecycle paths that
require Terraform acceptance orchestration remain part of Phase 5.

Live sandbox action and mailmap observations remain appropriate for the Phase
5 acceptance suite, behind the existing sandbox-only and mutation opt-ins.
