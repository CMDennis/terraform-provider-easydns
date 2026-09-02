package provider

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure EasyDNSProvider satisfies various provider interfaces
var _ provider.Provider = &EasyDNSProvider{}
var _ provider.ProviderWithActions = &EasyDNSProvider{}

// EasyDNSProvider defines the provider implementation
type EasyDNSProvider struct {
	version string
}

// EasyDNSProviderModel describes the provider data model
type EasyDNSProviderModel struct {
	Environment     types.String `tfsdk:"environment"`
	APIURL          types.String `tfsdk:"api_url"`
	APIToken        types.String `tfsdk:"api_token"`
	APIKey          types.String `tfsdk:"api_key"`
	RecordWriteMode types.String `tfsdk:"record_write_mode"`
	UseAsyncAPI     types.Bool   `tfsdk:"use_async_api"`

	EnableDomainRegistration types.Bool `tfsdk:"enable_domain_registration"`

	RecordPollInterval     types.String `tfsdk:"record_poll_interval"`
	RecordReconcileTimeout types.String `tfsdk:"record_reconcile_timeout"`
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
			"record_write_mode": schema.StringAttribute{
				Description: "Default record mutation mode: 'synchronous' or 'asynchronous'. Defaults to 'synchronous'. Can also be set via EASYDNS_RECORD_WRITE_MODE.",
				Optional:    true,
				Validators: []validator.String{
					RecordWriteModeValidator(),
				},
			},
			"enable_domain_registration": schema.BoolAttribute{
				Description: "Allow easydns_domain to register domains at a registry. Registration is billable and irreversible, so it defaults to false and DNS-only domains are unaffected. Can also be set via EASYDNS_ENABLE_DOMAIN_REGISTRATION.",
				Optional:    true,
			},
			"record_poll_interval": schema.StringAttribute{
				Description: "How often a mutation is re-read while waiting for it to become observable, as a Go duration. Defaults to 2s. Raising it spends fewer requests against the EasyDNS daily budget at the cost of settling later. Can also be set via EASYDNS_RECORD_POLL_INTERVAL.",
				Optional:    true,
				Validators: []validator.String{
					DurationValidator("record_poll_interval"),
				},
			},
			"record_reconcile_timeout": schema.StringAttribute{
				Description: "How long a mutation is polled for before it is reported as unobservable, as a Go duration. Defaults to 2m. Can also be set via EASYDNS_RECORD_RECONCILE_TIMEOUT.",
				Optional:    true,
				Validators: []validator.String{
					DurationValidator("record_reconcile_timeout"),
				},
			},
			"use_async_api": schema.BoolAttribute{
				Description:        "Use the async API for record operations. This queues zone reloads instead of processing immediately, which may help with rate limiting. Defaults to false. Can also be set via EASYDNS_USE_ASYNC_API environment variable.",
				Optional:           true,
				DeprecationMessage: "use_async_api is deprecated; use record_write_mode instead.",
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

	writeMode, err := resolveRecordWriteMode(config, os.Getenv("EASYDNS_RECORD_WRITE_MODE"), os.Getenv("EASYDNS_USE_ASYNC_API"))
	if err != nil {
		resp.Diagnostics.AddError("Invalid Record Write Mode", err.Error())
		return
	}

	enableDomainRegistration := false
	if !config.EnableDomainRegistration.IsNull() {
		enableDomainRegistration = config.EnableDomainRegistration.ValueBool()
	} else if envRegistration := os.Getenv("EASYDNS_ENABLE_DOMAIN_REGISTRATION"); envRegistration != "" {
		parsed, parseErr := strconv.ParseBool(envRegistration)
		if parseErr != nil {
			resp.Diagnostics.AddError(
				"Invalid Domain Registration Setting",
				"EASYDNS_ENABLE_DOMAIN_REGISTRATION must be a boolean such as true or false.",
			)
			return
		}
		enableDomainRegistration = parsed
	}

	pollInterval, err := resolveOptionalDuration(config.RecordPollInterval, os.Getenv("EASYDNS_RECORD_POLL_INTERVAL"), "record_poll_interval")
	if err != nil {
		resp.Diagnostics.AddError("Invalid Record Poll Interval", err.Error())
		return
	}
	reconcileTimeout, err := resolveOptionalDuration(config.RecordReconcileTimeout, os.Getenv("EASYDNS_RECORD_RECONCILE_TIMEOUT"), "record_reconcile_timeout")
	if err != nil {
		resp.Diagnostics.AddError("Invalid Record Reconcile Timeout", err.Error())
		return
	}
	// Polling less often than the deadline would send exactly one read, which
	// silently defeats the wait rather than tuning it.
	if pollInterval > 0 && reconcileTimeout > 0 && pollInterval > reconcileTimeout {
		resp.Diagnostics.AddError(
			"Record Poll Interval Exceeds The Reconcile Timeout",
			fmt.Sprintf("record_poll_interval (%s) must not be longer than record_reconcile_timeout (%s).", pollInterval, reconcileTimeout),
		)
		return
	}

	// Create client
	client, err := NewConfiguredClient(apiURL, apiToken, apiKey, writeMode, enableDomainRegistration, pollInterval, reconcileTimeout)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid EasyDNS Client Configuration",
			fmt.Sprintf("Could not configure the EasyDNS API client: %s", err),
		)
		return
	}

	// Make client available to resources and data sources
	resp.DataSourceData = client
	resp.ResourceData = client
	resp.ActionData = client
}

