# Phase 3 — Domains and Registrar Controls

## Outcome

Phase 3 is implemented in the safe [`provider/`](../../provider/) working
copy. It adds the domain lifecycle, registrar policy, delegation, and glue
records, and starts the `easydns_zone` deprecation.

No EasyDNS credential was read and no authenticated EasyDNS request was made.
Every test uses local HTTP servers, synthetic data, and reserved `.invalid`
domains. No domain was registered, deleted, or billed.

## Terraform surface

### Resources

| Resource | Purpose |
|---|---|
| `easydns_domain` | DNS-only domain addition, or guarded registration |
| `easydns_domain_registration_settings` | Registrar lock and renewal policy |
| `easydns_domain_nameservers` | Complete delegation set |
| `easydns_glue_record` | Registry glue CRUD and propagation status |

### Data sources

| Data source | Purpose |
|---|---|
| `easydns_domain` | Domain details; replaces the `easydns_zone` data source |
| `easydns_domains` | Every domain on a user, sorted by name |
| `easydns_domain_registration_statuses` | Account-wide reglock and renewal policy |
| `easydns_domain_nameservers` | Current delegation |
| `easydns_glue_records` | Every glue record a domain provides |

## Registrar safety

ADR-0003 is implemented rather than merely documented.

- `enable_domain_registration` defaults to `false` on the provider. A plan that
  would register a domain fails during `terraform plan`, before any request.
  The client enforces the same rule independently, so a registration cannot be
  issued through a client that was not configured to allow it.
- `easydns_domain.deletion_protection` defaults to `true`. Destroy fails while
  it is on, and the provider never substitutes a state-only removal.
- An imported domain always starts protected regardless of configuration.
- Premium registration requires the `premium` acknowledgement, the verified
  `premium_price`, and a `max_premium_price` ceiling. Prices are compared as
  exact decimals through `math/big`, never as floats, so nothing is accepted
  through rounding.
- `domain`, `service`, `term`, `currency`, and `dns_only` are immutable. A
  change produces a diagnostic naming the attribute and never schedules an
  automatic destroy-and-recreate of a real domain.
- Contacts and TLD registration fields are marked sensitive, with documentation
  stating plainly that Terraform still writes them to state.
- Destroying `easydns_domain_registration_settings` or
  `easydns_domain_nameservers` stops management and changes nothing remotely.
  Both warn on destroy so the behavior is visible in plan output.

Each of these guards has a test that fails when the guard is removed.

## Write safety

Every Phase 3 mutation follows the Phase 1 and 2 rule: exactly one write, then
reconciliation by reading. No write is replayed after an ambiguous transport
outcome.

- Domain creation reads the domain back when the response carries no data or
  the write was ambiguous.
- Domain deletion polls until the domain is off the system.
- Nameserver updates poll until the delegation matches the requested set.
- Glue creation and updates poll the domain's glue collection until the host
  shows the requested addresses.
- A registry refusal to delete glue still in use is surfaced, never retried.
- Registration settings ignore `reglock` on a TLD reporting
  `supports_reglock = false`, which would otherwise poll until the deadline.

## Contract ambiguities

Two pinned schemas disagree with the operations they describe. Both are handled
rather than guessed, and both are covered by fixtures:

- `getRegStatus` is typed as one domain but returns a whole account. Decoding
  accepts a domain-keyed object, an array of self-naming objects, and a bare
  single-domain object.
- `setRegStatus` types its body as an array while its own example shows a
  domain-keyed object. The provider sends the array form, matching the schema.

`flexibleBool` was added for the same reason: EasyDNS spells booleans as
`true`/`false`, `1`/`0`, `"1"`/`"0"`, and `"Y"`/`"N"` across these endpoints.

A dedicated sandbox observation is needed to settle both shapes; until then
they stay schema-derived in `testdata/contracts/`.

## Deprecating `easydns_zone`

`easydns_zone` and its data source now carry deprecation messages naming
`easydns_domain` as the replacement. Both keep working through v0.x and v1.x
and are removed in v2.0.0. The migration path — including the
`terraform state rm` and `terraform import` sequence, and why the plan must be
empty before applying — is in the provider's zone migration guide.

## Verification

| Gate | Result |
|---|---|
| `make check` (tidy, gofmt, vet, `-race` tests, build) | pass |
| `go test -tags=integration ./internal/provider -run '^$'` | pass, compile-only |
| `terraform fmt -check -recursive` | clean |
| `scripts/validate-phase-0.sh` | 36/36 operations mapped |
| Schema, docs, and example drift | none |

Statement coverage is 82.8% for `internal/client`, 28.0% for
`internal/provider`, and 56.0% overall. The API coverage matrix now records 24
complete operations, 10 planned, and 2 deliberate exclusions.

Every documented attribute and every attribute used in `provider/examples/` was
checked against the live protocol schema; there is no drift. Making that check
an automated gate is Phase 5 work.

## Phase 4 handoff

Phase 4 adds mailmaps, the user, service, subscription, and pricing data
sources, and the two provider actions. It can reuse the Phase 3 primitives
directly:

- `flexibleBool` and the envelope decoding patterns for loose scalar shapes
- The one-write-then-reconcile helpers for mailmap CRUD
- `configureDataSourceClient` for new read models
- The sensitive-attribute treatment established for contacts, which the user
  data source needs for its PII-bearing fields

`easydns_domain_pricing` pairs naturally with `easydns_domain.premium_price`:
it is the endpoint that supplies the verified price the premium ceiling checks
against. Wiring the two together in documentation is worth doing when the
pricing data source lands.
