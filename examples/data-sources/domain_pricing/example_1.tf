data "easydns_domain_pricing" "example" {
  domain   = "example.com"
  service  = "dns"
  min_term = 1
  max_term = 2
}
