package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestPhase3ProtocolSchemasAreValid(t *testing.T) {
	t.Parallel()

	server := providerserver.NewProtocol6(New("test")())()
	response, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema: %v", err)
	}
	for _, diagnostic := range response.Diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("schema diagnostic: %s: %s", diagnostic.Summary, diagnostic.Detail)
		}
	}

	for _, name := range []string{"easydns_domain", "easydns_domain_registration_settings", "easydns_domain_nameservers", "easydns_glue_record"} {
		if response.ResourceSchemas[name] == nil {
			t.Errorf("protocol schema missing resource %s", name)
		}
	}
	for _, name := range []string{"easydns_domain", "easydns_domains", "easydns_domain_registration_statuses", "easydns_domain_nameservers", "easydns_glue_records"} {
		if response.DataSourceSchemas[name] == nil {
			t.Errorf("protocol schema missing data source %s", name)
		}
	}
	if response.Provider.Block == nil {
		t.Fatal("provider block missing")
	}
	var found bool
	for _, attribute := range response.Provider.Block.Attributes {
		if attribute.Name == "enable_domain_registration" {
			found = true
		}
	}
	if !found {
		t.Error("provider is missing the enable_domain_registration opt-in")
	}
}

// easydns_zone and its data source must announce the replacement before v2.
func TestZoneCompatibilitySurfaceIsDeprecated(t *testing.T) {
	t.Parallel()

	resourceResponse := &resource.SchemaResponse{}
	(&ZoneResource{}).Schema(context.Background(), resource.SchemaRequest{}, resourceResponse)
	if !strings.Contains(resourceResponse.Schema.DeprecationMessage, "easydns_domain") {
		t.Errorf("easydns_zone deprecation message = %q", resourceResponse.Schema.DeprecationMessage)
	}

	dataResponse := &datasource.SchemaResponse{}
	(&ZoneDataSource{}).Schema(context.Background(), datasource.SchemaRequest{}, dataResponse)
	if !strings.Contains(dataResponse.Schema.DeprecationMessage, "easydns_domain") {
		t.Errorf("easydns_zone data source deprecation message = %q", dataResponse.Schema.DeprecationMessage)
	}
}

// ADR-0003: a registration plan is refused unless the provider opted in.
func TestDomainPlanRefusesRegistrationWithoutProviderOptIn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		enabled   bool
		dnsOnly   bool
		wantError bool
	}{
		{name: "registration without opt-in", enabled: false, dnsOnly: false, wantError: true},
		{name: "registration with opt-in", enabled: true, dnsOnly: false, wantError: false},
		{name: "dns-only without opt-in", enabled: false, dnsOnly: true, wantError: false},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			client, err := NewConfiguredClient("https://sandbox.rest.easydns.net", "token", "key", RecordWriteModeSynchronous, test.enabled, 0, 0)
			if err != nil {
				t.Fatalf("create client: %v", err)
			}
			domainResource := &DomainResource{client: client}

			schemaResponse := &resource.SchemaResponse{}
			domainResource.Schema(context.Background(), resource.SchemaRequest{}, schemaResponse)
			plan := domainPlan(t, schemaResponse.Schema, map[string]tftypes.Value{
				"domain":   tftypes.NewValue(tftypes.String, "example.invalid"),
				"service":  tftypes.NewValue(tftypes.String, "dns"),
				"term":     tftypes.NewValue(tftypes.Number, 1),
				"currency": tftypes.NewValue(tftypes.String, "USD"),
				"dns_only": tftypes.NewValue(tftypes.Bool, test.dnsOnly),
			})

			response := &resource.ModifyPlanResponse{Plan: plan}
			domainResource.ModifyPlan(context.Background(), resource.ModifyPlanRequest{
				Plan:  plan,
				State: tfsdk.State{Schema: schemaResponse.Schema, Raw: tftypes.NewValue(plan.Raw.Type(), nil)},
			}, response)

			if response.Diagnostics.HasError() != test.wantError {
				t.Fatalf("diagnostics=%v, wantError=%v", response.Diagnostics, test.wantError)
			}
			if test.wantError && !strings.Contains(response.Diagnostics.Errors()[0].Detail(), "enable_domain_registration") {
				t.Fatalf("diagnostic does not name the opt-in: %v", response.Diagnostics)
			}
		})
	}
}

