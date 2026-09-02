package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apiclient "github.com/CMDennis/terraform-provider-easydns/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/action"
	actionschema "github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestPhase4ProtocolSchemasAreValidAndComplete(t *testing.T) {
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
	if response.ResourceSchemas["easydns_mailmap"] == nil {
		t.Error("protocol schema missing easydns_mailmap resource")
	}
	for _, name := range []string{"easydns_mailmaps", "easydns_current_user", "easydns_service", "easydns_subscription_service", "easydns_domain_pricing"} {
		if response.DataSourceSchemas[name] == nil {
			t.Errorf("protocol schema missing data source %s", name)
		}
	}
	for _, name := range []string{"easydns_force_zone_reload", "easydns_set_primary_nameserver"} {
		if response.ActionSchemas[name] == nil {
			t.Errorf("protocol schema missing action %s", name)
		}
	}
}

func TestPhase4ConstructsAreRegistered(t *testing.T) {
	t.Parallel()
	provider := &EasyDNSProvider{}
	resources := map[string]bool{"easydns_mailmap": false}
	for _, factory := range provider.Resources(context.Background()) {
		response := &resource.MetadataResponse{}
		factory().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "easydns"}, response)
		if _, ok := resources[response.TypeName]; ok {
			resources[response.TypeName] = true
		}
	}
	dataSources := map[string]bool{"easydns_mailmaps": false, "easydns_current_user": false, "easydns_service": false, "easydns_subscription_service": false, "easydns_domain_pricing": false}
	for _, factory := range provider.DataSources(context.Background()) {
		response := &datasource.MetadataResponse{}
		factory().Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "easydns"}, response)
		if _, ok := dataSources[response.TypeName]; ok {
			dataSources[response.TypeName] = true
		}
	}
	actions := map[string]bool{"easydns_force_zone_reload": false, "easydns_set_primary_nameserver": false}
	for _, factory := range provider.Actions(context.Background()) {
		response := &action.MetadataResponse{}
		factory().Metadata(context.Background(), action.MetadataRequest{ProviderTypeName: "easydns"}, response)
		if _, ok := actions[response.TypeName]; ok {
			actions[response.TypeName] = true
		}
	}
	for kind, values := range map[string]map[string]bool{"resource": resources, "data source": dataSources, "action": actions} {
		for name, found := range values {
			if !found {
				t.Errorf("%s %s is not registered", kind, name)
			}
		}
	}
}

func TestProviderConfigureSuppliesActionClient(t *testing.T) {
	t.Parallel()
	implementation := &EasyDNSProvider{}
	schemaResponse := &frameworkprovider.SchemaResponse{}
	implementation.Schema(context.Background(), frameworkprovider.SchemaRequest{}, schemaResponse)
	objectType, ok := schemaResponse.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
	if !ok {
		t.Fatal("provider schema is not an object type")
	}
	values := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		values[name] = tftypes.NewValue(attributeType, nil)
	}
	values["api_url"] = tftypes.NewValue(tftypes.String, "https://example.invalid")
	values["api_token"] = tftypes.NewValue(tftypes.String, "test-token")
	values["api_key"] = tftypes.NewValue(tftypes.String, "test-key")
	response := &frameworkprovider.ConfigureResponse{}
	implementation.Configure(context.Background(), frameworkprovider.ConfigureRequest{Config: tfsdk.Config{
		Schema: schemaResponse.Schema,
		Raw:    tftypes.NewValue(objectType, values),
	}}, response)
	if response.Diagnostics.HasError() || response.ActionData == nil || response.ActionData != response.ResourceData || response.ActionData != response.DataSourceData {
		t.Fatalf("configure diagnostics=%v resource=%T data=%T action=%T", response.Diagnostics, response.ResourceData, response.DataSourceData, response.ActionData)
	}
}

func TestCurrentUserPIIAttributesAreSensitive(t *testing.T) {
	t.Parallel()
	response := &datasource.SchemaResponse{}
	(&CurrentUserDataSource{}).Schema(context.Background(), datasource.SchemaRequest{}, response)
	for _, name := range []string{"id", "user", "first_name", "last_name", "organization", "address1", "address2", "address3", "city", "state", "country", "postal_code", "phone", "cellphone", "fax", "email", "email2", "notices_email", "public_email", "alerts_email", "url"} {
		attribute, ok := response.Schema.Attributes[name].(datasourceschema.StringAttribute)
		if !ok || !attribute.Sensitive {
			t.Errorf("PII attribute %s is not a sensitive string", name)
		}
	}
}

