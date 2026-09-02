data "easydns_records" "all" {
  domain = "example.com"
}

output "total_records" {
  value = length(data.easydns_records.all.records)
}

output "all_records" {
  value = [for r in data.easydns_records.all.records : "${r.type} ${r.host} -> ${r.rdata}"]
}
