# Contract fixtures

All files in this directory are sanitized and safe for local tests. They use
reserved example domains, documentation address ranges, and non-routable
example email addresses.

## Verification levels

- **Schema-derived**: constructed from the pinned OpenAPI descriptions and
  examples. It is illustrative and may expose an ambiguity in that document.
- **Sandbox-verified**: captured from a dedicated EasyDNS sandbox, sanitized,
  and reviewed. No fixture currently has this status because Phase 0 made no
  authenticated calls.

Current files are all schema-derived:

| Fixture | Contract behavior represented |
|---|---|
| `domain/get-domain-info.success.json` | Domain envelope and service/subscription values |
| `records/list-records.mixed-scalars.json` | Numeric strings, numbers, zero, and null |
| `records/update-record.success.json` | Record mutation response envelope |
| `errors/enhance-your-calm.420.json` | API error embedded in an envelope |
| `nameservers/get-nameservers.success.json` | Nameserver collection response candidate |
| `glue/list-glue.success.json` | IPv4/IPv6 glue list |
| `mailmaps/list-mailmaps.success.json` | Mailmap IDs, destinations, and active state |
| `pricing/check-domain.success.json` | Availability and service pricing |
| `domain/create-domain.success.json` | Domain creation envelope with invoice identity |
| `domain/delete-domain.success.json` | Domain deletion envelope |
| `domains/list-user-domains.numeric-keys.json` | Sanitized copy of the observed list shape: numerically keyed domain entries beside `user`, with paging counters at the top level |
| `domains/list-user-domains.success.json` | User domain index entries |
| `regstatus/get-regstatus.domain-keyed.json` | Registration status keyed by domain name |
| `regstatus/get-regstatus.array.json` | Registration status as self-naming array entries with mixed boolean spellings |
| `regstatus/set-regstatus.request.json` | The domain-keyed request body EasyDNS accepts, contradicting the pinned schema |
| `regstatus/set-regstatus.success.json` | Registration status change result envelope |
| `glue/create-glue.success.json` | Glue mutation response using `nameserver_name` |
| `glue/check-registry-glue.success.json` | Registry glue propagation status |
| `regstatus/get-regstatus.nested-envelope.json` | Sanitized copy of the observed regstatus shape: domain map nested under `data.domains` beside a `user` string |
| `domain/get-domain-info.sentinels.json` | Sanitized copy of an observed domain response using the `NONE` and `false` null sentinels |

## Sandbox observations

One behavior has now been checked against the EasyDNS sandbox with read-only
requests. It is recorded here as evidence, not as a fixture, because no
response body was captured.

- `listUserDomains` returns each domain as a numerically keyed sibling of
  `user` inside `data`, not as members of the documented `index` array, and
  puts `total`, `count`, `start`, and `max` at the top level. Reading only the
  documented form reported an account holding five domains as empty.
- `deleteDomain` answers HTTP 200 with the domain name and then keeps
  reporting `onsystem: "Y"` indefinitely. A DNS-only domain created and
  deleted during acceptance testing was still present many minutes later, with
  its `next_due` date unchanged. The provider reports this rather than
  treating the accepted delete as done, since removing it from state would
  abandon a domain that may still bill.
- `createDomain` refuses `dns_only` on the lite service level with
  "Invalid request combination: DNS only domain creation is not supported for
  the lite service level". The cheapest level that accepts DNS-only is `dns`.
- `getRegStatus` returns the domain-keyed map nested under `data.domains`,
  beside a scalar `user` sibling. None of the three shapes the pinned contract
  suggested matched, and decoding failed outright. The response also omits
  `supports_reglock` entirely and returns `auto_renew_card_id` as `false`
  rather than a string when no card is on file.
- `getDomainNameservers` is refused on the observed sandbox account with
  HTTP 400 "Error encountered attempting to retrieve name servers for domain:
  Authentication Error", so registry-side delegation cannot be read or
  exercised there at all.
- `getZoneSOA` fails on the sandbox for every domain tried, on repeated
  attempts, with HTTP 400 "timeout on read select()". It reads as a backend
  timeout rather than a rejection.
- `updateGlue` is refused with HTTP 400 "Error encountered attempting to
  modify glue record". The refused update did not partially apply.
- `listGeoRegions` returned all 255 regions through the start/max paging path,
  exercising multi-page traversal end to end.
