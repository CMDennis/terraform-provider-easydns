resource "easydns_record" "web" {
  domain = "example.com"
  host   = "www"
  type   = "A"
  rdata  = "192.0.2.1"
  ttl    = 3600
}
