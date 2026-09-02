data "easydns_domains" "mine" {}

output "domain_names" {
  value = [for entry in data.easydns_domains.mine.domains : entry.domain]
}
