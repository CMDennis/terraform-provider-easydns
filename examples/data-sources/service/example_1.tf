data "easydns_service" "current" {
  service_id = data.easydns_domain.example.service_id
}
