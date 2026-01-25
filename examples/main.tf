terraform {
  required_providers {
    easydns = {
      source = "registry.terraform.io/easydns/easydns"
    }
  }
}

provider "easydns" {
  # Environment: "sandbox" (default) or "production"
  environment = "sandbox"

  # Optional: Use async API for better rate limiting (queues zone reloads)
  # use_async_api = true

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
