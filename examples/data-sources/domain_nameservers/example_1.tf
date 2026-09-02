data "easydns_domain_nameservers" "example" {
  domain = "example.com"
}

output "delegation" {
  value = data.easydns_domain_nameservers.example.nameservers
}