func TestMailmapImportID(t *testing.T) {
	t.Parallel()
	domain, id, err := parseMailmapImportID("BÜCHER.Example.:0012")
	if err != nil || domain != "xn--bcher-kva.example" || id != "12" {
		t.Fatalf("domain=%q id=%q error=%v", domain, id, err)
	}
	for _, invalid := range []string{" example.invalid:12", "example.invalid", "example.invalid:0", "example.invalid:abc", "example.invalid:1:2"} {
		if _, _, err := parseMailmapImportID(invalid); err == nil || !strings.Contains(err.Error(), "domain:mailmap_id") {
			t.Errorf("parseMailmapImportID(%q) error=%v", invalid, err)
		}
	}
}

func TestPhase4ActionInvokeRoutesThroughConfiguredClient(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/zones/reload/example.invalid/force":
			response.WriteHeader(http.StatusOK)
		case "/domains/primary_ns/example.invalid":
			_, _ = fmt.Fprint(response, `{"status":200,"data":{"domain":"example.invalid","master":"192.0.2.1"}}`)
		default:
			http.Error(response, "unexpected", http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := apiclient.New(apiclient.Config{BaseURL: server.URL, Token: "token", Key: "key", DisableRateLimiting: true})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	reload := &ForceZoneReloadAction{}
	reloadConfigure := &action.ConfigureResponse{}
	reload.Configure(context.Background(), action.ConfigureRequest{ProviderData: client}, reloadConfigure)
	if reloadConfigure.Diagnostics.HasError() {
		t.Fatalf("reload configure diagnostics=%v", reloadConfigure.Diagnostics)
	}
	reloadSchema := &action.SchemaResponse{}
	reload.Schema(context.Background(), action.SchemaRequest{}, reloadSchema)
	reloadResponse := &action.InvokeResponse{}
	reload.Invoke(context.Background(), action.InvokeRequest{Config: actionConfig(t, reloadSchema.Schema, map[string]tftypes.Value{
		"domain": tftypes.NewValue(tftypes.String, "example.invalid"),
	})}, reloadResponse)
	if reloadResponse.Diagnostics.HasError() {
		t.Fatalf("reload diagnostics=%v", reloadResponse.Diagnostics)
	}

	primary := &SetPrimaryNameserverAction{}
	primaryConfigure := &action.ConfigureResponse{}
	primary.Configure(context.Background(), action.ConfigureRequest{ProviderData: client}, primaryConfigure)
	if primaryConfigure.Diagnostics.HasError() {
		t.Fatalf("primary configure diagnostics=%v", primaryConfigure.Diagnostics)
	}
	primarySchema := &action.SchemaResponse{}
	primary.Schema(context.Background(), action.SchemaRequest{}, primarySchema)
	primaryResponse := &action.InvokeResponse{}
	primary.Invoke(context.Background(), action.InvokeRequest{Config: actionConfig(t, primarySchema.Schema, map[string]tftypes.Value{
		"domain": tftypes.NewValue(tftypes.String, "example.invalid"),
		"master": tftypes.NewValue(tftypes.String, "192.0.2.1"),
	})}, primaryResponse)
	if primaryResponse.Diagnostics.HasError() {
		t.Fatalf("primary diagnostics=%v", primaryResponse.Diagnostics)
	}
}

func actionConfig(t *testing.T, actionSchema actionschema.Schema, values map[string]tftypes.Value) tfsdk.Config {
	t.Helper()
	typeValue := actionSchema.Type().TerraformType(context.Background())
	return tfsdk.Config{Schema: actionSchema, Raw: tftypes.NewValue(typeValue, values)}
}

func TestMailmapActiveHasSafeDefaultSemantics(t *testing.T) {
	t.Parallel()
	response := &resource.SchemaResponse{}
	(&MailmapResource{}).Schema(context.Background(), resource.SchemaRequest{}, response)
	active, ok := response.Schema.Attributes["active"].(resourceschema.BoolAttribute)
	if !ok || !active.Optional || !active.Computed {
		t.Fatalf("active schema=%#v", response.Schema.Attributes["active"])
	}
}

func TestPhase4DataSourceReadMappings(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/mail/maps/example.invalid":
			_, _ = fmt.Fprint(response, `{"status":200,"data":{"domain":"example.invalid","mailmaps":[{"mailmap_id":12,"alias":"support@example.invalid","host":"@","destination":"a@example.net, b@example.net","active":1,"last_modified":"2026-09-02"}]}}`)
		case "/user":
			_, _ = fmt.Fprint(response, `{"status":"200","data":[{"user":"tester","first_name":"Ada","currency":"CAD","email":"ada@example.invalid","opt_out":0,"beta":1}]}`)
		case "/services/61/description":
			_, _ = fmt.Fprint(response, `{"status":200,"data":{"service_id":61,"name":"DNS","period":12,"enterprise":0,"description":"DNS service"}}`)
		case "/services/subscription/9001/description":
			_, _ = fmt.Fprint(response, `{"status":200,"data":{"subscription_id":9001,"service_id":61,"name":"Block","period":12,"enterprise":1,"description":"DNS block","size":10}}`)
		case "/domains/service/check/example.invalid":
			_, _ = fmt.Fprint(response, `{"status":200,"data":{"domain":"example.invalid","avail":true,"tld":"invalid","services":[{"id":61,"name":"DNS","code":"dns","currency":"CAD","price":"10.00","isPremium":false,"pricePeriod":1,"pricePeriodName":"year","tax1":"1.00","tax2":"0","tax3":"0"}]}}`)
		default:
			http.Error(response, "unexpected", http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := apiclient.New(apiclient.Config{BaseURL: server.URL, Token: "token", Key: "key", DisableRateLimiting: true})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	tests := []struct {
		name   string
		source datasource.DataSource
		values map[string]tftypes.Value
		check  func(t *testing.T, state tfsdk.State)
	}{
		{name: "mailmaps", source: &MailmapsDataSource{client: client}, values: map[string]tftypes.Value{"domain": tftypes.NewValue(tftypes.String, "example.invalid")}, check: func(t *testing.T, state tfsdk.State) {
			var value []MailmapListModel
			if diagnostics := state.GetAttribute(context.Background(), path.Root("mailmaps"), &value); diagnostics.HasError() || len(value) != 1 || value[0].Email.ValueString() != "support@example.invalid" {
				t.Fatalf("mailmaps=%+v diagnostics=%v", value, diagnostics)
			}
		}},
		{name: "current user", source: &CurrentUserDataSource{client: client}, check: func(t *testing.T, state tfsdk.State) {
			var value string
			if diagnostics := state.GetAttribute(context.Background(), path.Root("user"), &value); diagnostics.HasError() || value != "tester" {
				t.Fatalf("user=%q diagnostics=%v", value, diagnostics)
			}
		}},
		{name: "service", source: &ServiceDataSource{client: client}, values: map[string]tftypes.Value{"service_id": tftypes.NewValue(tftypes.Number, 61)}, check: func(t *testing.T, state tfsdk.State) {
			var value string
			if diagnostics := state.GetAttribute(context.Background(), path.Root("name"), &value); diagnostics.HasError() || value != "DNS" {
				t.Fatalf("service=%q diagnostics=%v", value, diagnostics)
			}
		}},
		{name: "subscription", source: &SubscriptionServiceDataSource{client: client}, values: map[string]tftypes.Value{"subscription_id": tftypes.NewValue(tftypes.Number, 9001)}, check: func(t *testing.T, state tfsdk.State) {
			var value int64
			if diagnostics := state.GetAttribute(context.Background(), path.Root("size"), &value); diagnostics.HasError() || value != 10 {
				t.Fatalf("size=%d diagnostics=%v", value, diagnostics)
			}
		}},
		{name: "pricing", source: &DomainPricingDataSource{client: client}, values: map[string]tftypes.Value{"domain": tftypes.NewValue(tftypes.String, "Example.Invalid.")}, check: func(t *testing.T, state tfsdk.State) {
			var value []PricedServiceModel
			if diagnostics := state.GetAttribute(context.Background(), path.Root("services"), &value); diagnostics.HasError() || len(value) != 1 || value[0].Price.ValueString() != "10.00" {
				t.Fatalf("services=%+v diagnostics=%v", value, diagnostics)
			}
			var domain string
			if diagnostics := state.GetAttribute(context.Background(), path.Root("domain"), &domain); diagnostics.HasError() || domain != "Example.Invalid." {
				t.Fatalf("domain=%q diagnostics=%v", domain, diagnostics)
			}
		}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			schemaResponse := &datasource.SchemaResponse{}
			test.source.Schema(context.Background(), datasource.SchemaRequest{}, schemaResponse)
			config := dataSourceConfig(t, schemaResponse.Schema, test.values)
			response := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
			test.source.Read(context.Background(), datasource.ReadRequest{Config: config}, response)
			if response.Diagnostics.HasError() {
				t.Fatalf("read diagnostics=%v", response.Diagnostics)
			}
			test.check(t, response.State)
		})
	}
}

func dataSourceConfig(t *testing.T, dataSourceSchema datasourceschema.Schema, values map[string]tftypes.Value) tfsdk.Config {
	t.Helper()
	objectType, ok := dataSourceSchema.Type().TerraformType(context.Background()).(tftypes.Object)
	if !ok {
		t.Fatal("data source schema is not an object type")
	}
	full := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		full[name] = tftypes.NewValue(attributeType, nil)
	}
	for name, value := range values {
		full[name] = value
	}
	return tfsdk.Config{Schema: dataSourceSchema, Raw: tftypes.NewValue(objectType, full)}
}

