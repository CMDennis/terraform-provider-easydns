resource "easydns_domain" "example" {
  domain   = "example.com"
  service  = "dns"
  term     = 1
  currency = "USD"
  dns_only = true
}
