# Phase 5 — Documentation, CI, and Release Hardening

## Outcome

Phase 5 is implemented in the safe [`provider/`](../../provider/) working copy.
The provider now has generated Terraform Registry documentation, deterministic
drift checks, layered CI, framework acceptance orchestration, a v0.1.1
migration path, dependency/security gates, and a verified release snapshot.

No EasyDNS credential was read and no authenticated API request was made. The
live suites were compiled but not executed because the available keys may
control real domains.

## Registry documentation

`tfplugindocs` v0.25.0 compiles the local provider and renders schema sections
from source. Templates retain the hand-written behavior, safety, import, and
example guidance. Generated coverage contains:

- one page for each of 7 resources, 16 data sources, and 2 actions;
- a five-minute sandbox quick start and provider authentication/configuration;
- synchronous/asynchronous record behavior;
- wildcard, geo, DKIM/DMARC, MX, SRV, and parked-record examples;
- registration, premium-price, deletion, state, PII, and CI-secret safety;
- timeout, rate-limit, ambiguous-write, and eventual-consistency diagnostics;
- v0.1.1 and deprecated-zone migration procedures;
- architecture/API coverage, testing/contribution, and release procedures.

`scripts/check-generated-docs.sh` renders a temporary copy and compares it to
`docs/`, so the check works even with unrelated uncommitted changes. The
Registry validator also checks that generated files match the provider surface.

## CI and test layers

Pull-request CI now runs:

- module tidiness, `gofmt`, `go vet`, builds, race tests, and coverage;
- unit, local HTTP contract, and framework suites;
- compile-only checks for `integration` and `acceptance` build tags;
- Terraform example parsing/formatting;
- generated-doc drift and Registry validation;
- golangci-lint v2.8.0 and actionlint v1.7.7;
- govulncheck v1.7.0;
- GoReleaser v2.18.0 configuration, unsigned snapshot, and checksum checks.

The acceptance harness uses Terraform Protocol 6 provider factories. Record
tests run the same create/read/update/import/empty-plan/delete lifecycle in
both synchronous and asynchronous modes. Additional suites cover mailmaps,
safe zone adoption, domain data, delegation, registrar settings, glue, a
disposable DNS-only domain, and state created by provider v0.1.1.

## Sandbox boundaries

The scheduled job is read-only. Every mutation suite is manual and bound to a
separate protected GitHub environment. Tests hard-code and validate
`https://sandbox.rest.easydns.net`; production, lookalike hosts, plain HTTP,
custom ports, credentials in URLs, and URL paths are rejected.

Records/mailmaps, registrar/delegation/glue, migration, and domain deletion
each require a different exact opt-in value. The domain lifecycle suite uses
the especially explicit `delete-disposable-sandbox-domain` gate. Fixtures must
be dedicated and disposable. Nameserver tests restore their initial set before
Terraform forgets the adoption-style resource.

## Supply chain and release

The provider keeps Go language compatibility at 1.25.8 and selects the
security-fixed Go 1.26.8 build toolchain. Direct networking/text dependencies
and gRPC were updated after govulncheck found reachable issues; the repeated
scan reports no vulnerabilities.

GoReleaser has an explicit `terraform-provider-easydns` project identity and
GitHub target. The local snapshot produced and checksum-verified ten Registry-
style archives across Darwin, Linux, FreeBSD, and Windows. Signing is skipped
only for snapshots. A tag release imports the protected GPG key, signs the
checksum file, verifies the archive checksums and detached signature, and runs
only after the full local-quality/documentation/security gate passes.

## Verification completed

The following non-live checks pass:

```shell
cd provider
make check
make docs-check docs-validate examples-check
make lint vuln actions-check
make release-check
```

The release snapshot and all ten archive checksums were verified. The pinned
OpenAPI matrix remains 34 implemented operations and 2 deliberate user-
mutation exclusions.

## Independent verification

Phase 5 was interrupted by a usage limit while its documentation templates were
being polished, so every claim above was re-checked from scratch rather than
trusted. All of them hold. Two gaps were found and closed:

- `docs/phase-1/README.md` still linked to `provider/docs/development.md`,
  which Phase 5 replaced with the contribution and testing guides. Every
  markdown link in the project now resolves.
- `make ci` ran only a subset of the pull-request workflow, so a local green
  result meant less than a green build. It now mirrors the workflow exactly.

The gates were also checked for substance, not just exit status:

- Injecting a line into a generated page makes `make docs-check` fail with the
  diff and a `run make docs` instruction, so the drift gate is real.
- The acceptance host guard was re-tested adversarially against production,
  suffix and hyphen lookalikes, a trailing-dot FQDN, an IDN lookalike, plain
  HTTP, a custom port, embedded credentials, and path traversal. It rejects
  all of them and accepts only the sandbox origin.
- Phase 4 was verified independently: 7 resources, 16 data sources, and 2
  actions are registered and match the planned surface; mailmap create, update,
  and delete each issue exactly one write and reconcile by reading; the
  semantically read-only pricing POST is retryable without being treated as an
  ambiguous write; and user PII attributes are sensitive.

`make ci` — the whole gate, end to end — passes.

## Remaining release evidence

Phase 5 implementation is complete, but v1 publication still requires the
protected sandbox acceptance/migration workflows to pass with genuinely
disposable fixtures and the tag workflow to verify a signature made with the
real release key. Those are external release gates, not safe local checks, and
were deliberately not simulated with real-domain credentials.
