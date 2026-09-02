resource "easydns_domain" "french" {
  # ...
  extra = {
    registrant_type = "individual"
    date_of_birth   = "1980-01-01"
  }
}