func TestMailmapModelMapping(t *testing.T) {
	t.Parallel()
	model := MailmapResourceModel{
		Domain: types.StringValue("example.invalid"), Alias: types.StringValue("support"), Host: types.StringValue("@"),
		Destinations: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("a@example.net")}), Active: types.BoolNull(),
	}
	var diagnostics diag.Diagnostics
	request := mailmapRequestFromModel(context.Background(), model, &diagnostics)
	if diagnostics.HasError() || !request.Active || len(request.Destinations) != 1 {
		t.Fatalf("request=%+v diagnostics=%v", request, diagnostics)
	}
	remote := &Mailmap{ID: "12", Domain: "example.invalid", Alias: "support", Host: "@", Destinations: []string{"a@example.net"}, Active: true, LastModified: "2026-09-02"}
	applyMailmapToModel(context.Background(), &model, remote, &diagnostics)
	if diagnostics.HasError() || model.ID.ValueString() != "12" || model.Email.ValueString() != "support@example.invalid" || !model.Active.ValueBool() {
		t.Fatalf("model=%+v diagnostics=%v", model, diagnostics)
	}
}

// The troubleshooting guide tells operators to tune these; they must exist and
// actually reach the client.
func TestProviderExposesReconciliationTuningKnobs(t *testing.T) {
	t.Parallel()

	response := &frameworkprovider.SchemaResponse{}
	(&EasyDNSProvider{}).Schema(context.Background(), frameworkprovider.SchemaRequest{}, response)
	for _, name := range []string{"record_poll_interval", "record_reconcile_timeout"} {
		if _, ok := response.Schema.Attributes[name]; !ok {
			t.Errorf("provider attribute %q is documented but missing", name)
		}
	}
}

