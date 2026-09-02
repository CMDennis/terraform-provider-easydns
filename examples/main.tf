terraform {
  required_providers {
    easydns = {
      source = "registry.terraform.io/CMDennis/easydns"
    }
  }
}

provider "easydns" {
  # Environment: "sandbox" (default) or "production"
  environment = "sandbox"

  # Optional: choose "synchronous" or "asynchronous" record mutations.
  # Both modes wait until the requested state is observable.
  record_write_mode = "synchronous"

  # Credentials via environment variables:
  # EASYDNS_API_TOKEN, EASYDNS_API_KEY
}

# ============================================================================
# ZONE DATA & RESOURCES
# ============================================================================

# Look up zone info (read-only)
data "easydns_zone" "main" {
  domain = "example.com"
}

# Manage a zone (import-only - create zones in EasyDNS dashboard first)
# Import with: terraform import easydns_zone.main example.com
resource "easydns_zone" "main" {
  domain = "example.com"
}

# ============================================================================
# RECORD DATA SOURCES
# ============================================================================

# List all records in a domain
data "easydns_records" "all" {
  domain = easydns_zone.main.domain
}

# Server-side record search
data "easydns_records" "mail" {
  domain         = easydns_zone.main.domain
  search_keyword = "mail"
}

data "easydns_parsed_records" "all" {
  domain = easydns_zone.main.domain
}

data "easydns_zone_soa" "main" {
  domain = easydns_zone.main.domain
}

data "easydns_geo_regions" "all" {}

# Look up a specific record
data "easydns_record" "www" {
  domain = easydns_zone.main.domain
  host   = "www"
  type   = "A"
}

# ============================================================================
# RECORD RESOURCES
# ============================================================================

# Create an A record
resource "easydns_record" "app" {
  domain = easydns_zone.main.domain
  host   = "app"
  type   = "A"
  rdata  = "192.0.2.10"
  ttl    = 3600
  # geozone_id = 1
  # write_mode  = "asynchronous"
}

# Create a CNAME pointing to the A record
resource "easydns_record" "api" {
  domain = easydns_zone.main.domain
  host   = "api"
  type   = "CNAME"
  rdata  = "app.example.com."
  ttl    = 3600
}

# MX record
resource "easydns_record" "mx" {
  domain = easydns_zone.main.domain
  host   = "@"
  type   = "MX"
  rdata  = "mail.example.com."
  ttl    = 3600
  prio   = 10
}

# ============================================================================
# OUTPUTS
# ============================================================================

output "zone_info" {
  description = "Zone information"
  value = {
    domain    = data.easydns_zone.main.domain
    exists    = data.easydns_zone.main.exists
    on_system = data.easydns_zone.main.on_system
    service   = data.easydns_zone.main.service
  }
}

output "record_count" {
  description = "Number of records in the zone"
  value       = length(data.easydns_records.all.records)
}

output "www_ip" {
  description = "IP address of www record"
  value       = data.easydns_record.www.rdata
}

# ---------------------------------------------------------------------------
# Phase 3: domains, registrar controls, delegation, and glue
# ---------------------------------------------------------------------------

# A DNS-only domain. This never contacts a registry, but it is still invoiced:
# EasyDNS charges for the DNS service itself, so the account needs a balance.
# deletion_protection defaults to true; destroy fails until it is set to false
# and applied.
resource "easydns_domain" "dns_only" {
  domain   = "example.invalid"
  service  = "dns"
  term     = 1
  currency = "USD"
  dns_only = true
}

# Registering a domain additionally requires enable_domain_registration = true
# on the provider block above, plus contacts. See the registrar safety guide.
#
# resource "easydns_domain" "registered" {
#   domain   = "example.invalid"
#   service  = "pro"
#   term     = 2
#   currency = "USD"
#   dns_only = false
#
#   contacts = {
#     owner = {
#       first_name  = "Jordan"
#       last_name   = "Lee"
#       address1    = "123 Example St"
#       city        = "Toronto"
#       state       = "ON"
#       country     = "CA"
#       postal_code = "A1A 1A1"
#       phone       = "+1.4165550100"
#       email       = "registrant@example.invalid"
#       language    = "en"
#     }
#   }
#
#   # A premium domain also needs premium = true, the verified premium_price,
#   # and a max_premium_price ceiling this configuration accepts.
# }

# Registrar lock and renewal policy. Destroying this leaves the remote policy
# untouched.
resource "easydns_domain_registration_settings" "example" {
  domain  = easydns_domain.dns_only.domain
  reglock = true
  renewal = "renew"
}

# The complete delegation set. Order is not significant.
resource "easydns_domain_nameservers" "example" {
  domain = easydns_domain.dns_only.domain

  nameservers = [
    "dns1.easydns.com",
    "dns2.easydns.net",
    "dns3.easydns.org",
  ]
}

# Glue for a nameserver inside the domain it serves.
resource "easydns_glue_record" "ns1" {
  domain = easydns_domain.dns_only.domain
  host   = "ns1.example.invalid"
  ipv4   = "192.0.2.53"
  ipv6   = "2001:db8::53"
}

data "easydns_domain" "example" {
  domain = easydns_domain.dns_only.domain
}

data "easydns_domains" "mine" {}

data "easydns_domain_registration_statuses" "all" {}

data "easydns_domain_nameservers" "example" {
  domain = easydns_domain.dns_only.domain
}

data "easydns_glue_records" "example" {
  domain = easydns_domain.dns_only.domain
}

output "domain_next_due" {
  value = data.easydns_domain.example.next_due
}

output "delegated_nameservers" {
  value = data.easydns_domain_nameservers.example.nameservers
}

output "glue_registry_status" {
  value = easydns_glue_record.ns1.registry_configured
}
