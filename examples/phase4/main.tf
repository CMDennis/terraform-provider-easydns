terraform {
  required_version = ">= 1.14.0"

  required_providers {
    easydns = {
      source = "CMDennis/easydns"
    }
  }
}

provider "easydns" {
  environment = "sandbox"
}

data "easydns_mailmaps" "example" {
  domain = "example.invalid"
}

data "easydns_current_user" "current" {}

data "easydns_domain_pricing" "example" {
  domain  = "example.invalid"
  service = "dns"
}

resource "easydns_mailmap" "support" {
  domain       = "example.invalid"
  alias        = "support"
  host         = "@"
  destinations = ["recipient@example.invalid"]
}

action "easydns_force_zone_reload" "example" {
  config {
    domain = "example.invalid"
  }
}

action "easydns_set_primary_nameserver" "example" {
  config {
    domain = "example.invalid"
    master = "192.0.2.53"
  }
}
