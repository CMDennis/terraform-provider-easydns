resource "easydns_record" "mail" {
  domain = "example.com"
  host   = "@"
  type   = "MX"
  rdata  = "mail.example.com."
  ttl    = 3600
  prio   = 10
}
