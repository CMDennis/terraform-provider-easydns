data "easydns_domain" "example" {
  domain = "example.com"
}

output "service_due" {
  value = data.easydns_domain.example.next_due
}
