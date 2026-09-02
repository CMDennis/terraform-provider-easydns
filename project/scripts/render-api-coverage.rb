#!/usr/bin/env ruby
# frozen_string_literal: true

require "csv"

root = File.expand_path("..", __dir__)
source = File.join(root, "api", "coverage.csv")
target = File.join(root, "docs", "api-coverage.md")

rows = CSV.read(source, headers: true)
counts = rows.group_by { |row| row.fetch("status") }.transform_values(&:length)

lines = []
lines << "# EasyDNS API Coverage Matrix"
lines << ""
lines << "This file is generated from `api/coverage.csv` by"
lines << "`scripts/render-api-coverage.rb`. Edit the CSV, not this file."
lines << ""
lines << "The pinned OpenAPI v1.1.1 document contains **#{rows.length} operations**:"
lines << "#{counts.fetch("complete", 0)} complete, #{counts.fetch("partial", 0)} partially implemented, #{counts.fetch("planned", 0)} planned, and #{counts.fetch("excluded", 0)} explicitly excluded."
lines << ""
lines << "| Method | Path | Operation | Terraform mapping | Mode | Target | Status | Notes |"
lines << "|---|---|---|---|---|---|---|---|"

rows.each do |row|
  mapping = [row["terraform_kind"], row["terraform_name"]]
            .reject { |value| value.nil? || value.empty? }
            .join(": ")
  values = [
    row["method"],
    "`#{row['path']}`",
    "`#{row['operation_id']}`",
    mapping,
    row["write_mode"],
    row["target_release"],
    row["status"],
    row["notes"]
  ]
  lines << "| #{values.map { |value| value.to_s.gsub('|', '\\|') }.join(' | ')} |"
end

lines << ""
lines << "## Modeling rules"
lines << ""
lines << "- Resources require a refreshable remote identity and meaningful lifecycle semantics."
lines << "- Data sources model reads even when the HTTP method is POST, as with pricing."
lines << "- Actions model imperative operations that cannot be represented as durable state."
lines << "- Exclusions require an explicit lifecycle or security rationale."
lines << "- The synchronous/asynchronous choice applies only to DNS-record create, update, and delete."
lines << ""
lines << "## Maintenance"
lines << ""
lines << "Run `./scripts/validate-phase-0.sh` after updating the pinned OpenAPI document or the coverage CSV."

content = lines.join("\n") + "\n"

if ARGV.include?("--check")
  abort "#{target} is missing; run scripts/render-api-coverage.rb" unless File.exist?(target)
  abort "#{target} is stale; run scripts/render-api-coverage.rb" unless File.read(target) == content
else
  File.write(target, content)
end
