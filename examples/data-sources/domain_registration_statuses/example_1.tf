data "easydns_domain_registration_statuses" "all" {}

output "unlocked_domains" {
  value = [
    for status in data.easydns_domain_registration_statuses.all.statuses :
    status.domain if status.supports_reglock && !status.reglock
  ]
}
