# Terraform Provider for EasyDNS

A Terraform provider for managing DNS records and zones via the [EasyDNS REST API](https://docs.sandbox.rest.easydns.net:3001/).

## Features

- Manage DNS records, domains, registrar settings, delegation, glue, and mailmaps
- Query domain, service, subscription, pricing, user, and DNS metadata
- Invoke force-reload and primary-nameserver actions with Terraform 1.14+
- Support for sandbox and production environments
- Synchronous and asynchronous record writes with read reconciliation
- Input validation for hostnames, IP addresses, and record types
- Exactly-one-write mutation handling followed by read reconciliation

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.14
- [Go](https://go.dev/doc/install) >= 1.25 (for building)
- EasyDNS API credentials (token and key)

For a first sandbox configuration, follow the
[five-minute quick start](docs/guides/quick-start.md). The generated
[Registry documentation](docs/index.md) contains the complete provider,
resource, data-source, action, migration, safety, and troubleshooting guides.

## Installation

### From Terraform Registry

```hcl
terraform {
  required_providers {
    easydns = {
      source = "CMDennis/easydns"
    }
  }
}
```

### Building from Source

```bash
git clone https://github.com/CMDennis/terraform-provider-easydns.git
cd terraform-provider-easydns
go build -o terraform-provider-easydns
```

### Local Development Setup

Create or update `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/CMDennis/easydns" = "/path/to/terraform-provider-easydns"
  }
  direct {}
}
```

## Provider Configuration

```hcl
terraform {
  required_providers {
    easydns = {
      source = "CMDennis/easydns"
    }
  }
}

provider "easydns" {
  environment       = "sandbox"     # or "production"
  api_token         = "your-token"  # or use EASYDNS_API_TOKEN env var
  api_key           = "your-key"    # or use EASYDNS_API_KEY env var
  record_write_mode = "synchronous" # or "asynchronous"
}
```

### Configuration Reference

| Attribute | Description | Default | Environment Variable |
|-----------|-------------|---------|---------------------|
| `environment` | API environment: `sandbox` or `production` | `sandbox` | `EASYDNS_ENVIRONMENT` |
| `api_url` | Custom API URL (overrides environment) | - | `EASYDNS_API_URL` |
| `api_token` | EasyDNS API token | - | `EASYDNS_API_TOKEN` |
| `api_key` | EasyDNS API key | - | `EASYDNS_API_KEY` |
| `record_write_mode` | Default record mutation mode: `synchronous` or `asynchronous` | `synchronous` | `EASYDNS_RECORD_WRITE_MODE` |
| `enable_domain_registration` | Explicit opt-in for billable registry registration | `false` | `EASYDNS_ENABLE_DOMAIN_REGISTRATION` |
| `use_async_api` | Deprecated Boolean compatibility alias | `false` | `EASYDNS_USE_ASYNC_API` |

### Environment URLs

| Environment | URL |
|-------------|-----|
| Sandbox | `https://sandbox.rest.easydns.net` |
| Production | `https://rest.easydns.net` |

### Record write modes

Synchronous writes request an immediate zone change. Asynchronous writes queue
zone regeneration. Both modes use the same Terraform lifecycle and wait until
the requested record is visible, updated, or absent. A write is issued only
once; uncertain results are reconciled through reads.

```hcl
provider "easydns" {
  record_write_mode = "asynchronous"
}
```

Or via environment variable:

```bash
export EASYDNS_RECORD_WRITE_MODE=asynchronous
```

`use_async_api` remains available for v0.1 compatibility but is deprecated.
Do not configure both settings.

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
| `geozone_id` | number | No | EasyDNS geo-region ID; zero means global (default: 0) |
| `write_mode` | string | No | Per-record `synchronous` or `asynchronous` override |

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
| `geozone_id` | EasyDNS geo-region ID |
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
| `search_keyword` | string | No | Optional EasyDNS server-side search keyword |

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
| `geozone_id` | EasyDNS geo-region ID |
| `last_mod` | Last modification timestamp |

---

### Additional DNS data sources

- `easydns_parsed_records` returns EasyDNS-expanded zone-file records,
  including `url` and `orig_rdata`.
- `easydns_zone_soa` returns the current SOA serial.
- `easydns_geo_regions` returns stable ID-sorted regions and follows all pages
  unless `start` or `max` requests one page.

See the pages in `docs/data-sources/` for complete schemas and examples.

---

### Mailmaps, metadata, and pricing

- `easydns_mailmap` manages a forwarding address with an immutable numeric ID
  and exactly-one-write reconciliation.
- `easydns_mailmaps` lists a domain's maps in stable ID order.
- `easydns_current_user` returns authenticated-account metadata and marks all
  identity, address, phone, email, and URL fields sensitive.
- `easydns_service` and `easydns_subscription_service` describe service IDs.
- `easydns_domain_pricing` returns domain availability and account-specific
  exact-decimal prices without initiating registration.

### Actions

Terraform 1.14+ can invoke `easydns_force_zone_reload` and
`easydns_set_primary_nameserver`. These imperative operations are sent exactly
once and do not create durable resource state. See `docs/actions/` for complete
examples and the ambiguous-outcome safety procedure.

---

## Complete Example

```hcl
terraform {
  required_providers {
    easydns = {
      source = "CMDennis/easydns"
    }
  }
}

provider "easydns" {
  environment       = "sandbox"
  record_write_mode = "asynchronous"
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
| AFSDB | AFS database service | No |
| ANAME | EasyDNS apex alias | No |
| CAA | Certificate Authority Authorization | No |
| CERT | Certificate record | No |
| CNAME | Canonical name (alias) | No |
| DYN | EasyDNS dynamic record | No |
| MX | Mail exchange | Yes |
| NAPTR | Naming Authority Pointer | No |
| NS | Nameserver | No |
| PTR | Pointer record | No |
| SECONDARY | EasyDNS secondary DNS record | No |
| SOA | Start of Authority | No |
| SPF | Sender Policy Framework (legacy) | No |
| SRV | Service record | Yes |
| SSHFP | SSH fingerprint | No |
| STEALTH | EasyDNS stealth forwarding | No |
| TLSA | TLS Authentication | No |
| TXT | Text record | No |
| URL | EasyDNS URL forwarding | No |
| URLHTTPS | EasyDNS HTTPS URL forwarding | No |

---

## Development

See [testing and contributing](docs/guides/testing.md),
[architecture and API coverage](docs/guides/architecture-and-coverage.md), and
the [release procedure](docs/guides/releasing.md) for the development workflow.

### Building

```bash
go build -o terraform-provider-easydns
```

### Testing

```bash
# Complete local quality suite
make check

# Read-only sandbox integration tests (requires sandbox credentials)
export TF_ACC=1
export EASYDNS_ACC_SANDBOX=1
export EASYDNS_API_TOKEN="your-token"
export EASYDNS_API_KEY="your-key"
export EASYDNS_TEST_DOMAIN="your-domain.com"
go test -tags=integration -v ./internal/provider -run TestIntegration

# Record mutation tests require this additional explicit opt-in:
export EASYDNS_ACC_ALLOW_MUTATIONS=sandbox-writes-only
```

### Project Structure

```
terraform-provider-easydns/
├── main.go                           # Provider entry point
├── go.mod                            # Go module
├── internal/client/                  # Typed, context-aware EasyDNS client
├── internal/provider/
│   ├── provider.go                   # Provider configuration
│   ├── client.go                     # Compatibility aliases for the client
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
