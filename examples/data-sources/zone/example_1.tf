data "easydns_zone" "main" {
  domain = "example.com"
}

output "zone_service" {
  value = data.easydns_zone.main.service
}

output "zone_exists" {
  value = data.easydns_zone.main.exists
}
