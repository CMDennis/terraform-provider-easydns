# Phase 1 — Client and Provider Foundation

## Outcome

Phase 1 is implemented in the safe working copy at
[`provider/`](../../provider/). The implementation does not contain credentials
and its automated tests use local HTTP servers only.

The original source tree at
`/Users/bwolf/Desktop/workshop/repos/easydnsterraform` was not modified. The
working copy was made from its tracked working tree so the pre-existing edits
to `record_resource.go` and `validators.go` remain present for later review.

## What changed

### Typed EasyDNS client

HTTP and wire-format behavior now lives in `provider/internal/client` rather
than the Terraform provider package. Records and zones expose typed,
context-aware methods, and every resource and data source passes its Terraform
operation context through to the client.

The client provides:

- Synchronous and asynchronous record-write routes
- A 30-second default HTTP timeout
- Path-segment escaping and base-URL validation
- Record pagination with a no-progress guard
- Narrow coercion for documented string, number, boolean-like, and null
  response variations
- Typed not-found, API, ambiguous-write, empty-response, and response-size
  errors
- Bounded response bodies and redacted error messages
- Basic authentication and consistent request headers

### Throttling and retries

Each configured client limits requests to one per second by default. Reads use
bounded retries for transport failures and HTTP 420, 429, 500, 502, 503, and
504 responses. Retry delays honor `Retry-After`, otherwise use bounded
exponential backoff with jitter.

Create, update, and delete requests are attempted exactly once. Transport,
response-read, empty-body, and decode failures after a mutation are returned
as typed ambiguous-write errors instead of being blindly replayed. Phase 2
adds resource-level read reconciliation and eventual-consistency waiters; this
separation prevents the client foundation from inventing Terraform state
before the DNS lifecycle rules are finalized.

Clocks, waiters, retry jitter, HTTP clients, and transports are injectable so
timing and failure behavior can be tested without sleeping or contacting
EasyDNS.

### Provider and toolchain

- Provider resources and data sources use the extracted client without a
  public schema change.
- `use_async_api` remains compatible; the planned write-mode replacement is a
  Phase 2 schema change.
- Terraform Plugin Framework is upgraded to v1.19.0.
- The Go language baseline is 1.25 and the target Terraform CLI baseline is
  1.14.
- The development guide documents architecture, dependency maintenance, and
  safe test commands.
- The CI workflow now checks module tidiness, formatting, vet, build, race
  tests, and coverage.

## Acceptance-test safety

Integration tests are compile-gated and runtime-gated. They require both
`TF_ACC=1` and `EASYDNS_ACC_SANDBOX=1`. The harness rejects HTTP, production,
lookalike hosts, credentials in URLs, path suffixes, query strings, fragments,
and nonstandard ports. It accepts only the official EasyDNS sandbox over
HTTPS.

Record mutations require a third explicit value:

```shell
EASYDNS_ACC_ALLOW_MUTATIONS=sandbox-writes-only
```

No acceptance test was executed during Phase 1. No EasyDNS key was read, and
no authenticated EasyDNS request was made.

## Verification

The following checks passed on September 1, 2026:

```text
make check
  gofmt check: passed
  go vet ./...: passed
  go test -race ./...: passed
  go build ./...: passed

go test -coverprofile=coverage.out ./...
  internal/client: 91.5% statement coverage
  all packages: 49.0% statement coverage

go test -tags=integration ./internal/provider -run '^$'
  compile-only integration check: passed; no tests run
```

The client coverage exceeds the Phase 0 target of 85%. Provider-framework
lifecycle coverage remains deliberately low and is expanded with Phase 2
resource work.

## Phase 2 handoff

Phase 2 should build on this foundation by adding:

1. Read-after-write reconciliation for empty or ambiguous mutation outcomes.
2. Bounded asynchronous create, update, and delete waiters.
3. Duplicate-record disambiguation when a create response has no record ID.
4. `geozone_id`, complete record-type support, and the planned write-mode
   schema with compatibility handling.
5. Terraform framework tests for import, drift, not-found behavior, and
   equivalent synchronous/asynchronous lifecycles.

See the provider's [contribution guide](../../provider/CONTRIBUTING.md) for
local commands and the [testing guide](../../provider/docs/guides/testing.md)
for the sandbox test gates. Both replaced the former
`provider/docs/development.md` when Phase 5 moved the provider onto generated
Registry documentation.
