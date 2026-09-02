# Import an existing zone
resource "easydns_zone" "main" {
  domain = "example.com"
}

# Use the zone with records
resource "easydns_record" "www" {
  domain = easydns_zone.main.domain
  host   = "www"
  type   = "A"
  rdata  = "192.0.2.1"
}
