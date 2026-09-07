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

API endpoints require HTTPS. HTTP is accepted only for literal loopback IP
addresses used by local test servers; DNS names such as `localhost` do not get
this exception. The client rejects every redirect, including redirects on reads,
so credentials and mutation bodies stay with the configured endpoint.

For structured API errors on ordinary reads, the client removes the configured
token, key, and Base64 Basic authorization value before limiting message length.
Requests with bodies and account, registration, and mail endpoints suppress
arbitrary server error text and report HTTP/API codes instead. Transport and
decode diagnostics use fixed categories rather than echoing URLs or rejected
response values. Response bodies are bounded and are never dumped into errors.
Callers, proxies, and external debug tooling remain responsible for their own
logging; these protections do not encrypt Terraform state or redact every
possible personal value from all user-supplied configuration diagnostics.

## Terraform state

Terraform's `sensitive` marking hides values from ordinary CLI presentation; it
does not encrypt state. Domain `contacts` and `extra` may contain names,
addresses, phone numbers, email addresses, birth information, or registration
identifiers.

Use an encrypted remote backend with access control, state locking, audit logs,
and retention appropriate for personal data. Restrict plans and saved plan
files too: they can contain proposed sensitive values.

## CI separation

The GitHub test and release workflows run without EasyDNS credentials. The
sandbox GitHub workflow has been removed. For optional local live tests, keep
these boundaries distinct:

1. read-only sandbox checks;
2. explicitly opted-in mutation tests on disposable records/mailmaps;
3. separately opted-in registrar and domain-deletion tests.

Never provide production credentials to an acceptance-test environment. The
test harness hard-codes and validates the official HTTPS sandbox endpoint, and
each mutation family needs an exact opt-in value.

## Reporting a vulnerability

Do not open a public issue containing an exploitable report or secrets. Follow
the private reporting route in the repository `SECURITY.md` and include only
sanitized reproduction material.
