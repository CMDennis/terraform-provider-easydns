---
page_title: "Security and sensitive data"
subcategory: "Guides"
description: |-
  Protect API credentials, Terraform state, contact PII, and CI fixtures.
---

# Security and sensitive data

## API credentials

- Use environment variables or a secret manager, not committed `.tf` or
  `.tfvars` files.
- Give CI a sandbox-only credential with the narrowest account access possible.
- Rotate credentials after suspected disclosure and remove them from both Git
  history and build logs.
- Do not enable Terraform debug logging in shared CI unless logs are access
  controlled and reviewed for sensitive payloads.

The client redacts authorization data and bounds error bodies, but callers and
proxies remain responsible for their own logging.

## Terraform state

Terraform's `sensitive` marking hides values from ordinary CLI presentation; it
does not encrypt state. Domain `contacts` and `extra` may contain names,
addresses, phone numbers, email addresses, birth information, or registration
identifiers.

Use an encrypted remote backend with access control, state locking, audit logs,
and retention appropriate for personal data. Restrict plans and saved plan
files too: they can contain proposed sensitive values.

## CI separation

Keep four boundaries distinct:

1. pull-request tests with no EasyDNS secrets;
2. scheduled read-only sandbox tests;
3. manually approved sandbox mutation tests on disposable records/mailmaps;
4. separately approved registrar and domain-deletion tests.

Never provide production credentials to an acceptance-test environment. The
test harness hard-codes and validates the official HTTPS sandbox endpoint, and
each mutation family needs an exact opt-in value.

## Reporting a vulnerability

Do not open a public issue containing an exploitable report or secrets. Follow
the private reporting route in the repository `SECURITY.md` and include only
sanitized reproduction material.
