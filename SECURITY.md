# Security policy

## Reporting a vulnerability

Use GitHub's private vulnerability reporting for this repository when it is
available. Otherwise contact the maintainer privately through the security
contact published on the repository profile. Do not open a public issue before
a fix is available.

Include the affected version, impact, a minimal sanitized reproduction, and any
suggested mitigation. Never send active EasyDNS credentials, Terraform state,
registrant contact data, or production domain details.

## Supported versions

Security fixes are provided for the latest released minor version. Pre-v1
versions may require upgrading to receive a fix. Release notes will identify
the first fixed version and any credential-rotation or state-remediation steps.

## Secrets and state

Provider credentials should come from environment variables or a secret
manager. Sensitive Terraform attributes are hidden in ordinary CLI output but
remain in state; use an encrypted, access-controlled backend.
