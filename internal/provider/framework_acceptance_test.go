//go:build acceptance

package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	accresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"easydns": providerserver.NewProtocol6WithError(New("acceptance")()),
}

// TestMain isolates the suite from the developer's Terraform CLI configuration.
//
// A dev_overrides block in ~/.terraformrc silently replaces a provider for
// every Terraform run, which breaks this suite two ways: it shadows the
// in-process provider under test, and it prevents the migration test from
// downloading the published v0.1.1 it needs. Unless the caller has chosen a
// CLI config explicitly, point Terraform at a neutral one.
func TestMain(m *testing.M) {
	if os.Getenv("TF_CLI_CONFIG_FILE") == "" {
		directory, err := os.MkdirTemp("", "easydns-acc-cli")
		if err != nil {
			fmt.Fprintf(os.Stderr, "create Terraform CLI config: %v\n", err)
			os.Exit(1)
		}
		defer os.RemoveAll(directory)

		path := filepath.Join(directory, "terraform.rc")
		if err := os.WriteFile(path, []byte("provider_installation {\n  direct {}\n}\n"), 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "write Terraform CLI config: %v\n", err)
			os.Exit(1)
		}
		os.Setenv("TF_CLI_CONFIG_FILE", path)
	}
	os.Exit(m.Run())
}

func TestAccReadOnlyDomain(t *testing.T) {
	domain := testAccDomain(t, "EASYDNS_TEST_DOMAIN")
	testAccPreCheck(t)

	accresource.Test(t, accresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []accresource.TestStep{{
			Config: testAccProviderConfig() + fmt.Sprintf(`
data "easydns_domain" "fixture" {
  domain = %s
}
`, strconv.Quote(domain)),
			Check: accresource.ComposeAggregateTestCheckFunc(
				accresource.TestCheckResourceAttr("data.easydns_domain.fixture", "domain", domain),
				accresource.TestCheckResourceAttrSet("data.easydns_domain.fixture", "id"),
			),
		}},
	})
}

