package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure EasyDNSProvider satisfies various provider interfaces
var _ provider.Provider = &EasyDNSProvider{}

// EasyDNSProvider defines the provider implementation
type EasyDNSProvider struct {
	version string
}

// EasyDNSProviderModel describes the provider data model
type EasyDNSProviderModel struct {
	Environment types.String `tfsdk:"environment"`
	APIURL      types.String `tfsdk:"api_url"`
	APIToken    types.String `tfsdk:"api_token"`
	APIKey      types.String `tfsdk:"api_key"`
	UseAsyncAPI types.Bool   `tfsdk:"use_async_api"`
}

const (
	sandboxURL    = "https://sandbox.rest.easydns.net"
	productionURL = "https://rest.easydns.net"
)

// New creates a new provider instance
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &EasyDNSProvider{
			version: version,
		}
	}
}

func (p *EasyDNSProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "easydns"
	resp.Version = p.version
}

func (p *EasyDNSProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Interact with EasyDNS API to manage DNS records.",
		Attributes: map[string]schema.Attribute{
			"environment": schema.StringAttribute{
				Description: "The EasyDNS environment: 'sandbox' or 'production'. Defaults to 'sandbox'. Can also be set via EASYDNS_ENVIRONMENT environment variable. This is ignored if api_url is set.",
				Optional:    true,
			},
			"api_url": schema.StringAttribute{
				Description: "The EasyDNS API URL. Overrides the environment setting. Can also be set via EASYDNS_API_URL environment variable.",
				Optional:    true,
			},
			"api_token": schema.StringAttribute{
				Description: "The EasyDNS API token. Can also be set via EASYDNS_API_TOKEN environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"api_key": schema.StringAttribute{
				Description: "The EasyDNS API key. Can also be set via EASYDNS_API_KEY environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"use_async_api": schema.BoolAttribute{
				Description: "Use the async API for record operations. This queues zone reloads instead of processing immediately, which may help with rate limiting. Defaults to false. Can also be set via EASYDNS_USE_ASYNC_API environment variable.",
				Optional:    true,
			},
		},
	}
}

func (p *EasyDNSProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config EasyDNSProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine API URL from environment setting or direct URL
	apiURL := sandboxURL // default

	// First, check environment setting
	environment := "sandbox"
	if !config.Environment.IsNull() {
		environment = config.Environment.ValueString()
	} else if envEnv := os.Getenv("EASYDNS_ENVIRONMENT"); envEnv != "" {
		environment = envEnv
	}

	switch environment {
	case "sandbox":
		apiURL = sandboxURL
	case "production":
		apiURL = productionURL
	default:
		resp.Diagnostics.AddError(
			"Invalid Environment",
			fmt.Sprintf("Environment must be 'sandbox' or 'production', got: %s", environment),
		)
	}

	// Direct URL overrides environment setting
	if !config.APIURL.IsNull() {
		apiURL = config.APIURL.ValueString()
	} else if envURL := os.Getenv("EASYDNS_API_URL"); envURL != "" {
		apiURL = envURL
	}

	apiToken := ""
	if !config.APIToken.IsNull() {
		apiToken = config.APIToken.ValueString()
	} else if envToken := os.Getenv("EASYDNS_API_TOKEN"); envToken != "" {
		apiToken = envToken
	}

	apiKey := ""
	if !config.APIKey.IsNull() {
		apiKey = config.APIKey.ValueString()
	} else if envKey := os.Getenv("EASYDNS_API_KEY"); envKey != "" {
		apiKey = envKey
	}

	// Validation
	if apiToken == "" {
		resp.Diagnostics.AddError(
			"Missing API Token",
			"The provider requires an API token. Set it in the provider configuration or via the EASYDNS_API_TOKEN environment variable.",
		)
	}
	if apiKey == "" {
		resp.Diagnostics.AddError(
			"Missing API Key",
			"The provider requires an API key. Set it in the provider configuration or via the EASYDNS_API_KEY environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Check for async API setting
	useAsyncAPI := false
	if !config.UseAsyncAPI.IsNull() {
		useAsyncAPI = config.UseAsyncAPI.ValueBool()
	} else if envAsync := os.Getenv("EASYDNS_USE_ASYNC_API"); envAsync != "" {
		useAsyncAPI = envAsync == "true" || envAsync == "1"
	}

	// Create client
	client := NewClient(apiURL, apiToken, apiKey, useAsyncAPI)

	// Make client available to resources and data sources
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *EasyDNSProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewRecordResource,
		NewZoneResource,
	}
}

func (p *EasyDNSProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewRecordDataSource,
		NewRecordsDataSource,
		NewZoneDataSource,
	}
}
