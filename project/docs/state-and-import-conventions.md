# State and Import Conventions

These conventions are fixed for the v1 implementation unless superseded by an
ADR. They preserve existing record import compatibility while giving every new
resource a deterministic remote identity.

## Managed resource identifiers

| Resource | Terraform ID/import form | Example using reserved names |
|---|---|---|
| `easydns_record` | `<domain>:<record_id>` | `example.invalid:123456` |
| `easydns_domain` | `<domain>` | `example.invalid` |
| `easydns_domain_registration_settings` | `<domain>` | `example.invalid` |
| `easydns_domain_nameservers` | `<domain>` | `example.invalid` |
| `easydns_glue_record` | `<domain>:<host>` | `example.invalid:ns1.example.invalid` |
| `easydns_mailmap` | `<domain>:<mailmap_id>` | `example.invalid:1234` |
| `easydns_zone` compatibility resource | `<domain>` | `example.invalid` |

Import parsers must reject empty segments, extra delimiters, surrounding
whitespace, invalid domains, and non-numeric API IDs where the API defines a
numeric ID. Error messages must show the expected form without echoing secrets.

## Canonicalization

- Domains are lower-case ASCII DNS names with one optional trailing dot removed.
  Internationalized domain names are converted to IDNA ASCII before use in an
  API path or identity.
- Record types are upper-case.
- EasyDNS record IDs and mailmap IDs are stored as strings even when the API
  encodes them as JSON numbers.
- Root record hosts use `@`. Host identity is case-insensitive and stored
  lower-case. Wildcards and underscore labels remain valid.
- IPv4 and IPv6 addresses use canonical textual representations.
- TXT, URL, and other opaque record values retain exact content.
- Domain-target record values such as CNAME, MX, NS, PTR, SRV, and ANAME use
  record-type-specific comparison that treats DNS-name case and one trailing
  dot as semantically equivalent. They are not globally rewritten as strings.
- TTL, priority, and geo-region IDs are integers. Null and absent values are
  distinguished during decoding and normalized only at the Terraform schema
  boundary.

## Collection stability

- Nameservers are a Terraform set because delegation order has no meaning.
- Mailmap destinations are a set and are serialized in stable sorted order.
- Record and mailmap collection data sources are sorted by API ID before state
  is written.
- Geo regions are sorted numerically by region ID.
- A server-side records search is part of a data source's identity; changing
  the search input replaces the computed collection naturally on refresh.

## Read and not-found behavior

- A managed resource removes itself from state only after an explicit API
  not-found result or a successful collection lookup proving its ID is absent.
- Authentication failure, authorization failure, throttling, malformed JSON,
  timeout, or server error leaves state intact and returns a diagnostic.
- A data source that cannot find its requested singleton returns an error.
  Collection data sources return an empty collection when the API successfully
  reports no matches.

## Create and update behavior

- After creation, prefer an API-returned ID when present and validate it by
  reading the object.
- If no ID is returned, compare pre-write and post-write ID sets, then validate
  the candidate's semantic fields.
- Updates reconcile remote state before returning after an ambiguous transport
  outcome.
- Changing an immutable domain registration attribute returns a planning error;
  it never schedules automatic domain destruction and re-registration.

## Sensitive state

Credential fields and domain contact/user PII are marked `Sensitive`. Provider
documentation must still warn that Terraform stores sensitive values in state
and that users need an encrypted, access-controlled state backend.
