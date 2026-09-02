data "easydns_parsed_records" "current" {
  domain = "example.com"
}

output "expanded_urls" {
  value = [for record in data.easydns_parsed_records.current.records : record.url if record.url != ""]
}
