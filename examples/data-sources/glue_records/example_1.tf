data "easydns_glue_records" "example" {
  domain = "example.com"
}

output "glue_hosts" {
  value = [for record in data.easydns_glue_records.example.glue_records : record.host]
}
