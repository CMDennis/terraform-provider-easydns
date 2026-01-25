---
page_title: "EasyDNS Provider"
subcategory: ""
description: |-
  The EasyDNS provider is used to manage DNS records and zones via the EasyDNS REST API.
---

# EasyDNS Provider

The EasyDNS provider allows you to manage DNS records and zones hosted on [EasyDNS](https://easydns.com/) using their REST API.

## Features

- Manage DNS records (A, AAAA, CNAME, MX, TXT, SRV, CAA, and more)
- Import and track existing zones
- Support for sandbox and production environments
- Async API support for better rate limiting
- Input validation for hostnames, IP addresses, and record types

## Example Usage

```terraform
terraform {
  required_providers {
    easydns = {
      source = "CMDennis/easydns"
    }
  }
}

provider "easydns" {
  environment = "production"
  # Credentials via environment variables:
  # EASYDNS_API_TOKEN, EASYDNS_API_KEY
}

# Look up zone info
data "easydns_zone" "main" {
  domain = "example.com"
}

# Create an A record
resource "easydns_record" "web" {
  domain = data.easydns_zone.main.domain
  host   = "www"
  type   = "A"
  rdata  = "192.0.2.1"
  ttl    = 3600
}
```

## Authentication

The EasyDNS provider requires API credentials (token and key) which can be obtained from your EasyDNS account dashboard.

Credentials can be provided in two ways:

1. **Provider configuration** (not recommended for production):

```terraform
provider "easydns" {
  api_token = "your-token"
  api_key   = "your-key"
}
```

2. **Environment variables** (recommended):

```bash
export EASYDNS_API_TOKEN="your-token"
export EASYDNS_API_KEY="your-key"
```

## Schema

### Optional

- `environment` (String) API environment: `sandbox` or `production`. Defaults to `sandbox`. Can also be set via `EASYDNS_ENVIRONMENT` environment variable.
- `api_url` (String) Custom API URL. Overrides `environment` setting. Can also be set via `EASYDNS_API_URL` environment variable.
- `api_token` (String, Sensitive) EasyDNS API token. Can also be set via `EASYDNS_API_TOKEN` environment variable.
- `api_key` (String, Sensitive) EasyDNS API key. Can also be set via `EASYDNS_API_KEY` environment variable.
- `use_async_api` (Boolean) Use the async API for record operations. This queues zone reloads instead of processing immediately, which may help with rate limiting. Defaults to `false`. Can also be set via `EASYDNS_USE_ASYNC_API` environment variable.

### Environment URLs

| Environment | URL |
|-------------|-----|
| sandbox | `https://sandbox.rest.easydns.net` |
| production | `https://rest.easydns.net` |
