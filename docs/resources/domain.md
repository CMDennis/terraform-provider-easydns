---
page_title: "easydns_domain Resource - EasyDNS"
description: |-
  Adds a domain to EasyDNS for DNS-only service or, behind an explicit opt-in, registers it at the registry.
---


# easydns_domain (Resource)

Adds a domain to the EasyDNS system. The resource has two modes:

- **DNS-only** (`dns_only = true`, the default) adds the domain for DNS service without touching a registry.
- **Registration** (`dns_only = false`) registers the domain at the registry. This is billable, cannot be undone by Terraform, and requires an explicit provider opt-in.

### Both modes cost money

`dns_only = true` avoids the *registry* fee, not every fee. EasyDNS invoices the DNS service itself, and `term` and `currency` are the billing inputs for that invoice. Creating a domain with an insufficient account balance fails with `Request cost (NN.NN) exceeds user balance amount (0.00)`.

`dns_only = true` is also refused on the `lite` level, so the cheapest DNS-only option is `dns`. The provider rejects that combination during planning.

Use the `easydns_domain_pricing` data source to see what a given domain and service level costs before applying. Sandbox observation for a `.com` at a one-year term: `lite` CAD 17.25, `dns` CAD 36.80, `pro` CAD 56.35, plus tax. There is no free service level.

## DNS-only domain

```terraform
resource "easydns_domain" "example" {
  domain   = "example.com"
  service  = "dns"
  term     = 1
  currency = "USD"
  dns_only = true
}
```

## Registering a domain

Registration requires `enable_domain_registration = true` on the provider. Without it the plan fails before anything is billed.

```terraform
provider "easydns" {
  enable_domain_registration = true
}

resource "easydns_domain" "registered" {
  domain   = "example.com"
  service  = "pro"
  term     = 2
  currency = "USD"
  dns_only = false

  contacts = {
    owner = {
      first_name  = "Jordan"
      last_name   = "Lee"
      address1    = "123 Example St"
      city        = "Toronto"
      state       = "ON"
      country     = "CA"
      postal_code = "A1A 1A1"
      phone       = "+1.4165550100"
      email       = "registrant@example.com"
      language    = "en" # required for .CA
    }
  }
}
```

`admin`, `tech`, and `billing` contacts follow the same shape. Only `owner` accepts `language` and `cpr`, which .CA registrations require.

### Premium domains

A registry-priced premium domain needs three things: the `premium` acknowledgement, the `premium_price` returned by the EasyDNS pricing endpoint, and a `max_premium_price` ceiling this configuration accepts. A `premium_price` above the ceiling fails during planning rather than at the registry.

```terraform
resource "easydns_domain" "premium" {
  domain            = "example.com"
  service           = "pro"
  term              = 1
  currency          = "USD"
  dns_only          = false
  premium           = true
  premium_price     = "45.97"
  max_premium_price = "50.00"

  contacts = { owner = { /* ... */ } }
}
```

Prices are compared as exact decimals, never as floating-point numbers.

### TLD-specific registration fields

Some TLDs require extra registrant data. Pass it through `extra`:

```terraform
resource "easydns_domain" "french" {
  # ...
  extra = {
    registrant_type = "individual"
    date_of_birth   = "1980-01-01"
  }
}
```

Documented keys include `br_register_number` (.BR), `registrant_type` and `intended_use` (.CAT), `registrant_type` and birth details (.FR), `entity_type`, `nationality_code` and `reg_code` (.IT), the `qli_*` fields (.LAW), `registrant_type`, `id_card_number` and `registration_number` (.NU, .SE, .SX), `paris_nexus` (.PARIS), `registration_number` (.SG), and `app_purpose`, `category` and `validator` (.US).

## Deleting a domain

`deletion_protection` defaults to `true`. While it is on, `terraform destroy` fails with a diagnostic instead of deleting the domain. Terraform never silently drops the domain from state instead of deleting it.

To delete a domain, set `deletion_protection = false`, apply that change, then destroy.

## Immutable attributes

`domain`, `service`, `term`, `currency`, and `dns_only` cannot be changed after creation. EasyDNS has no update operation for them, and Terraform will not delete and re-add a real domain to apply a change. Changing one produces a planning error naming the attribute. To move to a different configuration deliberately, remove the resource with `terraform state rm` and manage the new configuration explicitly.

## Deleting can be accepted without taking effect

~> **Deletion is unverified.** Against the EasyDNS sandbox it is accepted and then does not happen. Create, read and import are verified; see the [verification status guide](../guides/verification-status).

EasyDNS has answered a domain delete with HTTP 200 and then continued to report the domain as on the system indefinitely, with its next-due date unchanged. When that happens the provider reports a diagnostic and **leaves the resource in state**, rather than treating the accepted delete as done. Removing it from state would abandon a domain that may still bill.

If you see that diagnostic, check the domain in the EasyDNS dashboard; deletion may be queued or may require cancelling the service there.

## Import

```shell
terraform import easydns_domain.example example.com
```

An imported domain always starts with `deletion_protection = true`.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `currency` (String) Billing currency for the creation invoice: 'CAD' or 'USD'.
- `domain` (String) The domain name, for example 'example.com'.
- `service` (String) EasyDNS service level: 'lite', 'dns', 'pro', or 'enterprise'.
- `term` (Number) Service term in years, 1 through 10. For a registration this is also the registration term.

### Optional

