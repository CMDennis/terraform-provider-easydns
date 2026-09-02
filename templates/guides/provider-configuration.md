---
page_title: "Authentication and provider configuration"
subcategory: "Guides"
description: |-
  Configure EasyDNS credentials, environments, record modes, and safety switches.
---

# Authentication and provider configuration

## Recommended configuration

Put non-secret behavior in Terraform and credentials in the environment:

```terraform
provider "easydns" {
  environment       = "sandbox"
  record_write_mode = "synchronous"
}
```

```shell
export EASYDNS_API_TOKEN='...'
export EASYDNS_API_KEY='...'
```

`api_token` and `api_key` are marked sensitive, but values written in provider
configuration may still be retained in local files, shell history, automation
metadata, or state-related artifacts. Environment variables or a CI secret
store reduce accidental exposure.

## Settings and precedence

| Setting | Environment variable | Default |
|---|---|---|
| `environment` | `EASYDNS_ENVIRONMENT` | `sandbox` |
| `api_url` | `EASYDNS_API_URL` | URL selected by `environment` |
| `api_token` | `EASYDNS_API_TOKEN` | none |
| `api_key` | `EASYDNS_API_KEY` | none |
| `record_write_mode` | `EASYDNS_RECORD_WRITE_MODE` | `synchronous` |
| `enable_domain_registration` | `EASYDNS_ENABLE_DOMAIN_REGISTRATION` | `false` |
| `use_async_api` (deprecated) | `EASYDNS_USE_ASYNC_API` | `false` |

An explicitly configured Terraform attribute takes precedence over its
environment variable. `api_url` takes precedence over `environment`; reserve it
for development proxies and controlled tests.

## Environment URLs

- `sandbox`: `https://sandbox.rest.easydns.net`
- `production`: `https://rest.easydns.net`

Changing from sandbox to production changes the account and remote objects the
same configuration addresses. Use separate state, credentials, and CI
environments. Never reuse sandbox state against production.

## Compatibility setting

`use_async_api` remains supported through v1.x for v0.1 configurations. New
configurations should use `record_write_mode`. Configuring both is an error
because two sources of truth would make write behavior unclear.

## Registration opt-in

`enable_domain_registration` permits a billable registry operation but does
not register anything by itself. Registration also requires an
`easydns_domain` with `dns_only = false`. Leave the provider opt-in disabled in
DNS-only workspaces.