func resolveRecordWriteMode(config EasyDNSProviderModel, environmentMode, environmentAsync string) (RecordWriteMode, error) {
	if !config.RecordWriteMode.IsNull() && !config.UseAsyncAPI.IsNull() {
		return "", fmt.Errorf("record_write_mode and deprecated use_async_api cannot both be configured")
	}
	if !config.RecordWriteMode.IsNull() {
		return parseRecordWriteMode(config.RecordWriteMode.ValueString())
	}
	if !config.UseAsyncAPI.IsNull() {
		if config.UseAsyncAPI.ValueBool() {
			return RecordWriteModeAsynchronous, nil
		}
		return RecordWriteModeSynchronous, nil
	}
	if environmentMode != "" {
		return parseRecordWriteMode(environmentMode)
	}
	if environmentAsync == "true" || environmentAsync == "1" {
		return RecordWriteModeAsynchronous, nil
	}
	if environmentAsync != "" && environmentAsync != "false" && environmentAsync != "0" {
		return "", fmt.Errorf("EASYDNS_USE_ASYNC_API must be true, false, 1, or 0")
	}
	return RecordWriteModeSynchronous, nil
}

// resolveOptionalDuration prefers configuration over environment and returns
// zero when neither is set, letting the client keep its default.
func resolveOptionalDuration(configured types.String, environment, name string) (time.Duration, error) {
	value := ""
	switch {
	case !configured.IsNull() && !configured.IsUnknown():
		value = configured.ValueString()
	case environment != "":
		value = environment
	default:
		return 0, nil
	}
	parsed, err := parsePositiveDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return parsed, nil
}

func parseRecordWriteMode(value string) (RecordWriteMode, error) {
	mode := RecordWriteMode(value)
	if mode != RecordWriteModeSynchronous && mode != RecordWriteModeAsynchronous {
		return "", fmt.Errorf("record write mode must be 'synchronous' or 'asynchronous', got %q", value)
	}
	return mode, nil
}

func (p *EasyDNSProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewRecordResource,
		NewZoneResource,
		NewDomainResource,
		NewDomainRegistrationSettingsResource,
		NewDomainNameserversResource,
		NewGlueRecordResource,
		NewMailmapResource,
	}
}

func (p *EasyDNSProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewRecordDataSource,
		NewRecordsDataSource,
		NewParsedRecordsDataSource,
		NewZoneSOADataSource,
		NewGeoRegionsDataSource,
		NewZoneDataSource,
		NewDomainDataSource,
		NewDomainsDataSource,
		NewDomainRegistrationStatusesDataSource,
		NewDomainNameserversDataSource,
		NewGlueRecordsDataSource,
		NewMailmapsDataSource,
		NewCurrentUserDataSource,
		NewServiceDataSource,
		NewSubscriptionServiceDataSource,
		NewDomainPricingDataSource,
	}
}

func (p *EasyDNSProvider) Actions(ctx context.Context) []func() action.Action {
	return []func() action.Action{
		NewForceZoneReloadAction,
		NewSetPrimaryNameserverAction,
	}
}
