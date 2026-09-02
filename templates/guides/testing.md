---
page_title: "Testing and contributing"
subcategory: "Development"
description: |-
  Run local checks and safely exercise the EasyDNS sandbox.
---

# Testing and contributing


## The sandbox request budget

The sandbox enforces the same limits as production: one request per second and
500 requests per day, resetting at midnight EST. A full acceptance sweep can
approach that ceiling, especially if any mutation reconciles slowly, so run one
suite at a time rather than the whole matrix. The scheduled workflow runs only
the read-only suite for this reason.

The acceptance suite therefore configures `record_poll_interval = "5s"`, which
cuts the reads a slow reconcile can spend from 60 to 24. Override it with
`EASYDNS_TEST_POLL_INTERVAL` if a run needs to settle faster.

### Sandbox balance

Creating a domain is invoiced at every service level, DNS-only included, so a
sandbox account with a zero balance cannot create a fixture at all. The API
answers `Request cost (NN.NN) exceeds user balance amount (0.00)`.

Sandbox balances are notional, not real charges, so this is a provisioning
step rather than a spending decision: ask EasyDNS to fund the sandbox account
and the create path works. It does mean the cost model is exercised
realistically, which is how the lite restriction below was found.

### Acceptance suite status against the sandbox

| Suite | Result |
|---|---|
| `TestAccReadOnlyDomain` | passes |
| `TestAccZoneResourceAdoption` | passes |
| `TestAccRecordResource` | passes, both write modes |
| `TestAccDomainRegistrationSettingsResource` | passes |
| `TestAccV011RecordMigration` | passes |
| `TestAccMailmapResource` | fails: update refused |
| `TestAccGlueRecordResource` | fails: delete refused |
| `TestAccDomainNameserversResource` | cannot run: delegation unreadable |
| `TestAccDomainResourceLifecycle` | steps pass; teardown fails, delete has no effect |

The four that do not pass are blocked by the sandbox, not by the provider.

- **Mailmap update** answers HTTP 406 "Access to resource denied due to context
  restrictions" for both the bare alias and the fully-qualified path, while
  create and delete on the same domain succeed.
- **Glue delete** answers HTTP 400 "Error attempting to delete glue record.
  Please contact support for assistance (no result)" for both path forms, so a
  created glue record cannot be removed. Glue creation itself works.
- **Delegation reads** fail for every domain: HTTP 400 "Authentication Error"
  for registered domains, and an empty body for a DNS-only domain. Nothing in
  `easydns_domain_nameservers` can be exercised.
- **Domain delete** is accepted with HTTP 200 and never takes effect. The
  suite's own steps — create, empty second plan, and import verification — all
  pass against a funded sandbox account; only Terraform's post-test destroy
  fails, and it reports the accepted-but-ineffective deletion explicitly.

Each leaves an object behind that the API cannot remove, so re-running those
suites accumulates junk on the account.

`easydns_domain` deletion is accepted with HTTP 200 and then does not take
effect: the domain keeps reporting `onsystem: "Y"` indefinitely. Every run of
`TestAccDomainResourceLifecycle` therefore leaves a domain behind that cannot
be removed through the API. That is the reason to run it sparingly — the
sandbox invoice is notional, but the accumulating domains are not.

A domain already registered elsewhere is reported `available = false` by
`easydns_domain_pricing` and cannot be used as a fixture.

## Local checks

Normal development needs no EasyDNS credentials:

```shell
make check
make coverage
make docs-check docs-validate
```

`make check` runs module-tidiness, formatting, vet, race tests, builds, compile
checks for both opt-in test tags, and Terraform example formatting. Client
contract tests use local HTTP servers and deterministic clocks/waiters.

Additional checks download pinned tool releases through Go:

```shell
make lint       # golangci-lint
make vuln       # govulncheck
make release-check
```

## Documentation workflow

Edit files under `templates/` and HCL under `examples/`, then run:

```shell
make docs
make docs-check docs-validate examples-check
```

`tfplugindocs` v0.25.0 renders schema sections by compiling the local provider.
The drift check renders a clean temporary copy and compares it with `docs/`.

## Acceptance-test safety

Acceptance tests are not part of ordinary `go test`. They require `TF_ACC=1`,
`EASYDNS_ACC_SANDBOX=1`, sandbox credentials, the exact official sandbox URL,
and dedicated fixtures. Mutation groups require additional exact gate values.

Run read-only checks first:

```shell
TF_ACC=1 EASYDNS_ACC_SANDBOX=1 \
EASYDNS_API_TOKEN=... EASYDNS_API_KEY=... \
EASYDNS_TEST_DOMAIN=disposable.example \
go test -tags=acceptance -v ./internal/provider \
  -run '^TestAcc(ReadOnlyDomain|ZoneResourceAdoption)$'
```

Use the GitHub Actions manual workflow for mutation suites. It separates
records/mailmaps, registrar settings/delegation/glue, v0.1.1 migration, and
domain lifecycle into protected environments. The domain lifecycle suite
deletes its dedicated sandbox domain and must never receive a real domain.

## Pull requests

Keep changes focused, add a regression test with each behavior change, update
templates and examples for public-schema changes, and explain safety or state
compatibility effects. Do not commit credentials, captured API bodies, state,
plans, customer domains, or contact data.
