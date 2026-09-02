resource "easydns_glue_record" "ns1" {
  domain = "example.com"
  host   = "ns1.example.com"
  ipv4   = "192.0.2.53"
  ipv6   = "2001:db8::53"
}
