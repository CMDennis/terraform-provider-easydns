#!/usr/bin/env bash
set -euo pipefail

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
temporary_root=$(mktemp -d "${TMPDIR:-/tmp}/easydns-docs.XXXXXX")
trap 'rm -rf "$temporary_root"' EXIT

cp -R "$repository_root/." "$temporary_root/provider"

(
  cd "$temporary_root/provider"
  go run "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@${TFPLUGINDOCS_VERSION:-v0.25.0}" \
    generate --provider-name easydns >/dev/null
)

if ! diff -ru "$repository_root/docs" "$temporary_root/provider/docs"; then
  printf '%s\n' 'Generated documentation is stale. Run `make docs` and commit the result.' >&2
  exit 1
fi
