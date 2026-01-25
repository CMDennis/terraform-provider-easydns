# Terraform Provider for EasyDNS

A Terraform provider for managing DNS records and zones via the [EasyDNS REST API](https://docs.sandbox.rest.easydns.net:3001/).

## Features

- Manage DNS records (A, AAAA, CNAME, MX, TXT, SRV, CAA, etc.)
- Import and track existing zones
- Support for sandbox and production environments
- Async API support for better rate limiting
- Input validation for hostnames, IP addresses, and record types

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.21 (for building)
- EasyDNS API credentials (token and key)

## Installation

### From Terraform Registry

```hcl
terraform {
  required_providers {
    easydns = {
      source = "easydns/easydns"
    }
  }
}
```

### Building from Source

```bash
git clone https://github.com/easydns/terraform-provider-easydns.git
cd terraform-provider-easydns
go build -o terraform-provider-easydns
```

### Local Development Setup

Create or update `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/easydns/easydns" = "/path/to/terraform-provider-easydns"
  }
  direct {}
}
```

## Provider Configuration

```hcl
terraform {
  required_providers {
    easydns = {
      source = "easydns/easydns"
    }
  }
}

provider "easydns" {
  environment   = "sandbox"    # or "production"
  api_token     = "your-token" # or use EASYDNS_API_TOKEN env var
  api_key       = "your-key"   # or use EASYDNS_API_KEY env var
  use_async_api = false        # optional: use async API for rate limiting
}
```

### Configuration Reference

| Attribute | Description | Default | Environment Variable |
|-----------|-------------|---------|---------------------|
| `environment` | API environment: `sandbox` or `production` | `sandbox` | `EASYDNS_ENVIRONMENT` |
| `api_url` | Custom API URL (overrides environment) | - | `EASYDNS_API_URL` |
| `api_token` | EasyDNS API token | - | `EASYDNS_API_TOKEN` |
| `api_key` | EasyDNS API key | - | `EASYDNS_API_KEY` |
| `use_async_api` | Use async API for record operations | `false` | `EASYDNS_USE_ASYNC_API` |

### Environment URLs

| Environment | URL |
|-------------|-----|
| Sandbox | `https://sandbox.rest.easydns.net` |
| Production | `https://rest.easydns.net` |

### Async API

The async API queues zone reloads instead of processing them immediately. This can help with rate limiting when making many changes. Enable it with:

```hcl
provider "easydns" {
  use_async_api = true
}
```

Or via environment variable:

```bash
export EASYDNS_USE_ASYNC_API=true
```

---

## Resources

### easydns_record

Manages a DNS record in EasyDNS.

#### Example Usage

```hcl
# A record
resource "easydns_record" "web" {
  domain = "example.com"
  host   = "www"
  type   = "A"
  rdata  = "192.0.2.1"
  ttl    = 3600
}

# CNAME record
resource "easydns_record" "blog" {
  domain = "example.com"
  host   = "blog"
  type   = "CNAME"
  rdata  = "www.example.com."
  ttl    = 3600
}

# MX record
resource "easydns_record" "mail" {
  domain = "example.com"
  host   = "@"
  type   = "MX"
  rdata  = "mail.example.com."
  ttl    = 3600
  prio   = 10
}

# TXT record (SPF)
resource "easydns_record" "spf" {
  domain = "example.com"
  host   = "@"
  type   = "TXT"
  rdata  = "v=spf1 include:_spf.google.com ~all"
  ttl    = 3600
}

# AAAA record (IPv6)
resource "easydns_record" "ipv6" {
  domain = "example.com"
  host   = "www"
  type   = "AAAA"
  rdata  = "2001:db8::1"
  ttl    = 3600
}
```

#### Argument Reference

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `domain` | string | Yes | The domain/zone name (e.g., `example.com`) |
| `host` | string | Yes | Hostname/subdomain. Use `@` for root, `*` for wildcard |
| `type` | string | Yes | Record type (A, AAAA, CNAME, MX, TXT, NS, SRV, CAA, etc.) |
| `rdata` | string | Yes | Record data (IP address, hostname, text value) |
| `ttl` | number | No | Time to live in seconds (default: 600) |
| `prio` | number | No | Priority for MX/SRV records, 0-100 (default: 0) |

#### Attribute Reference

| Attribute | Description |
|-----------|-------------|
| `id` | The unique record ID |
| `last_mod` | Last modification timestamp |

#### Import

Import existing records using the format `domain:record_id`:

```bash
terraform import easydns_record.web "example.com:12345678"
```

#### Validation

The provider validates:
- **Hostname**: Must contain only valid DNS characters (letters, digits, hyphens, underscores, dots, `@`, `*`)
- **Record type**: Warns on unknown types (allows API to reject)
- **IPv4 (A records)**: Must be valid IPv4 address
- **IPv6 (AAAA records)**: Must be valid IPv6 address
- **Priority**: Must be 0-100

---

### easydns_zone

Manages a DNS zone in EasyDNS. This resource is **import-only** - zones must be created in the EasyDNS dashboard first, then imported into Terraform.

#### Example Usage

```hcl
# Import an existing zone
resource "easydns_zone" "main" {
  domain = "example.com"
}

# Use the zone in records
resource "easydns_record" "www" {
  domain = easydns_zone.main.domain
  host   = "www"
  type   = "A"
  rdata  = "192.0.2.1"
}
```

#### Argument Reference

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `domain` | string | Yes | The domain name of the zone |

#### Attribute Reference

| Attribute | Description |
|-----------|-------------|
| `id` | Zone identifier (same as domain) |
| `exists` | Whether the zone exists |
| `on_system` | Whether the zone is active on EasyDNS |
| `expiry` | Domain registration expiry date |
| `next_due` | Next billing due date |
| `service` | Service ID associated with the zone |

#### Import

```bash
terraform import easydns_zone.main example.com
```

#### Behavior

| Action | Behavior |
|--------|----------|
| Create | Adopts existing zone (errors if zone doesn't exist) |
| Read | Fetches current zone info from API |
| Delete | Removes from Terraform state only - does NOT delete from EasyDNS |

---

## Data Sources

### easydns_zone

Fetches information about a DNS zone.

#### Example Usage

```hcl
data "easydns_zone" "main" {
  domain = "example.com"
}

output "zone_service" {
  value = data.easydns_zone.main.service
}
```

#### Argument Reference

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `domain` | string | Yes | The domain name to look up |

#### Attribute Reference

| Attribute | Description |
|-----------|-------------|
| `id` | Zone identifier |
| `exists` | Whether the zone exists |
| `on_system` | Whether the zone is active |
| `expiry` | Domain expiry date |
| `next_due` | Next billing due date |
| `service` | Service ID |

---

### easydns_record

Fetches a specific DNS record by domain, host, and type.

#### Example Usage

```hcl
data "easydns_record" "www" {
  domain = "example.com"
  host   = "www"
  type   = "A"
}

output "www_ip" {
  value = data.easydns_record.www.rdata
}
```

#### Argument Reference

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `domain` | string | Yes | The domain/zone name |
| `host` | string | Yes | The hostname/subdomain |
| `type` | string | Yes | The record type |

#### Attribute Reference

| Attribute | Description |
|-----------|-------------|
| `id` | Record ID |
| `rdata` | Record data |
| `ttl` | Time to live |
| `prio` | Priority |
| `last_mod` | Last modification timestamp |

---

### easydns_records

Fetches all DNS records for a domain.

#### Example Usage

```hcl
data "easydns_records" "all" {
  domain = "example.com"
}

# Get all A records
output "a_records" {
  value = [for r in data.easydns_records.all.records : r if r.type == "A"]
}

# Get record count
output "total_records" {
  value = length(data.easydns_records.all.records)
}
```

#### Argument Reference

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `domain` | string | Yes | The domain/zone name |

#### Attribute Reference

| Attribute | Description |
|-----------|-------------|
| `records` | List of all records in the domain |

Each record in `records` contains:

| Attribute | Description |
|-----------|-------------|
| `id` | Record ID |
| `host` | Hostname |
| `type` | Record type |
| `rdata` | Record data |
| `ttl` | Time to live |
| `prio` | Priority |
| `last_mod` | Last modification timestamp |

---

## Complete Example

```hcl
terraform {
  required_providers {
    easydns = {
      source = "easydns/easydns"
    }
  }
}

provider "easydns" {
  environment   = "sandbox"
  use_async_api = true  # Enable async API for rate limiting
  # Credentials via EASYDNS_API_TOKEN and EASYDNS_API_KEY
}

# Import existing zone
resource "easydns_zone" "main" {
  domain = "example.com"
}

# Web server A record
resource "easydns_record" "web" {
  domain = easydns_zone.main.domain
  host   = "www"
  type   = "A"
  rdata  = "192.0.2.10"
  ttl    = 3600
}

# API subdomain
resource "easydns_record" "api" {
  domain = easydns_zone.main.domain
  host   = "api"
  type   = "CNAME"
  rdata  = "www.example.com."
  ttl    = 3600
}

# Mail server
resource "easydns_record" "mx" {
  domain = easydns_zone.main.domain
  host   = "@"
  type   = "MX"
  rdata  = "mail.example.com."
  ttl    = 3600
  prio   = 10
}

# SPF record
resource "easydns_record" "spf" {
  domain = easydns_zone.main.domain
  host   = "@"
  type   = "TXT"
  rdata  = "v=spf1 mx -all"
  ttl    = 3600
}

# Outputs
output "zone_info" {
  value = {
    domain  = easydns_zone.main.domain
    service = easydns_zone.main.service
  }
}

output "web_server" {
  value = easydns_record.web.rdata
}
```

---

## Supported Record Types

| Type | Description | Priority Used |
|------|-------------|---------------|
| A | IPv4 address | No |
| AAAA | IPv6 address | No |
| CNAME | Canonical name (alias) | No |
| MX | Mail exchange | Yes |
| TXT | Text record | No |
| NS | Nameserver | No |
| SRV | Service record | Yes |
| CAA | Certificate Authority Authorization | No |
| PTR | Pointer record | No |
| SPF | Sender Policy Framework (legacy) | No |
| ANAME | ALIAS record | No |
| SSHFP | SSH fingerprint | No |
| TLSA | TLS Authentication | No |

---

## Development

### Building

```bash
go build -o terraform-provider-easydns
```

### Testing

```bash
# Unit tests
go test -v ./...

# Integration tests (requires API credentials)
export EASYDNS_API_TOKEN="your-token"
export EASYDNS_API_KEY="your-key"
export EASYDNS_TEST_DOMAIN="your-domain.com"
go test -tags=integration -v ./internal/provider -run TestIntegration
```

### Project Structure

```
terraform-provider-easydns/
├── main.go                           # Provider entry point
├── go.mod                            # Go module
├── internal/provider/
│   ├── provider.go                   # Provider configuration
│   ├── client.go                     # EasyDNS API client
│   ├── client_test.go                # API client tests
│   ├── validators.go                 # Input validators
│   ├── record_resource.go            # easydns_record resource
│   ├── record_data_source.go         # easydns_record data source
│   ├── records_data_source.go        # easydns_records data source
│   ├── zone_resource.go              # easydns_zone resource
│   ├── zone_data_source.go           # easydns_zone data source
│   └── integration_test.go           # Integration tests
└── examples/
    └── main.tf                       # Example configuration
```

---

## License

MIT License - see [LICENSE](LICENSE) for details.
