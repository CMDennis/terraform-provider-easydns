# ADR-0003: Registrar safety boundaries

- Status: Accepted
- Date: 2026-09-01

## Context

Adding or registering a domain can incur charges and stores contact PII.
Deleting a domain can interrupt DNS and registrar service. The available local
credentials may be associated with production domains, so an ordinary test or
configuration mistake must not be able to mutate them.

## Decision

- `enable_domain_registration` defaults to `false` at provider level.
- `easydns_domain.deletion_protection` defaults to `true`.
- Registration fails during planning unless provider registration is enabled.
- Deletion fails while deletion protection is enabled; it does not silently
  remove only Terraform state.
- Premium registration requires both explicit premium opt-in and a maximum
  accepted premium price. A returned price above the maximum is an error.
- Immutable domain identity and registration fields do not use automatic
  destroy-and-recreate replacement. A requested change produces a diagnostic
  with migration guidance.
- Contact and user PII attributes are marked sensitive, with documentation that
  sensitive values still exist in Terraform state.
- Automated acceptance tests hard-fail unless the configured API host is the
  official sandbox. The existing production test switch is removed.
- Domain registration and deletion acceptance tests require an additional
  explicit environment gate and dedicated disposable sandbox account/domain.
- No production credential is stored in the project or CI.

## Consequences

- Destructive registrar operations require deliberate configuration changes.
- A protected domain prevents `terraform destroy` until protection is removed;
  users must not expect a state-only delete.
- Some acceptance paths remain manual because cleanup cannot be assumed after
  registration failures.

## Revisit when

- EasyDNS offers scoped test credentials or first-class disposable sandbox
  registrations, or
- Terraform adds a standard provider-level destructive-operation permission
  mechanism that is clearer for users.
