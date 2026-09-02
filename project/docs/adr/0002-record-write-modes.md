# ADR-0002: Record write modes and eventual consistency

- Status: Accepted
- Date: 2026-09-01

## Context

EasyDNS exposes paired synchronous and asynchronous endpoints for DNS-record
create, update, and delete. Reads use common endpoints. The asynchronous paths
queue zone regeneration, so a successful HTTP response does not guarantee that
a subsequent read immediately observes the change. The OpenAPI document also
does not promise a response body for record creation.

## Decision

- Add `record_write_mode = "synchronous" | "asynchronous"` to the provider.
- Add an optional `write_mode` override to `easydns_record`.
- Default to `synchronous` to preserve current behavior.
- Continue accepting `use_async_api`; map `true` to asynchronous and `false` to
  synchronous when `record_write_mode` is absent.
- Use the same state schema and lifecycle logic for both modes.
- After a write, reconcile through reads until the desired state is observable
  or the resource timeout expires.
- If create returns no ID, compare the set of record IDs before and after the
  request and then confirm the selected record matches the desired semantic
  value.
- Never automatically repeat a create/update/delete after a transport failure
  where the server may already have processed the request. Reconcile first.
- Retry bounded reads on documented throttling and transient server failures.

## Consequences

- Terraform will not finish an asynchronous resource operation merely because
  the change was queued; it finishes when Terraform can refresh stable state.
- Create may require extra list calls, which makes rate limiting and efficient
  polling mandatory.
- Duplicate records with identical values remain distinguishable by EasyDNS
  record ID and before/after set comparison.

## Revisit when

- EasyDNS adds operation-status endpoints or guaranteed IDs in all mutation
  responses, or
- the service adds idempotency keys for writes.
