data "easydns_records" "all" {
  domain = "example.com"
}

output "a_records" {
  value = [for r in data.easydns_records.all.records : r if r.type == "A"]
}

output "mx_records" {
  value = [for r in data.easydns_records.all.records : r if r.type == "MX"]
}