- `contacts` (Attributes, Sensitive) Registrant contacts. Required to register a domain and rejected for a DNS-only domain. These fields are personal data and are stored in Terraform state. (see [below for nested schema](#nestedatt--contacts))
- `deletion_protection` (Boolean) Refuses `terraform destroy` for this domain while true. Defaults to true. Set it to false and apply before destroying.
- `dns_only` (Boolean) When true the domain is added for DNS service only and is never registered at a registry. Defaults to true. This is not the same as free: EasyDNS invoices the DNS service itself, so creating any domain draws on the account balance.
- `domain_group` (String) An existing domain group to assign the new domain to.
- `extra` (Map of String, Sensitive) Documented TLD-specific registration fields, such as registrant_type for .FR or app_purpose for .US. Values are passed through unchanged and may contain personal data.
- `max_premium_price` (String) The highest premium price this configuration accepts. A premium_price above it fails during planning rather than at the registry.
- `nameservers` (Set of String) Optional delegation to apply at creation, up to six hosts. Ongoing delegation belongs to easydns_domain_nameservers.
- `premium` (Boolean) Acknowledges that the registry prices this domain as premium. Requires max_premium_price.
- `premium_price` (String) The verified premium price sent to EasyDNS, taken from the pricing endpoint. Must not exceed max_premium_price.
- `primary_ns` (String) Primary nameserver addresses for a secondary domain, separated by semicolons. Setting this makes the domain secondary and starts a zone transfer.

### Read-Only

- `cloned_to` (String) Domain this one is cloned to, when cloning is enabled.
- `exists` (Boolean) Whether the domain exists at the registry.
- `expiry` (String) Registration expiry date, empty for a domain without registration.
- `id` (String) The domain identity, equal to the normalized domain name.
- `next_due` (String) Date the EasyDNS service is next due.
- `on_system` (Boolean) Whether the domain exists on the EasyDNS system.
- `service_id` (Number) Numeric EasyDNS service ID in use.
- `subscription_id` (Number) Subscription block ID when the domain belongs to one.

<a id="nestedatt--contacts"></a>
### Nested Schema for `contacts`

Optional:

- `admin` (Attributes) Administrative contact. (see [below for nested schema](#nestedatt--contacts--admin))
- `billing` (Attributes) Billing contact. (see [below for nested schema](#nestedatt--contacts--billing))
- `owner` (Attributes) Registrant contact. Required for a registration. (see [below for nested schema](#nestedatt--contacts--owner))
- `tech` (Attributes) Technical contact. (see [below for nested schema](#nestedatt--contacts--tech))

<a id="nestedatt--contacts--admin"></a>
### Nested Schema for `contacts.admin`

Required:

- `address1` (String) Street address.
- `city` (String) City.
- `country` (String) Two-letter ISO 3166-1 country code.
- `email` (String) Email address.
- `first_name` (String) Given name.
- `last_name` (String) Family name.
- `phone` (String) Phone number in E.164 form, for example '+1.4165550100'.
- `postal_code` (String) Postal or ZIP code.
- `state` (String) Province or state.

Optional:

- `address2` (String) Second address line.
- `cpr` (String) Canadian Presence Requirement type. .CA registrations only.
- `language` (String) Contact language, 'en' or 'fr'. Required for .CA registrations.
- `org_name` (String) Organization.


<a id="nestedatt--contacts--billing"></a>
### Nested Schema for `contacts.billing`

Required:

- `address1` (String) Street address.
- `city` (String) City.
- `country` (String) Two-letter ISO 3166-1 country code.
- `email` (String) Email address.
- `first_name` (String) Given name.
- `last_name` (String) Family name.
- `phone` (String) Phone number in E.164 form, for example '+1.4165550100'.
- `postal_code` (String) Postal or ZIP code.
- `state` (String) Province or state.

Optional:

- `address2` (String) Second address line.
- `cpr` (String) Canadian Presence Requirement type. .CA registrations only.
- `language` (String) Contact language, 'en' or 'fr'. Required for .CA registrations.
- `org_name` (String) Organization.


<a id="nestedatt--contacts--owner"></a>
### Nested Schema for `contacts.owner`

Required:

- `address1` (String) Street address.
- `city` (String) City.
- `country` (String) Two-letter ISO 3166-1 country code.
- `email` (String) Email address.
- `first_name` (String) Given name.
- `last_name` (String) Family name.
- `phone` (String) Phone number in E.164 form, for example '+1.4165550100'.
- `postal_code` (String) Postal or ZIP code.
- `state` (String) Province or state.

Optional:

- `address2` (String) Second address line.
- `cpr` (String) Canadian Presence Requirement type. .CA registrations only.
- `language` (String) Contact language, 'en' or 'fr'. Required for .CA registrations.
- `org_name` (String) Organization.


<a id="nestedatt--contacts--tech"></a>
### Nested Schema for `contacts.tech`

Required:

- `address1` (String) Street address.
- `city` (String) City.
- `country` (String) Two-letter ISO 3166-1 country code.
- `email` (String) Email address.
- `first_name` (String) Given name.
- `last_name` (String) Family name.
- `phone` (String) Phone number in E.164 form, for example '+1.4165550100'.
- `postal_code` (String) Postal or ZIP code.
- `state` (String) Province or state.

Optional:

- `address2` (String) Second address line.
- `cpr` (String) Canadian Presence Requirement type. .CA registrations only.
- `language` (String) Contact language, 'en' or 'fr'. Required for .CA registrations.
- `org_name` (String) Organization.

## Sensitive data

`contacts` and `extra` hold personal data and are marked sensitive, but Terraform still writes them to state in plain text. Use an encrypted, access-controlled state backend for any configuration that registers domains.
