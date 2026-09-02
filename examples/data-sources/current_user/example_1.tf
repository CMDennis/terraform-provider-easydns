data "easydns_current_user" "current" {}

output "billing_currency" {
  value = data.easydns_current_user.current.currency
}
