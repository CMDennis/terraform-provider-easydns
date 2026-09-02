#!/bin/sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
openapi_file="$project_root/api/openapi/easydns-v1.1.1.yaml"
coverage_file="$project_root/api/coverage.csv"
expected_sha="a94056396de269cce233677ed7f5fcfc98400738b7452ae0e1cb96c3589672b9"

actual_sha=$(shasum -a 256 "$openapi_file" | awk '{print $1}')
if [ "$actual_sha" != "$expected_sha" ]; then
  echo "OpenAPI checksum mismatch: expected $expected_sha, got $actual_sha" >&2
  exit 1
fi

ruby -ryaml -rcsv -e '
  spec = YAML.load_file(ARGV.fetch(0))
  rows = CSV.read(ARGV.fetch(1), headers: true)
  methods = %w[get put post delete patch]

  operations = spec.fetch("paths").flat_map do |path, path_item|
    path_item.each_with_object([]) do |(method, operation), result|
      next unless methods.include?(method)
      result << [method.upcase, path, operation.fetch("operationId")]
    end
  end

  csv_operations = rows.map do |row|
    [row.fetch("method"), row.fetch("path"), row.fetch("operation_id")]
  end

  duplicates = csv_operations.group_by(&:itself).select { |_key, values| values.length > 1 }.keys
  abort "duplicate coverage rows: #{duplicates.inspect}" unless duplicates.empty?

  missing = operations - csv_operations
  extra = csv_operations - operations
  abort "missing OpenAPI operations: #{missing.inspect}" unless missing.empty?
  abort "coverage rows not present in OpenAPI: #{extra.inspect}" unless extra.empty?

  allowed_statuses = %w[complete partial planned excluded]
  rows.each do |row|
    status = row.fetch("status")
    abort "invalid status #{status.inspect} for #{row.fetch("operation_id")}" unless allowed_statuses.include?(status)
    name = row.fetch("terraform_name").to_s
    if status == "excluded"
      abort "excluded operation unexpectedly has a Terraform name: #{row.fetch("operation_id")}" unless name.empty?
    else
      abort "mapped operation has no Terraform name: #{row.fetch("operation_id")}" if name.empty?
    end
  end

  abort "expected 36 OpenAPI operations, found #{operations.length}" unless operations.length == 36
  puts "OpenAPI coverage: #{operations.length}/#{operations.length} operations mapped"
' "$openapi_file" "$coverage_file"

find "$project_root/testdata/contracts" -type f -name '*.json' -print0 |
  xargs -0 -n 1 ruby -rjson -e 'JSON.parse(File.read(ARGV.fetch(0)))'

ruby "$project_root/scripts/render-api-coverage.rb" --check

for required_file in \
  "$project_root/PROJECT_PLAN.md" \
  "$project_root/docs/phase-0/README.md" \
  "$project_root/docs/state-and-import-conventions.md" \
  "$project_root/docs/testing-strategy.md" \
  "$project_root/docs/adr/0001-target-platform-and-compatibility.md" \
  "$project_root/docs/adr/0002-record-write-modes.md" \
  "$project_root/docs/adr/0003-registrar-safety.md" \
  "$project_root/docs/adr/0004-contracts-identities-and-state.md"
do
  if [ ! -s "$required_file" ]; then
    echo "Required Phase 0 file is missing or empty: $required_file" >&2
    exit 1
  fi
done

echo "Phase 0 validation passed"
