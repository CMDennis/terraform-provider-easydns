resource "easydns_domain_registration_settings" "example" {
  domain  = "example.com"
  reglock = true
  renewal = "renew"
}
