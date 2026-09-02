data "easydns_geo_regions" "all" {}

output "region_ids" {
  value = { for region in data.easydns_geo_regions.all.regions : region.geo_code => region.id }
}
