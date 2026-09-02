action "easydns_set_primary_nameserver" "example" {
  config {
    domain = "example.com"
    master = "192.0.2.53"
  }
}
