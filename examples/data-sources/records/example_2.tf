data "easydns_records" "mail" {
  domain         = "example.com"
  search_keyword = "mail"
}
