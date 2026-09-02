data "easydns_zone_soa" "current" {
  domain = "example.com"
}

output "soa_serial" {
  value = data.easydns_zone_soa.current.serial
}
