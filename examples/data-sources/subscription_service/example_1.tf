data "easydns_subscription_service" "current" {
  subscription_id = data.easydns_domain.example.subscription_id
}