// ADR-0003: a protected domain refuses destroy instead of dropping state.
func TestDomainDeleteRefusesWhileProtected(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		t.Errorf("a protected domain must not reach the API, got %s %s", request.Method, request.URL.Path)
	}))
	defer server.Close()

	client, err := NewClientWithMode(server.URL, "token", "key", RecordWriteModeSynchronous)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	domainResource := &DomainResource{client: client}

	schemaResponse := &resource.SchemaResponse{}
	domainResource.Schema(context.Background(), resource.SchemaRequest{}, schemaResponse)
	state := tfsdk.State{Schema: schemaResponse.Schema, Raw: domainPlan(t, schemaResponse.Schema, map[string]tftypes.Value{
		"domain":              tftypes.NewValue(tftypes.String, "example.invalid"),
		"service":             tftypes.NewValue(tftypes.String, "dns"),
		"term":                tftypes.NewValue(tftypes.Number, 1),
		"currency":            tftypes.NewValue(tftypes.String, "USD"),
		"dns_only":            tftypes.NewValue(tftypes.Bool, true),
		"deletion_protection": tftypes.NewValue(tftypes.Bool, true),
	}).Raw}

	response := &resource.DeleteResponse{State: state}
	domainResource.Delete(context.Background(), resource.DeleteRequest{State: state}, response)
	if !response.Diagnostics.HasError() {
		t.Fatal("a protected domain was deleted")
	}
	if !strings.Contains(response.Diagnostics.Errors()[0].Detail(), "deletion_protection = false") {
		t.Fatalf("diagnostic does not explain how to proceed: %v", response.Diagnostics)
	}
}

// ADR-0003: immutable fields report a diagnostic rather than planning a
// destroy-and-recreate of a real domain.
func TestDomainPlanRefusesImmutableChangesInsteadOfReplacing(t *testing.T) {
	t.Parallel()

	client, err := NewConfiguredClient("https://sandbox.rest.easydns.net", "token", "key", RecordWriteModeSynchronous, true, 0, 0)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	domainResource := &DomainResource{client: client}

	schemaResponse := &resource.SchemaResponse{}
	domainResource.Schema(context.Background(), resource.SchemaRequest{}, schemaResponse)

	current := map[string]tftypes.Value{
		"domain":   tftypes.NewValue(tftypes.String, "example.invalid"),
		"service":  tftypes.NewValue(tftypes.String, "dns"),
		"term":     tftypes.NewValue(tftypes.Number, 1),
		"currency": tftypes.NewValue(tftypes.String, "USD"),
		"dns_only": tftypes.NewValue(tftypes.Bool, true),
	}
	state := tfsdk.State{Schema: schemaResponse.Schema, Raw: domainPlan(t, schemaResponse.Schema, current).Raw}

	desired := map[string]tftypes.Value{}
	for key, value := range current {
		desired[key] = value
	}
	desired["service"] = tftypes.NewValue(tftypes.String, "pro")
	desired["term"] = tftypes.NewValue(tftypes.Number, 2)
	plan := domainPlan(t, schemaResponse.Schema, desired)

	response := &resource.ModifyPlanResponse{Plan: plan}
	domainResource.ModifyPlan(context.Background(), resource.ModifyPlanRequest{Plan: plan, State: state}, response)
	if !response.Diagnostics.HasError() {
		t.Fatal("an immutable change was planned silently")
	}
	joined := response.Diagnostics.Errors()[0].Detail() + response.Diagnostics.Errors()[1].Detail()
	if !strings.Contains(joined, "service") || !strings.Contains(joined, "term") {
		t.Fatalf("diagnostics did not name both changed fields: %v", response.Diagnostics)
	}
	if len(response.RequiresReplace) > 0 {
		t.Fatal("an immutable change must not schedule a replacement of a real domain")
	}
}

