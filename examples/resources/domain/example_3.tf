resource "easydns_domain" "premium" {
  domain            = "example.com"
  service           = "pro"
  term              = 1
  currency          = "USD"
  dns_only          = false
  premium           = true
  premium_price     = "45.97"
  max_premium_price = "50.00"

  contacts = { owner = { /* ... */ } }
}
