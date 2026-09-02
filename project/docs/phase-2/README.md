# Phase 2 — DNS Core

## Outcome

Phase 2 is implemented in the safe [`provider/`](../../provider/) working
copy. It completes the DNS-record lifecycle for synchronous and asynchronous
writes and adds the remaining DNS-focused read models from the pinned EasyDNS
OpenAPI contract.

No EasyDNS credential was read and no authenticated EasyDNS request was made.
All implementation tests use local HTTP servers, synthetic records, and
reserved `.invalid` domains.

## Record lifecycle

The provider now supports a default provider setting plus a resource override:

```terraform
provider "easydns" {
  record_write_mode = "synchronous" # or "asynchronous"
}

resource "easydns_record" "example" {
  domain     = "example.invalid"
  host       = "www"
  type       = "A"
  rdata      = "192.0.2.10"
  ttl        = 600
  geozone_id = 0
  write_mode = "asynchronous" # optional override
}
```

The deprecated `use_async_api` setting remains compatible when
`record_write_mode` is absent. Configuring both is rejected.

### Safe reconciliation

- Create lists record IDs before issuing exactly one mutation.
- If the response has an ID, that ID must become observable with the desired
  semantic value.
- If the response has no ID or is ambiguous, only matching IDs absent from the
  pre-write snapshot are candidates.
- One new match succeeds; multiple new matches return a deterministic error
  listing their IDs; no match is polled until the deadline.
- Update issues one mutation and polls the known ID until all managed values
  match.
- Delete issues one mutation and polls until a successful list proves the ID
  is absent.
- Authentication, authorization, malformed responses, and non-transient read
  errors never masquerade as not-found results.

The default reconciliation deadline is two minutes and the poll interval is
two seconds. Terraform context cancellation stops rate-limit, retry, and
reconciliation waits.

## Record model and normalization

`easydns_record`, the singleton data source, and record collections now expose
`geozone_id`. The provider recognizes every record type enumerated by OpenAPI:

`A`, `AAAA`, `AFSDB`, `ANAME`, `CAA`, `CERT`, `CNAME`, `DYN`, `MX`, `NAPTR`,
`NS`, `PTR`, `SECONDARY`, `SOA`, `SPF`, `SRV`, `SSHFP`, `STEALTH`, `TLSA`,
`TXT`, `URL`, and `URLHTTPS`.

Domains become lower-case IDNA ASCII without one optional trailing dot. Hosts
become lower-case, record types upper-case, and IP addresses canonical text.
DNS-name targets compare case-insensitively without one trailing dot. Opaque
values such as TXT and URL content remain exact.

Imports require `domain:positive_numeric_record_id`; whitespace, malformed
domains, empty fields, extra delimiters, and non-numeric IDs are rejected.

## Data sources

| Data source | Phase 2 behavior |
|---|---|
| `easydns_record` | Case-normalized singleton lookup; rejects duplicate host/type matches |
| `easydns_records` | Stable numeric-ID ordering and optional `search_keyword` endpoint |
| `easydns_parsed_records` | Parsed records including `url` and `orig_rdata` |
| `easydns_zone_soa` | Current SOA serial with compatible response-shape decoding |
| `easydns_geo_regions` | All-page traversal and numeric-ID sorting, or an explicitly requested page |

## Test strategy and coverage

The Phase 2 suite emphasizes local contract and lifecycle tests:

- Equal lifecycle assertions for synchronous and asynchronous create routes
- Empty-success and ambiguous mutation reconciliation
- Exactly-one-write assertions
- Duplicate new-record identification
- Update visibility and delete absence polling
- Timeout cause preservation and cancellation-safe waits
- Import parsing, remote drift, not-found, and duplicate singleton behavior
- IDNA, address, DNS-target, and opaque-value normalization
- Search path escaping, scalar coercion, stable ordering, and pagination
- Provider/resource schema and compatibility registration checks
- Compile-only sandbox integration coverage

## Edge-case audit

The closing audit found and fixed three defects.

`easydns_zone_soa` and `easydns_zone` wrote a normalized domain back over the
`domain` argument. Terraform requires a data source to return its configured
arguments unchanged, so any non-canonical configured domain — `Example.Invalid.`
for instance — produced an inconsistent-result error instead of a serial or a
zone. Both data sources now leave `domain` exactly as configured, and
`TestDataSourcesReturnConfiguredArgumentsUnchanged` reads each one through the
framework and fails if a configured argument is altered.

The `easydns_record` singleton reported a lookup that matched nothing under the
"Record lookup was not unique" summary, which described the opposite failure.
Not-found and ambiguous matches now carry separate diagnostic summaries.

`make fmt-check` derived its file list from `rg`, so on a machine without
ripgrep the target passed without checking anything. The Makefile now formats
and checks with `gofmt` directly, matching what CI already ran.

## Verification

All Phase 2 gates pass:

| Gate | Result |
|---|---|
| `make check` (tidy, gofmt, vet, `-race` tests, build) | pass |
| `go test -tags=integration ./internal/provider -run '^$'` | pass, compile-only |
| `terraform fmt -check -recursive` | clean |
| `scripts/validate-phase-0.sh` | 36/36 operations mapped |

Statement coverage is 86.5% for `internal/client`, 26.8% for
`internal/provider`, and 60.3% overall. The provider figure stays well below the
client figure because resource and data-source lifecycles are exercised through
the framework rather than through end-to-end Terraform runs; the sandbox
acceptance suite that closes that gap is Phase 5 work.

No EasyDNS credential was read and no authenticated request was made during
verification.

## Phase 3 handoff

Phase 3 can build on the same typed client, normalization, pagination, and
reconciliation primitives for domains, registration settings, nameservers,
and glue records. Registrar mutations remain outside this phase and retain the
separate safety decisions in ADR-0003.