func TestPremiumPriceCeilingComparesExactly(t *testing.T) {
	t.Parallel()

	if err := assertPremiumPriceWithinCeiling("45.97", "45.97"); err != nil {
		t.Fatalf("an equal price was rejected: %v", err)
	}
	if err := assertPremiumPriceWithinCeiling("45.96", "45.97"); err != nil {
		t.Fatalf("a lower price was rejected: %v", err)
	}
	err := assertPremiumPriceWithinCeiling("45.98", "45.97")
	if err == nil || !strings.Contains(err.Error(), "above max_premium_price") {
		t.Fatalf("error=%v", err)
	}
	// Money is compared as an exact decimal, never through a float.
	if err := assertPremiumPriceWithinCeiling("0.1", "0.3"); err != nil {
		t.Fatalf("decimal comparison failed: %v", err)
	}
	if err := assertPremiumPriceWithinCeiling("notaprice", "45.97"); err == nil {
		t.Fatal("a non-numeric price was accepted")
	}
}

func TestGlueImportIDRequiresHostInsideDomain(t *testing.T) {
	t.Parallel()

	domain, host, err := parseGlueImportID("Example.Invalid.:NS1.Example.Invalid.")
	if err != nil || domain != "example.invalid" || host != "ns1.example.invalid" {
		t.Fatalf("domain=%q host=%q error=%v", domain, host, err)
	}
	for _, invalid := range []string{
		" example.invalid:ns1.example.invalid",
		"example.invalid:ns1.other.invalid",
		"example.invalid",
		"example.invalid:ns1.example.invalid:extra",
		"example.invalid:",
	} {
		if _, _, err := parseGlueImportID(invalid); err == nil {
			t.Errorf("parseGlueImportID(%q) was accepted", invalid)
		}
	}
	// A sibling suffix must not be mistaken for a subdomain.
	if _, _, err := parseGlueImportID("example.invalid:ns1.notexample.invalid"); err == nil {
		t.Error("a host sharing only a suffix was accepted")
	}
}

func TestGlueConfigRejectsAddresslessAndOutOfDomainHosts(t *testing.T) {
	t.Parallel()

	glueResource := &GlueRecordResource{}
	schemaResponse := &resource.SchemaResponse{}
	glueResource.Schema(context.Background(), resource.SchemaRequest{}, schemaResponse)

	build := func(values map[string]tftypes.Value) tfsdk.Config {
		objectType := schemaResponse.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
		full := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
		for name, attributeType := range objectType.AttributeTypes {
			full[name] = tftypes.NewValue(attributeType, nil)
		}
		for name, value := range values {
			full[name] = value
		}
		return tfsdk.Config{Schema: schemaResponse.Schema, Raw: tftypes.NewValue(objectType, full)}
	}

	addressless := &resource.ValidateConfigResponse{}
	glueResource.ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: build(map[string]tftypes.Value{
		"domain": tftypes.NewValue(tftypes.String, "example.invalid"),
		"host":   tftypes.NewValue(tftypes.String, "ns1.example.invalid"),
	})}, addressless)
	if !addressless.Diagnostics.HasError() {
		t.Error("a glue record with no address was accepted")
	}

	outside := &resource.ValidateConfigResponse{}
	glueResource.ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: build(map[string]tftypes.Value{
		"domain": tftypes.NewValue(tftypes.String, "example.invalid"),
		"host":   tftypes.NewValue(tftypes.String, "ns1.other.invalid"),
		"ipv4":   tftypes.NewValue(tftypes.String, "192.0.2.53"),
	})}, outside)
	if !outside.Diagnostics.HasError() {
		t.Error("a glue host outside its domain was accepted")
	}
}

