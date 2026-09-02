# Pin the toolchain to the one go.mod selects, so a local run cannot silently
# download a newer Go than CI has and pass a gate that CI then fails. CI gets
# this pin from actions/setup-go reading go.mod; this is the local equivalent.
GOTOOLCHAIN := go1.26.8
export GOTOOLCHAIN

TFPLUGINDOCS_VERSION ?= v0.25.0
GOLANGCI_LINT_VERSION ?= v2.8.0
GOVULNCHECK_VERSION ?= v1.7.0
GORELEASER_VERSION ?= v2.17.0
ACTIONLINT_VERSION ?= v1.7.7

.PHONY: acceptance-compile actions-check build check ci coverage docs docs-check docs-validate examples-check fmt fmt-check integration-compile lint release-check test test-race tidy-check vet vuln

build:
	go build ./...

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

test:
	go test ./...

test-race:
	go test -race ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@$(TFPLUGINDOCS_VERSION) generate --provider-name easydns

docs-check:
	TFPLUGINDOCS_VERSION=$(TFPLUGINDOCS_VERSION) ./scripts/check-generated-docs.sh

docs-validate:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@$(TFPLUGINDOCS_VERSION) validate --provider-name easydns

examples-check:
	terraform fmt -check -recursive examples

vet:
	go vet ./...

tidy-check:
	go mod tidy -diff

acceptance-compile:
	go test -tags=acceptance -run '^$$' ./internal/provider

actions-check:
	go run github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION) .github/workflows/test.yml .github/workflows/release.yml .github/workflows/sandbox-acceptance.yml

integration-compile:
	go test -tags=integration -run '^$$' ./internal/provider

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

release-check:
	go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION) check
	go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION) release --snapshot --clean --skip=sign
	@checksum_file="$$(find dist -maxdepth 1 -name '*_SHA256SUMS' -print -quit)"; \
	  test -n "$$checksum_file"; \
	  checksum_name="$$(basename "$$checksum_file")"; \
	  if command -v sha256sum >/dev/null 2>&1; then \
	    cd dist && sha256sum --check "$$checksum_name"; \
	  else \
	    cd dist && shasum -a 256 -c "$$checksum_name"; \
	  fi

check: tidy-check fmt-check vet test-race build acceptance-compile integration-compile examples-check

# ci mirrors every gate the pull-request workflow runs, so a local green
# result means the same thing as a green build.
ci: check docs-check docs-validate actions-check lint vuln release-check