func TestAccZoneResourceAdoption(t *testing.T) {
	domain := testAccDomain(t, "EASYDNS_TEST_DOMAIN")
	testAccPreCheck(t)

	const resourceName = "easydns_zone.fixture"
	config := testAccProviderConfig() + fmt.Sprintf(`
resource "easydns_zone" "fixture" {
  domain = %s
}
`, strconv.Quote(domain))

	accresource.Test(t, accresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []accresource.TestStep{
			{
				Config: config,
				Check: accresource.ComposeAggregateTestCheckFunc(
					accresource.TestCheckResourceAttr(resourceName, "domain", domain),
					accresource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{Config: config, PlanOnly: true},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateId:     domain,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccRecordResource(t *testing.T) {
	testAccRequireGate(t, "EASYDNS_ACC_ALLOW_MUTATIONS", "sandbox-writes-only")
	domain := testAccDomain(t, "EASYDNS_TEST_DOMAIN")

	for _, mode := range []RecordWriteMode{RecordWriteModeSynchronous, RecordWriteModeAsynchronous} {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			testAccPreCheck(t)
			host := "tfacc-" + strconv.FormatInt(time.Now().UnixNano(), 36)
			const resourceName = "easydns_record.fixture"
			initial := testAccRecordConfig(domain, host, "192.0.2.123", 300, mode)
			updated := testAccRecordConfig(domain, host, "192.0.2.124", 600, mode)

			accresource.Test(t, accresource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []accresource.TestStep{
					{
						Config: initial,
						Check: accresource.ComposeAggregateTestCheckFunc(
							accresource.TestCheckResourceAttr(resourceName, "host", host),
							accresource.TestCheckResourceAttr(resourceName, "rdata", "192.0.2.123"),
							accresource.TestCheckResourceAttr(resourceName, "write_mode", string(mode)),
							accresource.TestCheckResourceAttrSet(resourceName, "id"),
						),
					},
					{Config: initial, PlanOnly: true},
					{
						Config: updated,
						Check: accresource.ComposeAggregateTestCheckFunc(
							accresource.TestCheckResourceAttr(resourceName, "rdata", "192.0.2.124"),
							accresource.TestCheckResourceAttr(resourceName, "ttl", "600"),
						),
					},
					{Config: updated, PlanOnly: true},
					{
						ResourceName:        resourceName,
						ImportState:         true,
						ImportStateIdPrefix: domain + ":",
						ImportStateVerify:   true,
						ImportStateVerifyIgnore: []string{
							"write_mode",
						},
					},
				},
			})
		})
	}
}

func TestAccMailmapResource(t *testing.T) {
	testAccRequireGate(t, "EASYDNS_ACC_ALLOW_MUTATIONS", "sandbox-writes-only")
	domain := testAccDomain(t, "EASYDNS_TEST_DOMAIN")
	destination := strings.TrimSpace(os.Getenv("EASYDNS_TEST_MAILMAP_DESTINATION"))
	if destination == "" {
		t.Skip("EASYDNS_TEST_MAILMAP_DESTINATION must name a disposable destination")
	}
	testAccPreCheck(t)

	alias := "tfacc-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	const resourceName = "easydns_mailmap.fixture"
	initial := testAccMailmapConfig(domain, alias, destination, true)
	updated := testAccMailmapConfig(domain, alias, destination, false)

	accresource.Test(t, accresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []accresource.TestStep{
			{
				Config: initial,
				Check: accresource.ComposeAggregateTestCheckFunc(
					accresource.TestCheckResourceAttr(resourceName, "alias", alias),
					accresource.TestCheckResourceAttr(resourceName, "active", "true"),
					accresource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{Config: initial, PlanOnly: true},
			{
				Config: updated,
				Check:  accresource.TestCheckResourceAttr(resourceName, "active", "false"),
			},
			{Config: updated, PlanOnly: true},
			{
				ResourceName:        resourceName,
				ImportState:         true,
				ImportStateIdPrefix: domain + ":",
				ImportStateVerify:   true,
				ImportStateVerifyIgnore: []string{
					"last_modified",
				},
			},
		},
	})
}

func TestAccDomainNameserversResource(t *testing.T) {
	testAccRequireGate(t, "EASYDNS_ACC_ALLOW_DELEGATION", "sandbox-delegation-writes-only")
	domain := testAccDomain(t, "EASYDNS_TEST_DELEGATION_DOMAIN")
	initial := testAccNameservers(t, "EASYDNS_TEST_NAMESERVERS_INITIAL")
	updated := testAccNameservers(t, "EASYDNS_TEST_NAMESERVERS_UPDATED")
	testAccPreCheck(t)

	const resourceName = "easydns_domain_nameservers.fixture"
	initialConfig := testAccNameserverConfig(domain, initial)
	updatedConfig := testAccNameserverConfig(domain, updated)
	accresource.Test(t, accresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []accresource.TestStep{
			{Config: initialConfig, Check: accresource.TestCheckResourceAttr(resourceName, "domain", domain)},
			{Config: initialConfig, PlanOnly: true},
			{Config: updatedConfig},
			{Config: updatedConfig, PlanOnly: true},
			{ResourceName: resourceName, ImportState: true, ImportStateId: domain, ImportStateVerify: true},
			// Restore the dedicated fixture before Terraform forgets it on destroy.
			{Config: initialConfig},
			{Config: initialConfig, PlanOnly: true},
		},
	})
}

func TestAccDomainRegistrationSettingsResource(t *testing.T) {
	testAccRequireGate(t, "EASYDNS_ACC_ALLOW_REGISTRAR_MUTATIONS", "sandbox-registrar-writes-only")
	domain := testAccDomain(t, "EASYDNS_TEST_REGISTRAR_DOMAIN")
	testAccPreCheck(t)

	const resourceName = "easydns_domain_registration_settings.fixture"
	config := testAccProviderConfig() + fmt.Sprintf(`
resource "easydns_domain_registration_settings" "fixture" {
  domain  = %s
  reglock = false
  renewal = "remind"
}
`, strconv.Quote(domain))

	accresource.Test(t, accresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []accresource.TestStep{
			{Config: config, Check: accresource.TestCheckResourceAttr(resourceName, "renewal", "remind")},
			{Config: config, PlanOnly: true},
			{ResourceName: resourceName, ImportState: true, ImportStateId: domain, ImportStateVerify: true},
		},
	})
}

func TestAccGlueRecordResource(t *testing.T) {
	testAccRequireGate(t, "EASYDNS_ACC_ALLOW_GLUE", "sandbox-glue-writes-only")
	domain := testAccDomain(t, "EASYDNS_TEST_GLUE_DOMAIN")
	host := testAccDomain(t, "EASYDNS_TEST_GLUE_HOST")
	testAccPreCheck(t)

	const resourceName = "easydns_glue_record.fixture"
	config := testAccProviderConfig() + fmt.Sprintf(`
resource "easydns_glue_record" "fixture" {
  domain = %s
  host   = %s
  ipv4   = "192.0.2.53"
}
`, strconv.Quote(domain), strconv.Quote(host))

	accresource.Test(t, accresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []accresource.TestStep{
			{Config: config, Check: accresource.TestCheckResourceAttr(resourceName, "ipv4", "192.0.2.53")},
			{Config: config, PlanOnly: true},
			{ResourceName: resourceName, ImportState: true, ImportStateId: domain + ":" + host, ImportStateVerify: true},
		},
	})
}

