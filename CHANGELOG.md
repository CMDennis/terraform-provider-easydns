# Changelog

All notable changes will be documented here. Releases follow Semantic
Versioning after v1.0.0.

## v1.0.2

### Security and reliability

- Require HTTPS for API endpoints, with a literal loopback exception for local
  test servers, and reject all redirects to prevent credential disclosure and
  replay of mutation bodies.
- Save confirmed domain and DNS record identities when read-back fails. Poll
  domain visibility, retain deletion protection, and report a refresh warning
  instead of tainting confirmed creations for replacement. Pending resources
  remain in state when subsequent reads are unavailable.
- Reject incomplete success envelopes, missing/null collections, invalid
  presence flags, mismatched domain identities, and inconsistent collection
  metadata instead of interpreting them as resource absence.
- Redact configured credentials from structured read errors; suppress arbitrary
  API text on sensitive endpoints and requests with bodies. Transport and decode
  failures no longer echo remote values or URL paths into diagnostics.

### Maintenance

- Remove the GitHub sandbox acceptance workflow. Keep the manual local
  acceptance and integration suites and their existing opt-in protections.
- Add synthetic regression coverage and update security, recovery, testing,
  and release documentation. No live EasyDNS tests are required by GitHub CI.

## v1.0.1

Supersedes v1.0.0, which was withdrawn. A test fixture in the domain list
tests carried a real account username taken from a live sandbox response.
It was replaced with a neutral value and removed from the repository history,
so v1.0.0 no longer exists as a tag, release, or resolvable revision.

No provider code changed between the two. The affected file is a test and was
never part of a released binary.

## v1.0.0 (withdrawn)

First stable release. Covers every operation in the pinned EasyDNS OpenAPI
v1.1.1 document except two deliberate exclusions.

### Added

- Seven resources, sixteen data sources, and two actions spanning DNS records,
  domains, registrar controls, delegation, glue, mail forwarding, and account
  metadata.
- A typed, context-aware client with timeouts, path escaping, pagination,
  bounded redacted errors, a one-request-per-second limiter, and bounded read
  retries.
- Synchronous and asynchronous record write modes sharing one lifecycle, with
  `record_write_mode` on the provider and `write_mode` per resource.
- Read-after-write reconciliation for every mutation. A write is issued exactly
  once and confirmed by reading; an ambiguous outcome is never replayed.
- `record_poll_interval` and `record_reconcile_timeout` for tuning that
  reconciliation against the EasyDNS daily request budget.
- Registrar safety from ADR-0003: `enable_domain_registration` defaults to
  false and is enforced in both the plan and the client; `deletion_protection`
  defaults to true; premium prices are compared as exact decimals against a
  required ceiling; immutable attributes report a diagnostic instead of
  destroying and recreating a real domain.
- Generated Terraform Registry documentation, thirteen guides, and layered
  local, CI, sandbox acceptance, migration, vulnerability, and release
  verification.

### Verification

Five constructs completed full Terraform lifecycles against a live EasyDNS
sandbox, along with migration from the published v0.1.1. Nine of the
thirty-four implemented operations could not be confirmed against a live API;
each is listed with its exact response in the verification status guide.

### Deprecated

- `easydns_zone` and its data source, replaced by `easydns_domain`. Both keep
  working through v1.x and are removed in v2.0.0.
- `use_async_api`, replaced by `record_write_mode`.

### Excluded

- Arbitrary user creation and update. The API exposes no complete read or
  delete lifecycle for a created user, and returning new credentials through
  durable Terraform state would be an avoidable security hazard.