func TestDomainConfigGuardsRegistrationAndDNSOnlyMixups(t *testing.T) {
	t.Parallel()

	domainResource := &DomainResource{}
	schemaResponse := &resource.SchemaResponse{}
	domainResource.Schema(context.Background(), resource.SchemaRequest{}, schemaResponse)
	objectType := schemaResponse.Schema.Type().TerraformType(context.Background()).(tftypes.Object)

	build := func(values map[string]tftypes.Value) tfsdk.Config {
		full := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
		for name, attributeType := range objectType.AttributeTypes {
			full[name] = tftypes.NewValue(attributeType, nil)
		}
		full["domain"] = tftypes.NewValue(tftypes.String, "example.invalid")
		full["service"] = tftypes.NewValue(tftypes.String, "dns")
		full["term"] = tftypes.NewValue(tftypes.Number, 1)
		full["currency"] = tftypes.NewValue(tftypes.String, "USD")
		for name, value := range values {
			full[name] = value
		}
		return tfsdk.Config{Schema: schemaResponse.Schema, Raw: tftypes.NewValue(objectType, full)}
	}

	// A registration with no contacts is refused during validation.
	missingContacts := &resource.ValidateConfigResponse{}
	domainResource.ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: build(map[string]tftypes.Value{
		"dns_only": tftypes.NewValue(tftypes.Bool, false),
	})}, missingContacts)
	if !missingContacts.Diagnostics.HasError() {
		t.Error("a registration without contacts was accepted")
	}

	// Premium without a ceiling is refused.
	premiumWithoutCeiling := &resource.ValidateConfigResponse{}
	domainResource.ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: build(map[string]tftypes.Value{
		"dns_only": tftypes.NewValue(tftypes.Bool, false),
		"premium":  tftypes.NewValue(tftypes.Bool, true),
	})}, premiumWithoutCeiling)
	if !premiumWithoutCeiling.Diagnostics.HasError() {
		t.Error("a premium registration without max_premium_price was accepted")
	}

	// A DNS-only domain must not carry premium pricing.
	dnsOnlyPremium := &resource.ValidateConfigResponse{}
	domainResource.ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: build(map[string]tftypes.Value{
		"dns_only": tftypes.NewValue(tftypes.Bool, true),
		"premium":  tftypes.NewValue(tftypes.Bool, true),
	})}, dnsOnlyPremium)
	if !dnsOnlyPremium.Diagnostics.HasError() {
		t.Error("a DNS-only domain with premium pricing was accepted")
	}
}

