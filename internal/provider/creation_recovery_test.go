package provider

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/CMDennis/terraform-provider-easydns/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type recoveryTransport func(*http.Request) (*http.Response, error)

func (transport recoveryTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func recoveryClient(t *testing.T, transport recoveryTransport) *Client {
	t.Helper()
	c, err := client.New(client.Config{
		BaseURL: "https://example.invalid", Token: "synthetic-token", Key: "synthetic-key",
		HTTPClient: &http.Client{Transport: transport}, DisableRateLimiting: true,
		RecordPollInterval: time.Millisecond, RecordReconcileTimeout: 3 * time.Millisecond,
		RetryPolicy: client.RetryPolicy{MaxAttempts: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func recoveryResponse(status int, body string) (*http.Response, error) {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func TestConfirmedCreationsRetainStateAcrossReadFailures(t *testing.T) {
	for _, kind := range []string{"domain", "record"} {
		for _, failure := range []string{"404", "403", "timeout", "cancellation"} {
			t.Run(kind+"/"+failure, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				writes, reads := 0, 0
				recovered := false
				c := recoveryClient(t, func(request *http.Request) (*http.Response, error) {
					if request.Method == http.MethodPut {
						writes++
						if failure == "cancellation" {
							cancel()
						}
						if kind == "domain" {
							return recoveryResponse(201, `{"status":201,"data":{"domain":"example.invalid","inv_id":123}}`)
						}
						return recoveryResponse(201, `{"status":201,"data":{"id":"20"}}`)
					}
					reads++
					if kind == "record" && writes == 0 {
						return recoveryResponse(200, `{"status":200,"data":[]}`)
					}
					if recovered {
						if kind == "domain" {
							return recoveryResponse(200, `{"status":200,"data":{"domain":"example.invalid","exists":"N","onsystem":"Y"}}`)
						}
						return recoveryResponse(200, `{"status":200,"data":[{"id":"20","domain":"example.invalid","host":"www","type":"A","rdata":"192.0.2.10","ttl":300}]}`)
					}
					switch failure {
					case "404":
						return recoveryResponse(404, `{"status":404}`)
					case "403":
						return recoveryResponse(403, `{"status":403}`)
					case "timeout":
						return nil, context.DeadlineExceeded
					default:
						return nil, context.Canceled
					}
				})
				var managed resource.Resource
				values := map[string]tftypes.Value{"domain": tftypes.NewValue(tftypes.String, "example.invalid"), "id": tftypes.NewValue(tftypes.String, tftypes.UnknownValue)}
				if kind == "domain" {
					managed = &DomainResource{client: c}
					values["service"] = tftypes.NewValue(tftypes.String, "dns")
					values["term"] = tftypes.NewValue(tftypes.Number, 1)
					values["currency"] = tftypes.NewValue(tftypes.String, "CAD")
					values["dns_only"] = tftypes.NewValue(tftypes.Bool, true)
					values["premium"] = tftypes.NewValue(tftypes.Bool, false)
					values["deletion_protection"] = tftypes.NewValue(tftypes.Bool, true)
					values["on_system"] = tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue)
				} else {
					managed = &RecordResource{client: c}
					values["host"] = tftypes.NewValue(tftypes.String, "www")
					values["type"] = tftypes.NewValue(tftypes.String, "A")
					values["rdata"] = tftypes.NewValue(tftypes.String, "192.0.2.10")
					values["ttl"] = tftypes.NewValue(tftypes.Number, 300)
					values["prio"] = tftypes.NewValue(tftypes.Number, 0)
					values["geozone_id"] = tftypes.NewValue(tftypes.Number, 0)
					values["last_mod"] = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
				}
				schemaResponse := &resource.SchemaResponse{}
				managed.Schema(ctx, resource.SchemaRequest{}, schemaResponse)
				plan := domainPlan(t, schemaResponse.Schema, values)
				created := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResponse.Schema, Raw: tftypes.NewValue(plan.Raw.Type(), nil)}}
				managed.Create(ctx, resource.CreateRequest{Plan: plan}, created)
				if created.Diagnostics.HasError() || created.Diagnostics.WarningsCount() != 1 || created.State.Raw.IsNull() || !created.State.Raw.IsFullyKnown() {
					t.Fatalf("confirmed creation lost usable state or would be tainted: %v", created.Diagnostics)
				}
				if writes != 1 || reads == 0 {
					t.Fatalf("writes=%d reads=%d", writes, reads)
				}
				var id types.String
				if diagnostics := created.State.GetAttribute(context.Background(), path.Root("id"), &id); diagnostics.HasError() || id.ValueString() == "" {
					t.Fatal("confirmed identity was lost")
				}
				if kind == "domain" {
					var protected types.Bool
					if diagnostics := created.State.GetAttribute(context.Background(), path.Root("deletion_protection"), &protected); diagnostics.HasError() || !protected.ValueBool() {
						t.Fatal("deletion protection was lost")
					}
				}
				// A still-unavailable refresh must retain ownership, including 404.
				pending := &resource.ReadResponse{State: created.State}
				managed.Read(context.Background(), resource.ReadRequest{State: created.State}, pending)
				if !pending.Diagnostics.HasError() || !pending.State.Raw.Equal(created.State.Raw) {
					t.Fatal("pending refresh forgot the confirmed creation")
				}
				recovered = true
				read := &resource.ReadResponse{State: pending.State}
				managed.Read(context.Background(), resource.ReadRequest{State: pending.State}, read)
				if read.Diagnostics.HasError() || read.State.Raw.IsNull() || writes != 1 {
					t.Fatalf("recovery failed or replayed a write: %v", read.Diagnostics)
				}
			})
		}
	}
}

func TestMalformedDomainRefreshRetainsProtectedState(t *testing.T) {
	for _, body := range []string{`{}`, `{"status":200,"data":{}}`, `{"status":200,"data":{"domain":"example.invalid","exists":"Y","onsystem":"unexpected"}}`} {
		r := &DomainResource{client: recoveryClient(t, func(_ *http.Request) (*http.Response, error) { return recoveryResponse(200, body) })}
		schemaResponse := &resource.SchemaResponse{}
		r.Schema(context.Background(), resource.SchemaRequest{}, schemaResponse)
		plan := domainPlan(t, schemaResponse.Schema, map[string]tftypes.Value{
			"id": tftypes.NewValue(tftypes.String, "example.invalid"), "domain": tftypes.NewValue(tftypes.String, "example.invalid"),
			"on_system": tftypes.NewValue(tftypes.Bool, true), "deletion_protection": tftypes.NewValue(tftypes.Bool, true),
		})
		state := tfsdk.State{Schema: schemaResponse.Schema, Raw: plan.Raw}
		read := &resource.ReadResponse{State: state}
		r.Read(context.Background(), resource.ReadRequest{State: state}, read)
		if !read.Diagnostics.HasError() || !read.State.Raw.Equal(state.Raw) {
			t.Fatalf("uncertain absence lost state: %v", read.Diagnostics)
		}
	}
}
