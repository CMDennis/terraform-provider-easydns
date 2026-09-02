---
page_title: "Release procedure"
subcategory: "Development"
description: |-
  Build, sign, verify, and publish a Terraform Registry-compatible release.
---

# Release procedure

Releases are tag-triggered and use GoReleaser v2. They produce ZIP archives for
the supported operating-system/architecture matrix, a SHA-256 checksum file,
and a detached GPG signature for that checksum file.

## Prerequisites

- All pull-request jobs are green.
- The protected sandbox acceptance suites required for the release have passed.
- The tag matches the reviewed changelog and version policy.
- `GPG_PRIVATE_KEY` and `PASSPHRASE` are configured as release secrets.
- The public signing key is registered for Terraform Registry verification.

## Local release rehearsal

The Make targets download pinned tool releases through Go:

```shell
make ci
make lint vuln
make release-check
```

The snapshot command skips signing but exercises compilation, archive naming,
and checksums. It never publishes.

## Publish

Create an annotated semantic-version tag from the reviewed commit and push the
tag. The release workflow imports the signing key, runs `goreleaser release
--clean`, verifies every archive against the checksum file, and verifies the
detached signature before completing.

Do not rerun a partially published version with different bytes. Fix the
workflow and publish a new version.

## Registry verification

After publication:

1. Confirm the Registry lists every expected platform archive.
2. Confirm the checksum signature is accepted and the protocol manifest is
   present.
3. Inspect the rendered provider, resource, data-source, action, and guide
   pages for broken links or missing schema.
4. Install the exact version in a clean directory and run `terraform providers
   schema -json`.
5. Run the five-minute sandbox example and verify the second plan is empty.

Release notes should name breaking changes, deprecations, migration steps,
security fixes, and any endpoint deliberately excluded from coverage.
