resource "easydns_record" "spf" {
  domain = "example.com"
  host   = "@"
  type   = "TXT"
  rdata  = "v=spf1 include:_spf.google.com ~all"
  ttl    = 3600
}
