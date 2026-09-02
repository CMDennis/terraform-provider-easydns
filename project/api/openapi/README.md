# EasyDNS OpenAPI snapshot

| Field | Value |
|---|---|
| Upstream | <https://docs.sandbox.rest.easydns.net:3001/swagger3.yaml> |
| Retrieved | 2026-09-01 |
| OpenAPI | 3.0.1 |
| API document version | 1.1.1 |
| Local file | `easydns-v1.1.1.yaml` |
| SHA-256 | `a94056396de269cce233677ed7f5fcfc98400738b7452ae0e1cb96c3589672b9` |

This is a byte-for-byte public contract snapshot. It contains no credentials,
account data, real domains, or responses from an authenticated API call.

The upstream document has incomplete response bodies and loosely specified
objects. Implementation must combine this schema with sanitized observations
from a dedicated EasyDNS sandbox and must retain regression fixtures for any
observed discrepancy.

To update the snapshot:

1. Download the new upstream file without authentication.
2. Record its advertised version, retrieval date, and SHA-256 here.
3. Compare operation IDs and schemas against the previous snapshot.
4. Update `api/coverage.csv` for every added, removed, or changed operation.
5. Run `./scripts/validate-phase-0.sh`.
6. Review the change as an API-contract update, not as a formatting change.
