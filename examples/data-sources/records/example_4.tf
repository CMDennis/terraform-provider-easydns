data "easydns_records" "all" {
  domain = "example.com"
}

output "record_ids" {
  value = { for r in data.easydns_records.all.records : "${r.host}.${r.type}" => r.id }
}
