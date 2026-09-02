resource "easydns_record" "blog" {
  domain = "example.com"
  host   = "blog"
  type   = "CNAME"
  rdata  = "www.example.com."
  ttl    = 3600
}
