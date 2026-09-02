# EasyDNS Terraform Provider Completion Plan

## Outcome

Deliver a safe, documented v1.0 Terraform provider that covers every EasyDNS
OpenAPI operation that maps cleanly to Terraform and explicitly documents the
operations that do not.

Assumption: one experienced Go/Terraform engineer. Estimated elapsed effort is
six to eight engineering weeks.

## Safety boundary

- Development and automated acceptance testing are sandbox-only.
- Production credentials and production domains are never used by automated
  tests.
- No write is automatically retried after an ambiguous network failure.
- Domain registration and domain deletion require separate explicit opt-ins.
- Credentials, contact data, authorization headers, and response bodies that
  may contain PII are not logged.

## Current baseline

The existing provider is v0.1.1 and currently has:

- Resources: `easydns_record`, adoption/import-only `easydns_zone`
- Data sources: `easydns_record`, `easydns_records`, `easydns_zone`
- Synchronous and asynchronous paths for record writes
- Passing local unit tests with 18.5% statement coverage
- No Terraform framework acceptance suite
- Hand-maintained Terraform Registry documentation

The source tree already has uncommitted edits in
`internal/provider/record_resource.go` and `internal/provider/validators.go`.
Those changes must be preserved and reviewed separately before implementation
work begins.

## Target provider surface

### Managed resources

| Construct | Purpose |
|---|---|
| `easydns_record` | DNS record CRUD with synchronous and asynchronous write modes |
| `easydns_domain` | Guarded DNS-only domain addition or domain registration |
| `easydns_domain_registration_settings` | Registration lock and renewal policy |
| `easydns_domain_nameservers` | Delegated nameserver management |
| `easydns_glue_record` | Registry glue-record CRUD and status |
| `easydns_mailmap` | Mail-forwarding map CRUD |
| `easydns_zone` | Compatibility resource, deprecated after replacements ship |

### Data sources

- `easydns_domain` and `easydns_domains`
- `easydns_record` and `easydns_records`
- `easydns_parsed_records` and `easydns_zone_soa`
- `easydns_domain_registration_statuses`
- `easydns_domain_nameservers`
- `easydns_glue_records`
- `easydns_mailmaps`
- `easydns_geo_regions`
- `easydns_current_user`
- `easydns_service` and `easydns_subscription_service`
- `easydns_domain_pricing`

### Provider actions

- `easydns_force_zone_reload`
- `easydns_set_primary_nameserver`

The actions require Terraform 1.14 or newer. They model imperative endpoints
that do not have a reliable CRUD/read lifecycle.

### Deliberate v1 exclusion

Arbitrary EasyDNS user creation and update are not modeled in v1. The API does
not expose a complete read/delete lifecycle for a created user, and returning
new credentials through durable Terraform state would create an avoidable
security hazard.

## Synchronous and asynchronous record writes

Replace the boolean-only public interface with a provider default and optional
resource override:

```hcl
provider "easydns" {
  record_write_mode = "synchronous" # or "asynchronous"
}

resource "easydns_record" "example" {
  write_mode = "asynchronous" # optional override
  domain     = "example.invalid"
  host       = "www"
  type       = "A"
  rdata      = "192.0.2.10"
}
```

`use_async_api` remains a deprecated compatibility alias through v1.x.

Both modes share one resource lifecycle. When a create response has no usable
record ID, the implementation compares record IDs before and after the write.
Asynchronous writes poll until creation/update is observable or deletion is no
longer observable. An ambiguous write is reconciled by reading remote state
before Terraform reports success, failure, or recommends import.

## Delivery phases

### Phase 0 — contract and design baseline (complete in this repository)

- Pin the official OpenAPI v1.1.1 document with provenance and checksum.
- Map every operation to a resource, data source, action, or exclusion.
- Decide minimum Terraform/framework versions and compatibility policy.
- Decide import identifiers, normalization, and state semantics.
- Record safety boundaries for registration, deletion, and tests.
- Add schema-derived sanitized fixtures and identify live-contract gaps.

### Phase 1 — client and provider foundation (complete in `provider/`)

- Extract a typed, context-aware EasyDNS client.
- Add HTTP timeouts, path escaping, pagination, redacted errors, and response
  coercion for inconsistent scalar types.
- Add a default one-request-per-second limiter.
- Retry bounded read operations on 420, 429, and transient 5xx responses.
- Never blindly retry ambiguous writes; surface them for resource-level read
  reconciliation and Phase 2 eventual-consistency waiters.
- Upgrade the Terraform Plugin Framework and introduce testable clocks,
  sleepers, and transports.

### Phase 2 — DNS core (complete in `provider/`)

