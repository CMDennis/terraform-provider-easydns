# EasyDNS Terraform Provider Completion Project

This repository contains the plan, contract baseline, and safe implementation
workspace for completing the existing EasyDNS Terraform provider located at:

`/Users/bwolf/Desktop/workshop/repos/easydnsterraform`

Phase 0 is documentation- and contract-only. Phases 1 through 5 are implemented
in the local [`provider/`](provider/) working copy. No phase used EasyDNS
credentials, called authenticated account APIs, modified DNS records, or
registered, deleted, or billed a domain.

## Start here

- [Approved project plan](PROJECT_PLAN.md)
- [Phase 0 baseline and handoff](docs/phase-0/README.md)
- [Phase 1 implementation and handoff](docs/phase-1/README.md)
- [Phase 2 DNS core implementation and handoff](docs/phase-2/README.md)
- [Phase 3 domains and registrar controls](docs/phase-3/README.md)
- [Phase 4 mail, metadata, and actions](docs/phase-4/README.md)
- [Phase 5 documentation, CI, and release hardening](docs/phase-5/README.md)
- [Generated provider documentation](provider/docs/index.md)
- [Provider testing and contribution guide](provider/docs/guides/testing.md)
- [API coverage matrix](docs/api-coverage.md)
- [State and import conventions](docs/state-and-import-conventions.md)
- [Testing strategy](docs/testing-strategy.md)
- [Architecture decisions](docs/adr/README.md)
- [OpenAPI snapshot provenance](api/openapi/README.md)
- [Contract fixture policy](testdata/contracts/README.md)

## Validate the project

```shell
./scripts/validate-phase-0.sh
```

The validator checks that every operation in the pinned OpenAPI document has
exactly one disposition in `api/coverage.csv`, verifies the snapshot checksum,
and validates all JSON fixtures.

Run the provider checks separately:

```shell
cd provider
make check
make coverage
```
