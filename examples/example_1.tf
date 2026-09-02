terraform {
  required_providers {
    easydns = {
      source = "CMDennis/easydns"
    }
  }
}

provider "easydns" {
  environment       = "sandbox"
  record_write_mode = "synchronous"
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
