resource "easydns_domain_nameservers" "example" {
  domain = "example.com"

  nameservers = [
    "dns1.easydns.com",
    "dns2.easydns.net",
    "dns3.easydns.org",
  ]
}
