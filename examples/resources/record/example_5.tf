resource "easydns_record" "ipv6" {
  domain = "example.com"
  host   = "www"
  type   = "AAAA"
  rdata  = "2001:db8::1"
  ttl    = 3600
}
