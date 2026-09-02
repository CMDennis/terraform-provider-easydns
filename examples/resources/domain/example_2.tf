provider "easydns" {
  enable_domain_registration = true
}

resource "easydns_domain" "registered" {
  domain   = "example.com"
  service  = "pro"
  term     = 2
  currency = "USD"
  dns_only = false

  contacts = {
    owner = {
      first_name  = "Jordan"
      last_name   = "Lee"
      address1    = "123 Example St"
      city        = "Toronto"
      state       = "ON"
      country     = "CA"
      postal_code = "A1A 1A1"
      phone       = "+1.4165550100"
      email       = "registrant@example.com"
      language    = "en" # required for .CA
    }
  }
}
