# Contributing

Thank you for improving the EasyDNS provider. Keep pull requests focused and
include tests and documentation for user-visible behavior.

## Before opening a pull request

```shell
make check
make docs-check docs-validate
```

If installed, also run `make lint vuln`. Changes to release packaging should
pass `make release-check`.

Edit Registry content under `templates/` and Terraform snippets under
`examples/`; run `make docs` to update the generated `docs/` tree. Do not edit a
generated schema section directly.

Normal tests must not read EasyDNS credentials or contact the network. Add HTTP
contract fixtures for client changes and framework tests for schema or
lifecycle changes. Live acceptance testing is sandbox-only and uses the
separate, explicitly gated suites described in `docs/guides/testing.md`.

Never commit API credentials, Terraform state or plans, customer domains,
captured response bodies, or registrant contact data.
