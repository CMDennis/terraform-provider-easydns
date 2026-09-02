# ADR-0004: API contracts, identities, and state

- Status: Accepted
- Date: 2026-09-01

## Context

The OpenAPI document contains inconsistent and sometimes incomplete schemas.
Observed examples include numbers represented as strings or null, loosely typed
response envelopes, and mutation responses with no defined content. Terraform
requires stable identities and state even when the remote contract is loose.

## Decision

- Pin the upstream OpenAPI document and map every operation by `operationId`.
- Treat the OpenAPI schema as a starting contract, not proof of runtime shape.
- Schema-derived fixtures are explicitly labeled illustrative. Dedicated
  sandbox observations must be sanitized before becoming verified fixtures.
- Decode EasyDNS envelopes with typed models plus narrowly scoped flexible
  scalar types; do not use unbounded `map[string]any` in resource logic.
- Preserve API-assigned numeric identifiers as Terraform strings.
- Use the stable import formats defined in
  `docs/state-and-import-conventions.md`.
- A resource read removes state only on a verified not-found condition. It does
  not infer not-found from arbitrary message text, 403, throttling, or network
  errors.
- Collection data sources use deterministic ordering before writing state.
- Request/response errors are redacted and bounded; authorization headers,
  credentials, contacts, and arbitrary full response bodies are not logged.

## Consequences

- Phase 1 needs explicit envelope and scalar types and contract regression
  tests.
- Sandbox validation can correct fixture shapes without changing Terraform
  resource boundaries or identifiers.
- The coverage validator prevents new API operations from silently going
  unmapped when the OpenAPI snapshot is updated.

## Revisit when

- EasyDNS publishes a corrected schema with stronger compatibility guarantees,
  or
- an official generated Go SDK becomes available and is demonstrably safer
  than the provider's narrow client.