- `forceZoneReload` and `setPrimaryNS` were both invoked and accepted.
- `deleteGlue` is refused with HTTP 400 "Error attempting to delete glue
  record. Please contact support for assistance (no result)" for both the
  fully-qualified host and the bare label, while `createGlue` on the same
  domain succeeds. A created glue record cannot be removed through the API.
- `getDomainNameservers` fails for every domain observed: HTTP 400
  "Authentication Error" for registered domains and an empty body for a
  DNS-only one, so registry delegation cannot be read at all.
- `updateMailmap` is refused with HTTP 406 "Access to resource denied due to
  context restrictions" on the observed sandbox account, for both the bare
  alias and the fully-qualified `alias@domain` path, while `createMailmap` and
  `deleteMailmap` on the same account and domain succeed. The Phase 0 question
  of alias versus address is partly settled by the stored data: `listMailmaps`
  returns `alias` fully qualified. Whether the 406 is an account restriction or
  a general limitation needs a second sandbox account to tell apart.
- `listMailmaps` nests its collection under `data.mailmaps`, and the key is
  absent entirely when a domain has no mailmaps rather than being an empty
  array.
- Terraform record lifecycles were exercised end to end against the sandbox in
  both write modes: create, empty second plan, update, empty second plan,
  import verification, and delete all succeeded.
- `getDomainInfo` uses string sentinels for absent values. An observed
  response carried `sub_block: "NONE"`, `cloned_to: "NONE"`, and
  `expiry: false` for a domain with no subscription block that is not cloned.
  `"NONE"` in an integer field failed decoding outright until the scalar types
  learned the sentinel, which broke `easydns_domain` reads for any such domain.
  `domain/get-domain-info.sentinels.json` is a sanitized copy of that shape.
- `createDomain` is invoiced for every service level, including DNS-only. A
  sandbox account with a zero balance is refused with HTTP 400 "Request cost
  (41.76) exceeds user balance amount (0.00)". Sandbox balances are notional,
  so this exercises the real cost model without a real charge. The provider's guarded
  registration path is therefore not the only one that costs money.
- `getServicePricingForDomain` returns one priced service per level with
  currency, price, and tax components, and reports `available = false` for a
  domain already registered elsewhere. Observed for a `.com` at a one-year
  term: `lite` CAD 17.25, `dns` CAD 36.80, `pro` CAD 56.35, `enterprise`
  CAD 14.83 per month.
- `listUserDomains` takes a real username in its path. `GET /domains/list/self`
  answers HTTP 400 with "Username provided does not match provided
  credentials", so the Phase 0 note that a placeholder "defaults to the
  authenticated account when supported by the observed contract" is settled:
  it is not supported. The client resolves an empty user through `GET /user`
  instead. All verified 2026-09-02 against `sandbox.rest.easydns.net` with read-only
  requests and one refused creation.

## Known contract ambiguities

Two shapes in the pinned OpenAPI document disagree with the operation they
describe. Fixtures cover every plausible reading so a correction upstream does
not break the provider.

- `getRegStatus` is typed as a single-domain object (`ResultModelRegStatus`)
  but the operation returns "a users list of domains". The provider decodes a
  domain-keyed object, an array of self-naming objects, and a bare
  single-domain object.
- `setRegStatus` typed its body as an array of `RequestModelSetRegStatus`
  while its own example showed an object keyed by domain name. **Settled by
  observation:** the array form is refused with HTTP 406 "List of domains to
  modify provided in invalid format". The example is correct and the schema is
  wrong. The provider sends the domain-keyed object.

The `getRegStatus` shape is likewise settled above. Both were resolved against
the sandbox rather than guessed.

## Promotion to sandbox-verified

For each fixture:

1. Use a dedicated sandbox account, never credentials associated with real
   domains.
2. Save the raw response only in a temporary, access-controlled location.
3. Replace domains, IDs, timestamps, names, email addresses, and addresses with
   reserved examples while preserving JSON types and envelope structure.
4. Remove response headers or fields that could identify an account.
5. Have a second reviewer check the sanitized diff.
6. Add a sibling `.metadata.json` recording operation ID, observation date,
   sandbox host, HTTP status, and verification level—never credentials.

Empty-body record-create and delete responses should be represented directly
in HTTP test cases with a zero-length response body rather than a JSON fixture.