- Complete `easydns_record` in both write modes.
- Add read-after-write and delete waiters.
- Add `geozone_id` and all documented record types.
- Complete records search, parsed-zone, SOA, and geo-region data sources.
- Add import, drift, not-found, duplicate-value, and normalization tests.

### Phase 3 — domains and registrar controls (complete in `provider/`)

- Add domain details/list data sources.
- Add guarded DNS-only and registration modes to `easydns_domain`.
- Model contacts and documented TLD-specific fields as sensitive nested data.
- Add nameserver, registration-settings, and glue resources/data sources.
- Publish the `easydns_zone` migration and deprecation path.

### Phase 4 — mail, metadata, and actions (complete in `provider/`)

- Add mailmap resource and data source.
- Add user, service, subscription, and pricing data sources.
- Add force-reload and primary-nameserver actions.
- Audit the endpoint coverage matrix for omissions.

The audit accounts for all 36 operations in the pinned OpenAPI contract: 34
are implemented and the two arbitrary-user mutations remain deliberate v1
exclusions for lifecycle and credential-state safety reasons.

### Phase 5 — documentation, CI, and release hardening (complete in `provider/`)

- Generate Terraform Registry reference pages with `tfplugindocs` templates.
- Add schema/doc drift checks and example validation.
- Add unit, contract, framework, race, static-analysis, and sandbox acceptance
  jobs.
- Verify migration, release signing, checksums, and Registry rendering.

The implementation is complete without using EasyDNS credentials. Local gates
cover generated Registry rendering, schema/doc drift, example formatting,
unit/contract/framework/race tests, static analysis, vulnerability scanning,
acceptance-suite compilation, workflow validation, a GoReleaser snapshot, and
snapshot checksums. Protected CI environments hold the remaining live release
evidence: sandbox lifecycle/migration runs and verification with the real
release signing key. Those jobs cannot target production and are intentionally
not run from this completion workspace.

## Test strategy

| Layer | Required coverage |
|---|---|
| Unit | Request mapping, response coercion, validation, throttling, retry decisions, reconciliation, redaction, import parsing |
| Contract | Every endpoint through local HTTP fixtures, including malformed and error responses |
| Framework | Provider configuration, schema, plan modifiers, diagnostics, resources, data sources, and actions |
| Acceptance | Sandbox-only lifecycle tests for every managed remote object |
| Compatibility | Both record write modes and migration from `use_async_api` |
| Static | Formatting, vet, lint/static analysis, race tests, vulnerability scanning |
| Documentation | Generated-doc drift and executable example checks |

The current production-selection switch in `integration_test.go` must be
removed. Acceptance tests must reject any host other than the official EasyDNS
sandbox and use dedicated disposable fixtures.

## Documentation deliverables

- Five-minute sandbox quick start
- Authentication and provider configuration reference
- One page per resource, data source, and action
- Synchronous versus asynchronous behavior guide
- Registration, premium-price, and deletion safety guide
- Import and v0.1.1 migration guides
- Record-type examples, including geo, wildcard, DKIM/DMARC, MX, SRV, and
  parked records
- Rate-limit, timeout, retry, and eventual-consistency troubleshooting
- Architecture, endpoint coverage, testing, contribution, and release guides
- Terraform state, credentials, contact PII, and CI-secret security guidance

## Release gates

v1.0 is ready when:

- Every pinned OpenAPI operation is implemented or explicitly excluded.
- Both record write modes pass equivalent lifecycle tests.
- Every managed resource passes create/read/update/import/drift/delete testing
  where the API supports those operations.
- A second plan after apply is empty.
- Automated acceptance tests cannot reach production.
- Generated documentation and examples match the provider schema.
- Migration from v0.1.1 is tested and documented.
- Release artifacts, checksums, signatures, and Registry rendering are
  verified.

Recommended release sequence:

1. `v0.2.0`: client foundation and DNS core preview
2. `v0.3.0`: domain, registry, and mail coverage
3. `v1.0.0-rc.1`: sandbox soak and migration validation
4. `v1.0.0`: stable release

## Primary references

- EasyDNS OpenAPI UI: <https://docs.sandbox.rest.easydns.net:3001/>
- EasyDNS legacy documentation and rate limits:
  <https://docs.sandbox.rest.easydns.net/>
- Terraform Plugin Framework:
  <https://developer.hashicorp.com/terraform/plugin/framework>
- Terraform provider acceptance tests:
  <https://developer.hashicorp.com/terraform/plugin/testing/acceptance-tests>
- Terraform Registry provider documentation:
  <https://developer.hashicorp.com/terraform/registry/providers/docs>
