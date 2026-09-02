package provider

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func TestPhase2ProviderAndRecordSchemas(t *testing.T) {
	t.Parallel()

	providerResponse := &frameworkprovider.SchemaResponse{}
	(&EasyDNSProvider{}).Schema(context.Background(), frameworkprovider.SchemaRequest{}, providerResponse)
	if providerResponse.Diagnostics.HasError() {
		t.Fatalf("provider schema diagnostics=%v", providerResponse.Diagnostics)
	}
	if _, ok := providerResponse.Schema.Attributes["record_write_mode"]; !ok {
		t.Fatal("record_write_mode provider attribute missing")
	}
	legacy, ok := providerResponse.Schema.Attributes["use_async_api"].(providerschema.BoolAttribute)
	if !ok || legacy.DeprecationMessage == "" {
		t.Fatal("use_async_api is not marked deprecated")
	}

	recordResponse := &resource.SchemaResponse{}
	(&RecordResource{}).Schema(context.Background(), resource.SchemaRequest{}, recordResponse)
	for _, name := range []string{"geozone_id", "write_mode"} {
		if _, ok := recordResponse.Schema.Attributes[name]; !ok {
			t.Errorf("record attribute %q missing", name)
		}
	}
}

func TestProtocol6SchemaHasNoImplementationErrors(t *testing.T) {
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
	for _, name := range []string{"easydns_parsed_records", "easydns_zone_soa", "easydns_geo_regions"} {
		if response.DataSourceSchemas[name] == nil {
			t.Errorf("protocol schema missing %s", name)
		}
	}
}

func TestPhase2DataSourcesAreRegistered(t *testing.T) {
	t.Parallel()

	want := map[string]bool{
		"easydns_record": false, "easydns_records": false, "easydns_parsed_records": false,
		"easydns_zone_soa": false, "easydns_geo_regions": false, "easydns_zone": false,
	}
	provider := &EasyDNSProvider{}
	for _, factory := range provider.DataSources(context.Background()) {
		response := &datasource.MetadataResponse{}
		factory().Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "easydns"}, response)
		if _, expected := want[response.TypeName]; expected {
			want[response.TypeName] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("data source %s is not registered", name)
		}
	}
}

func TestResolveRecordWriteModeCompatibility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   EasyDNSProviderModel
		envMode  string
		envAsync string
		want     RecordWriteMode
		wantErr  bool
	}{
		{name: "default", config: EasyDNSProviderModel{RecordWriteMode: types.StringNull(), UseAsyncAPI: types.BoolNull()}, want: RecordWriteModeSynchronous},
		{name: "new config", config: EasyDNSProviderModel{RecordWriteMode: types.StringValue("asynchronous"), UseAsyncAPI: types.BoolNull()}, want: RecordWriteModeAsynchronous},
		{name: "legacy config", config: EasyDNSProviderModel{RecordWriteMode: types.StringNull(), UseAsyncAPI: types.BoolValue(true)}, want: RecordWriteModeAsynchronous},
		{name: "new environment", config: EasyDNSProviderModel{RecordWriteMode: types.StringNull(), UseAsyncAPI: types.BoolNull()}, envMode: "asynchronous", want: RecordWriteModeAsynchronous},
		{name: "legacy environment", config: EasyDNSProviderModel{RecordWriteMode: types.StringNull(), UseAsyncAPI: types.BoolNull()}, envAsync: "1", want: RecordWriteModeAsynchronous},
		{name: "conflicting config", config: EasyDNSProviderModel{RecordWriteMode: types.StringValue("synchronous"), UseAsyncAPI: types.BoolValue(false)}, wantErr: true},
		{name: "invalid environment", config: EasyDNSProviderModel{RecordWriteMode: types.StringNull(), UseAsyncAPI: types.BoolNull()}, envMode: "queued", wantErr: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveRecordWriteMode(test.config, test.envMode, test.envAsync)
			if (err != nil) != test.wantErr || (!test.wantErr && got != test.want) {
				t.Fatalf("mode=%q error=%v", got, err)
			}
		})
	}
}