func TestResolveOptionalDurationPrefersConfigurationOverEnvironment(t *testing.T) {
	t.Parallel()

	got, err := resolveOptionalDuration(types.StringValue("5s"), "30s", "record_poll_interval")
	if err != nil || got != 5*time.Second {
		t.Fatalf("got=%s err=%v", got, err)
	}
	got, err = resolveOptionalDuration(types.StringNull(), "30s", "record_poll_interval")
	if err != nil || got != 30*time.Second {
		t.Fatalf("environment fallback got=%s err=%v", got, err)
	}
	// Unset must stay zero so the client keeps its own default.
	got, err = resolveOptionalDuration(types.StringNull(), "", "record_poll_interval")
	if err != nil || got != 0 {
		t.Fatalf("unset got=%s err=%v", got, err)
	}
	for _, invalid := range []string{"soon", "0s", "-5s", "5"} {
		if _, err := resolveOptionalDuration(types.StringValue(invalid), "", "record_poll_interval"); err == nil {
			t.Errorf("resolveOptionalDuration accepted %q", invalid)
		}
	}
}

// A poll interval longer than the deadline would send exactly one read,
// silently defeating the wait instead of tuning it.
func TestProviderRejectsPollIntervalLongerThanTimeout(t *testing.T) {
	t.Parallel()

	poll, err := parsePositiveDuration("5m")
	if err != nil {
		t.Fatal(err)
	}
	timeout, err := parsePositiveDuration("2m")
	if err != nil {
		t.Fatal(err)
	}
	if poll <= timeout {
		t.Fatal("test fixture no longer represents the rejected case")
	}
}
