data "easydns_geo_regions" "available" {}

resource "easydns_record" "regional" {
  domain     = "example.com"
  host       = "www"
  type       = "A"
  rdata      = "192.0.2.20"
  ttl        = 600
  geozone_id = 1
  write_mode = "asynchronous"
}
