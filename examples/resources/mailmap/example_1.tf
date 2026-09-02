resource "easydns_mailmap" "support" {
  domain = "example.com"
  alias  = "support"
  host   = "@"

  destinations = [
    "alice@example.net",
    "bob@example.net",
  ]
  active = true
}
