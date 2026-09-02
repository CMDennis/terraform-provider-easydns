package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Terraform rejects a data source whose result changes an argument the
// configuration set. Normalizing `domain` into the result made every
// non-canonical configured domain fail with an inconsistent-result error, so
// each data source must echo its configured arguments back unchanged.
func TestDataSourcesReturnConfiguredArgumentsUnchanged(t *testing.T) {
	t.Parallel()

	const configuredDomain = "Example.Invalid."

	tests := []struct {
		name     string
		response string
		read     func(*testing.T, *Client, tfsdk.Config) tfsdk.State
		schema   dsschema.Schema
	}{
		{
			name:     "easydns_zone_soa",
			response: `{"data":{"domain":"example.invalid","soa":2026090101},"status":200}`,
			schema:   zoneSOASchema(t),
			read: func(t *testing.T, client *Client, config tfsdk.Config) tfsdk.State {
				source := &ZoneSOADataSource{client: client}
				return readDataSource(t, source.Read, config)
			},
		},
		{
			name:     "easydns_zone",
			response: `{"data":{"id":"55","domain":"example.invalid","exists":"Y","onsystem":"Y","expiry":null,"next_due":"","service":"1"},"status":200}`,
			schema:   zoneSchema(t),
			read: func(t *testing.T, client *Client, config tfsdk.Config) tfsdk.State {
				source := &ZoneDataSource{client: client}
				return readDataSource(t, source.Read, config)
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.Header().Set("Content-Type", "application/json")
				_, _ = response.Write([]byte(test.response))
			}))
			defer server.Close()

			client, err := NewClientWithMode(server.URL, "token", "key", RecordWriteModeSynchronous)
			if err != nil {
				t.Fatalf("create client: %v", err)
			}

			config := configWithDomain(t, test.schema, configuredDomain)
			state := test.read(t, client, config)

			var domain types.String
			if diagnostics := state.GetAttribute(context.Background(), path.Root("domain"), &domain); diagnostics.HasError() {
				t.Fatalf("read domain from state: %v", diagnostics)
			}
			if domain.ValueString() != configuredDomain {
				t.Fatalf("data source rewrote its configured domain: got %q, want %q", domain.ValueString(), configuredDomain)
			}
		})
	}
}

func zoneSOASchema(t *testing.T) dsschema.Schema {
	t.Helper()
	response := &datasource.SchemaResponse{}
	(&ZoneSOADataSource{}).Schema(context.Background(), datasource.SchemaRequest{}, response)
	return response.Schema
}

func zoneSchema(t *testing.T) dsschema.Schema {
	t.Helper()
	response := &datasource.SchemaResponse{}
	(&ZoneDataSource{}).Schema(context.Background(), datasource.SchemaRequest{}, response)
	return response.Schema
}

// configWithDomain builds a configuration whose only set argument is `domain`.
func configWithDomain(t *testing.T, schema dsschema.Schema, domain string) tfsdk.Config {
	t.Helper()
	objectType, ok := schema.Type().TerraformType(context.Background()).(tftypes.Object)
	if !ok {
		t.Fatalf("schema is not an object type")
	}
	values := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		values[name] = tftypes.NewValue(attributeType, nil)
	}
	values["domain"] = tftypes.NewValue(tftypes.String, domain)
	return tfsdk.Config{Schema: schema, Raw: tftypes.NewValue(objectType, values)}
}

func readDataSource(t *testing.T, read func(context.Context, datasource.ReadRequest, *datasource.ReadResponse), config tfsdk.Config) tfsdk.State {
	t.Helper()
	response := &datasource.ReadResponse{State: tfsdk.State(config)}
	read(context.Background(), datasource.ReadRequest{Config: config}, response)
	if response.Diagnostics.HasError() {
		t.Fatalf("data source read diagnostics: %v", response.Diagnostics)
	}
	return response.State
}
