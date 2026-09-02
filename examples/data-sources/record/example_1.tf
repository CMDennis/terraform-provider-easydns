data "easydns_record" "www" {
  domain = "example.com"
  host   = "www"
  type   = "A"
}

output "www_ip" {
  value = data.easydns_record.www.rdata
}

output "www_ttl" {
  value = data.easydns_record.www.ttl
}