func TestAccDomainResourceLifecycle(t *testing.T) {
	testAccRequireGate(t, "EASYDNS_ACC_ALLOW_DOMAIN_LIFECYCLE", "delete-disposable-sandbox-domain")
	domain := testAccDomain(t, "EASYDNS_TEST_NEW_DOMAIN")
	testAccPreCheck(t)

	// Sandbox invoices are notional, but the domain this creates cannot be
	// deleted through the API afterwards, so every run leaves one behind.
	// dns is the cheapest level that accepts DNS-only; lite is cheaper but
	// EasyDNS refuses dns_only on it.
	service := strings.TrimSpace(os.Getenv("EASYDNS_TEST_NEW_DOMAIN_SERVICE"))
	if service == "" {
		service = "dns"
	}

	const resourceName = "easydns_domain.fixture"
	config := testAccProviderConfig() + fmt.Sprintf(`
resource "easydns_domain" "fixture" {
  domain              = %s
  service             = %s
  term                = 1
  currency            = "CAD"
  dns_only            = true
  deletion_protection = false
}
`, strconv.Quote(domain), strconv.Quote(service))

	accresource.Test(t, accresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []accresource.TestStep{
			{Config: config, Check: accresource.TestCheckResourceAttr(resourceName, "domain", domain)},
			{Config: config, PlanOnly: true},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateId:     domain,
				ImportStateVerify: true,
				// These are creation-time billing and policy inputs that the
				// API does not report back, so an import cannot recover them.
				ImportStateVerifyIgnore: []string{
					"currency", "deletion_protection", "dns_only", "premium", "service", "term",
				},
			},
		},
	})
}

func TestAccV011RecordMigration(t *testing.T) {
	testAccRequireGate(t, "EASYDNS_ACC_ALLOW_MIGRATION", "v0.1.1-sandbox-record")
	domain := testAccDomain(t, "EASYDNS_TEST_DOMAIN")
	testAccPreCheck(t)

	host := "tfacc-migrate-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	const resourceName = "easydns_record.fixture"
	config := testAccLegacyRecordConfig(domain, host)
	oldProvider := map[string]accresource.ExternalProvider{
		"easydns": {Source: "CMDennis/easydns", VersionConstraint: "= 0.1.1"},
	}

	accresource.Test(t, accresource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []accresource.TestStep{
			{
				Config:            config,
				ExternalProviders: oldProvider,
				Check: accresource.ComposeAggregateTestCheckFunc(
					accresource.TestCheckResourceAttr(resourceName, "host", host),
					accresource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config:                   config,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				PlanOnly:                 true,
			},
			{
				Config:                   config,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			},
			{
				ResourceName:             resourceName,
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				ImportState:              true,
				ImportStateIdPrefix:      domain + ":",
				ImportStateVerify:        true,
				ImportStateVerifyIgnore: []string{
					"write_mode",
				},
			},
		},
	})
}

func testAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("EASYDNS_ACC_SANDBOX") != "1" {
		t.Fatal("EASYDNS_ACC_SANDBOX=1 is required")
	}
	if os.Getenv("EASYDNS_API_TOKEN") == "" || os.Getenv("EASYDNS_API_KEY") == "" {
		t.Fatal("sandbox EASYDNS_API_TOKEN and EASYDNS_API_KEY are required")
	}
	if configuredURL := os.Getenv("EASYDNS_API_URL"); configuredURL != "" {
		if _, err := validateAcceptanceBaseURL(configuredURL); err != nil {
			t.Fatalf("unsafe acceptance-test configuration: %v", err)
		}
	}
}

func testAccRequireGate(t *testing.T, name, expected string) {
	t.Helper()
	if os.Getenv(name) != expected {
		t.Skipf("%s=%s is required for this sandbox mutation test", name, expected)
	}
}

func testAccDomain(t *testing.T, name string) string {
	t.Helper()
	domain := strings.TrimSpace(os.Getenv(name))
	if domain == "" {
		t.Skipf("%s must name a dedicated sandbox fixture", name)
	}
	normalized, err := NormalizeDomain(domain)
	if err != nil || normalized != domain {
		t.Fatalf("%s must be a normalized domain name: %v", name, err)
	}
	return domain
}

func testAccNameservers(t *testing.T, name string) []string {
	t.Helper()
	values := strings.Split(os.Getenv(name), ",")
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	if len(result) < 2 {
		t.Skipf("%s must contain at least two comma-separated sandbox nameservers", name)
	}
	return result
}

// testAccPollInterval lets a run trade settling speed against the daily
// request budget without editing the suite.
func testAccPollInterval() string {
	if value := strings.TrimSpace(os.Getenv("EASYDNS_TEST_POLL_INTERVAL")); value != "" {
		return value
	}
	return "5s"
}

func testAccProviderConfig() string {
	// No required_providers block: the suite serves the provider in-process
	// through ProtoV6ProviderFactories, which registers it under the implied
	// namespace. Naming a registry source here makes Terraform try to install
	// a published provider instead of using the one under test.
	return fmt.Sprintf(`
provider "easydns" {
  api_url = %s

  # The sandbox allows only 500 requests per day. Polling every five seconds
  # instead of two cuts the reads a slow reconcile can spend from 60 to 24.
  record_poll_interval = %s
}
`, strconv.Quote(sandboxURL), strconv.Quote(testAccPollInterval()))
}

func testAccRecordConfig(domain, host, address string, ttl int, mode RecordWriteMode) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "easydns_record" "fixture" {
  domain     = %s
  host       = %s
  type       = "A"
  rdata      = %s
  ttl        = %d
  write_mode = %s
}
`, strconv.Quote(domain), strconv.Quote(host), strconv.Quote(address), ttl, strconv.Quote(string(mode)))
}

func testAccMailmapConfig(domain, alias, destination string, active bool) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "easydns_mailmap" "fixture" {
  domain       = %s
  alias        = %s
  host         = "@"
  destinations = [%s]
  active       = %t
}
`, strconv.Quote(domain), strconv.Quote(alias), strconv.Quote(destination), active)
}

func testAccNameserverConfig(domain string, nameservers []string) string {
	quoted := make([]string, 0, len(nameservers))
	for _, nameserver := range nameservers {
		quoted = append(quoted, strconv.Quote(nameserver))
	}
	return testAccProviderConfig() + fmt.Sprintf(`
resource "easydns_domain_nameservers" "fixture" {
  domain      = %s
  nameservers = [%s]
}
`, strconv.Quote(domain), strings.Join(quoted, ", "))
}

func testAccLegacyRecordConfig(domain, host string) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    easydns = {
      source = "CMDennis/easydns"
    }
  }
}

provider "easydns" {
  api_url       = %s
  use_async_api = false
}

resource "easydns_record" "fixture" {
  domain = %s
  host   = %s
  type   = "A"
  rdata  = "192.0.2.125"
  ttl    = 300
}
`, strconv.Quote(sandboxURL), strconv.Quote(domain), strconv.Quote(host))
}