// The Phase 2 audit rule applies to every new data source too.
func TestPhase3DataSourcesReturnConfiguredArgumentsUnchanged(t *testing.T) {
	t.Parallel()

	const configuredDomain = "Example.Invalid."

	tests := []struct {
		name     string
		response string
		source   func(*Client) datasource.DataSource
	}{
		{
			name:     "easydns_domain",
			response: `{"msg":"OK","data":{"id":"example.invalid","domain":"example.invalid","exists":"Y","onsystem":"Y","expiry":"2030-01-02","next_due":"2029-12-01","service":3857},"status":200}`,
			source:   func(client *Client) datasource.DataSource { return &DomainDataSource{client: client} },
		},
		{
			name:     "easydns_domain_nameservers",
			response: `{"status":200,"msg":"OK","data":{"domain":"example.invalid","nameservers":["ns1.example.invalid","ns2.example.invalid"]}}`,
			source:   func(client *Client) datasource.DataSource { return &DomainNameserversDataSource{client: client} },
		},
		{
			name:     "easydns_glue_records",
			response: `{"status":200,"msg":"OK","data":{"domain":"example.invalid","total":1,"glue_records":[{"name":"ns1.example.invalid","domain":"example.invalid","ipaddress":"192.0.2.53"}]}}`,
			source:   func(client *Client) datasource.DataSource { return &GlueRecordsDataSource{client: client} },
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				_, _ = response.Write([]byte(test.response))
			}))
			defer server.Close()

			client, err := NewClientWithMode(server.URL, "token", "key", RecordWriteModeSynchronous)
			if err != nil {
				t.Fatalf("create client: %v", err)
			}
			source := test.source(client)

			schemaResponse := &datasource.SchemaResponse{}
			source.Schema(context.Background(), datasource.SchemaRequest{}, schemaResponse)
			config := configWithDomain(t, schemaResponse.Schema, configuredDomain)

			readResponse := &datasource.ReadResponse{State: tfsdk.State(config)}
			source.Read(context.Background(), datasource.ReadRequest{Config: config}, readResponse)
			if readResponse.Diagnostics.HasError() {
				t.Fatalf("read diagnostics: %v", readResponse.Diagnostics)
			}

			var domain types.String
			if diagnostics := readResponse.State.GetAttribute(context.Background(), path.Root("domain"), &domain); diagnostics.HasError() {
				t.Fatalf("read domain: %v", diagnostics)
			}
			if domain.ValueString() != configuredDomain {
				t.Fatalf("data source rewrote its configured domain: got %q, want %q", domain.ValueString(), configuredDomain)
			}
		})
	}
}

// domainPlan builds a plan whose named attributes are set and whose remaining
// attributes are null.
func domainPlan(t *testing.T, resourceSchema schema.Schema, values map[string]tftypes.Value) tfsdk.Plan {
	t.Helper()
	objectType, ok := resourceSchema.Type().TerraformType(context.Background()).(tftypes.Object)
	if !ok {
		t.Fatal("resource schema is not an object type")
	}
	full := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		full[name] = tftypes.NewValue(attributeType, nil)
	}
	for name, value := range values {
		full[name] = value
	}
	return tfsdk.Plan{Schema: resourceSchema, Raw: tftypes.NewValue(objectType, full)}
}

// EasyDNS refuses dns_only on the lite service level. The provider reports it
// during planning rather than letting the apply fail against the API.
func TestDomainConfigRejectsDNSOnlyOnLiteService(t *testing.T) {
	t.Parallel()

	domainResource := &DomainResource{}
	schemaResponse := &resource.SchemaResponse{}
	domainResource.Schema(context.Background(), resource.SchemaRequest{}, schemaResponse)
	objectType := schemaResponse.Schema.Type().TerraformType(context.Background()).(tftypes.Object)

	build := func(service string, dnsOnly bool) tfsdk.Config {
		full := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
		for name, attributeType := range objectType.AttributeTypes {
			full[name] = tftypes.NewValue(attributeType, nil)
		}
		full["domain"] = tftypes.NewValue(tftypes.String, "example.invalid")
		full["service"] = tftypes.NewValue(tftypes.String, service)
		full["term"] = tftypes.NewValue(tftypes.Number, 1)
		full["currency"] = tftypes.NewValue(tftypes.String, "CAD")
		full["dns_only"] = tftypes.NewValue(tftypes.Bool, dnsOnly)
		return tfsdk.Config{Schema: schemaResponse.Schema, Raw: tftypes.NewValue(objectType, full)}
	}

	rejected := &resource.ValidateConfigResponse{}
	domainResource.ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: build("lite", true)}, rejected)
	if !rejected.Diagnostics.HasError() {
		t.Error("dns_only on the lite service level was accepted")
	}

	// dns is the cheapest level that does support it.
	accepted := &resource.ValidateConfigResponse{}
	domainResource.ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: build("dns", true)}, accepted)
	if accepted.Diagnostics.HasError() {
		t.Errorf("dns_only on the dns service level was rejected: %v", accepted.Diagnostics)
	}
}