func TestParseRecordImportID(t *testing.T) {
	t.Parallel()

	domain, recordID, err := parseRecordImportID("BÜCHER.Example.:0012")
	if err != nil || domain != "xn--bcher-kva.example" || recordID != "12" {
		t.Fatalf("domain=%q id=%q error=%v", domain, recordID, err)
	}
	for _, invalid := range []string{" example.invalid:12", "example.invalid:", "example.invalid:abc", "example.invalid:0", "example.invalid:1:2"} {
		if _, _, err := parseRecordImportID(invalid); err == nil || !strings.Contains(err.Error(), "domain:record_id") {
			t.Errorf("parseRecordImportID(%q) error=%v", invalid, err)
		}
	}
}

func TestApplyRecordToResourceModelPreservesEquivalentConfiguredRdata(t *testing.T) {
	t.Parallel()

	model := RecordResourceModel{
		Domain: types.StringValue("Example.Invalid."), Host: types.StringValue("WWW"), Type: types.StringValue("CNAME"),
		Rdata: types.StringValue("Target.Example.Invalid."), TTL: types.Int64Value(300), Prio: types.Int64Value(0), GeozoneID: types.Int64Value(0),
	}
	record := &Record{ID: "12", Domain: "example.invalid", Host: "www", Type: "cname", Rdata: "target.example.invalid", TTL: 300}
	applyRecordToResourceModel(&model, record)
	if model.Domain.ValueString() != "example.invalid" || model.Host.ValueString() != "www" || model.Type.ValueString() != "CNAME" {
		t.Fatalf("normalized model=%+v", model)
	}
	if model.Rdata.ValueString() != "Target.Example.Invalid." {
		t.Fatalf("equivalent configured rdata was not preserved: %q", model.Rdata.ValueString())
	}

	record.TTL = 600
	applyRecordToResourceModel(&model, record)
	if model.TTL.ValueInt64() != 600 {
		t.Fatal("remote TTL drift was not reflected")
	}
}

func TestRecordTypeAndPhase2IntegerValidators(t *testing.T) {
	t.Parallel()

	expectedTypes := []string{"A", "AAAA", "AFSDB", "ANAME", "CAA", "CERT", "CNAME", "DYN", "MX", "NAPTR", "NS", "PTR", "SECONDARY", "SOA", "SPF", "SRV", "SSHFP", "STEALTH", "TLSA", "TXT", "URL", "URLHTTPS"}
	if len(knownRecordTypes) != len(expectedTypes) {
		t.Fatalf("known record types=%d, want %d", len(knownRecordTypes), len(expectedTypes))
	}
	for _, recordType := range expectedTypes {
		if !knownRecordTypes[recordType] {
			t.Errorf("documented record type %s is missing", recordType)
		}
	}

	var ttlResponse validator.Int64Response
	TTLValidator().ValidateInt64(context.Background(), validator.Int64Request{ConfigValue: types.Int64Value(299)}, &ttlResponse)
	if !ttlResponse.Diagnostics.HasError() {
		t.Fatal("TTL below 300 was accepted")
	}
	var geoResponse validator.Int64Response
	NonNegativeValidator("geozone_id").ValidateInt64(context.Background(), validator.Int64Request{ConfigValue: types.Int64Value(-1)}, &geoResponse)
	if !geoResponse.Diagnostics.HasError() {
		t.Fatal("negative geozone_id was accepted")
	}
}

func TestFindRecordByHostAndTypeRejectsDuplicates(t *testing.T) {
	t.Parallel()

	records := []Record{{ID: "1", Host: "WWW", Type: "a"}, {ID: "2", Host: "www", Type: "A"}}
	if _, err := findRecordByHostAndType(records, "www", "A"); err == nil || !strings.Contains(err.Error(), "found 2") {
		t.Fatalf("duplicate lookup error=%v", err)
	}
	if record, err := findRecordByHostAndType(records[:1], "www", "A"); err != nil || record.ID != "1" {
		t.Fatalf("record=%+v error=%v", record, err)
	}

	// A singleton lookup that matched nothing must not be reported as an
	// ambiguity; the two failures carry different diagnostic summaries.
	var match *recordMatchError
	_, missingErr := findRecordByHostAndType(records, "absent", "A")
	if !errors.As(missingErr, &match) || match.summary != "Record not found" {
		t.Fatalf("missing record summary=%v error=%v", match, missingErr)
	}
	_, duplicateErr := findRecordByHostAndType(records, "www", "A")
	if !errors.As(duplicateErr, &match) || match.summary != "Record lookup was not unique" {
		t.Fatalf("duplicate record summary=%v error=%v", match, duplicateErr)
	}
}
