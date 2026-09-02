# ADR-0001: Target platform and compatibility policy

- Status: Accepted
- Date: 2026-09-01

## Context

The provider currently uses Terraform Plugin Framework v1.4.2 and protocol 6.
The EasyDNS API includes imperative operations, notably forced zone reload and
setting a primary nameserver, that should not be modeled as durable resources.
Terraform provider actions accurately represent those operations but require
Terraform 1.14 or newer.

The provider has public v0.1.x releases and existing users may already have
`easydns_zone` state and `use_async_api` configurations.

## Decision

- v1 targets Terraform CLI 1.14 or newer, Terraform Plugin Protocol 6, and a
  Terraform Plugin Framework release with action support (at least v1.18).
- `easydns_force_zone_reload` and `easydns_set_primary_nameserver` are actions,
  not resources.
- `use_async_api` is deprecated when `record_write_mode` ships, but remains
  supported through v1.x. Supplying conflicting values is a configuration
  error.
- The existing `easydns_zone` resource and data source remain available in
  v1.x as compatibility surfaces. They receive deprecation notices after
  `easydns_domain` ships and are not removed before v2.0.
- State schema migrations are required for any released resource schema change
  that cannot be refreshed safely from existing state.

## Consequences

- The provider gains a truthful model for imperative endpoints and a clear
  compatibility policy while it is still pre-v1.
- Users pinned below Terraform 1.14 must remain on the last compatible provider
  release.
- The framework upgrade is a Phase 1 prerequisite and needs protocol/schema
  regression tests.

## Revisit when

- Terraform actions become usable with an older supported CLI, or
- registry/user demand justifies a separately maintained compatibility branch.
