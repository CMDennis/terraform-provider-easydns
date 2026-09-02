# Testing Strategy

## Goals

The suite must provide fast local feedback, verify Terraform lifecycle behavior,
detect EasyDNS contract drift, and make it mechanically difficult to touch a
production account.

Current Phase 5 baseline: local unit, HTTP contract, framework, race, lint,
vulnerability, documentation, workflow, and release-snapshot checks pass.
Sandbox acceptance and v0.1.1 migration suites are implemented and compile,
but require protected sandbox fixtures before a release.

## Test layers

### Unit tests

Use fake transports, clocks, and sleepers. Cover:

- Authentication and headers without logging credentials
- URL/path/query construction and IDNA handling
- Flexible string/number/null scalar decoding
- API envelope success/error parsing and bounded redaction
- Rate limiting, backoff, jitter, cancellation, and timeout
- Retry classification, including read-only POST and side-effecting GET cases
- Record and mailmap reconciliation and duplicate-value identification
- Import parsers and canonicalization
- Attribute and cross-field validators

Target at least 85% statement coverage for client, normalization, validation,
and reconciliation packages. Aggregate framework boilerplate coverage is not a
release criterion.

### Local HTTP contract tests

Every operation in `api/coverage.csv` receives local HTTP tests for applicable
cases:

- Documented success response
- Empty success body
- String/number/null scalar variants
- 403 authorization failure
- 409 conflict
- 420 throttling
- 429 throttling if observed
- Transient 5xx
- Malformed or oversized error body
- Connection failure and context cancellation

Fixtures in `testdata/contracts` are safe local inputs. Schema-derived fixtures
are illustrative until a dedicated sandbox observation verifies them.

### Terraform framework tests

Use provider factories and a fake EasyDNS server to test:

- Provider defaults, environment variables, aliases, and conflicting settings
- Resource schemas, plan modifiers, timeouts, diagnostics, and sensitive flags
- Create/read/update/delete and disappearance behavior
- Import-state parsing and state equivalence
- Data-source singleton and collection behavior
- Both record write modes against the same lifecycle assertions
- Action configuration, planning, and invocation
- State upgrades and `use_async_api` compatibility

### Sandbox acceptance tests

Acceptance tests use real Terraform commands and a dedicated EasyDNS sandbox.
They must:

- Require `TF_ACC` and a second provider-specific opt-in
- Require dedicated sandbox credentials from CI secrets
- Parse the configured URL and accept only the exact EasyDNS sandbox hostname
- Reject production even if a production-related environment variable is set
- Use a dedicated disposable sandbox domain and unique per-test prefixes
- Register cleanup immediately after successful creation
- Verify remote state in addition to Terraform state
- Test apply, empty follow-up plan, update, import, drift/disappearance, and
  destroy for each managed resource
- Run synchronous and asynchronous record cases separately

Domain registration and deletion have an additional manual gate and run in a
separate job. They never share credentials with normal record tests.

### Static and supply-chain checks

- `gofmt`/format check
- `go vet`
- static analysis/lint
- race-enabled unit and local contract tests
- dependency vulnerability scanning
- generated documentation drift
- Terraform example formatting and validation
- OpenAPI snapshot/coverage validation

## CI shape

| Trigger | Jobs |
|---|---|
| Every pull request | Format, unit, contract, framework, race, static, docs/coverage drift |
| Scheduled/manual | Read-only sandbox contract smoke tests |
| Manual with protected environment | Sandbox mutation acceptance tests |
| Manual with separate approval | Sandbox registration/deletion tests |
| Release tag | Full non-production suite, build, sign, checksum, Registry doc verification |

## Required lifecycle cases

Every resource must cover normal create/read, update where supported, import,
out-of-band disappearance, remote drift, not-found, and protected deletion.
Resources with state-only removal semantics must state that behavior explicitly
and test it. The recommended design avoids state-only deletion for domains.

Imperative actions must prove that a transient response cannot trigger an
automatic replay. Read-only operations using a non-GET HTTP method must prove
the inverse: they retain bounded read retry behavior and never surface as an
ambiguous write.

## Known live-contract gap

No authenticated sandbox calls were made through Phase 5 implementation. The
protected CI workflows must use a dedicated disposable sandbox account to
exercise the acceptance suites and capture only sanitized representative
responses for gaps listed in `testdata/contracts/README.md`. Real-domain or
production credentials are not acceptable for that activity.
