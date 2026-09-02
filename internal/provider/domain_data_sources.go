package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource = &DomainDataSource{}
	_ datasource.DataSource = &DomainsDataSource{}
	_ datasource.DataSource = &DomainRegistrationStatusesDataSource{}
	_ datasource.DataSource = &DomainNameserversDataSource{}
	_ datasource.DataSource = &GlueRecordsDataSource{}
)

// configureDataSourceClient is the shared Configure body for the Phase 3 read
// models.
func configureDataSourceClient(req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) *Client {
	if req.ProviderData == nil {
		return nil
	}
	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *Client, got: %T.", req.ProviderData))
		return nil
	}
	return client
}

// ---------------------------------------------------------------- easydns_domain

type DomainDataSource struct{ client *Client }

type DomainDataSourceModel struct {
	Domain         types.String `tfsdk:"domain"`
	ID             types.String `tfsdk:"id"`
	Exists         types.Bool   `tfsdk:"exists"`
	OnSystem       types.Bool   `tfsdk:"on_system"`
	Expiry         types.String `tfsdk:"expiry"`
	NextDue        types.String `tfsdk:"next_due"`
	ClonedTo       types.String `tfsdk:"cloned_to"`
	ServiceID      types.Int64  `tfsdk:"service_id"`
	SubscriptionID types.Int64  `tfsdk:"subscription_id"`
}

func NewDomainDataSource() datasource.DataSource { return &DomainDataSource{} }

func (d *DomainDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (d *DomainDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches the EasyDNS record of a domain on the system.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				Description: "The domain to look up.",
				Required:    true,
				Validators:  []validator.String{DomainNameValidator()},
			},
			"id":              schema.StringAttribute{Description: "The EasyDNS domain identifier.", Computed: true},
			"exists":          schema.BoolAttribute{Description: "Whether the domain exists at the registry.", Computed: true},
			"on_system":       schema.BoolAttribute{Description: "Whether the domain exists on the EasyDNS system.", Computed: true},
			"expiry":          schema.StringAttribute{Description: "Registration expiry date, empty for a domain without registration.", Computed: true},
			"next_due":        schema.StringAttribute{Description: "Date the EasyDNS service is next due.", Computed: true},
			"cloned_to":       schema.StringAttribute{Description: "Domain this one is cloned to, when cloning is enabled.", Computed: true},
			"service_id":      schema.Int64Attribute{Description: "Numeric EasyDNS service ID in use.", Computed: true},
			"subscription_id": schema.Int64Attribute{Description: "Subscription block ID when the domain belongs to one.", Computed: true},
		},
	}
}

func (d *DomainDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *DomainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config DomainDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain, err := d.client.GetDomain(ctx, config.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading domain",
			fmt.Sprintf("Could not read %s: %s", config.Domain.ValueString(), err))
		return
	}

	// `domain` is a required argument and is returned exactly as configured.
	config.ID = types.StringValue(domain.ID)
	config.Exists = types.BoolValue(domain.Exists)
	config.OnSystem = types.BoolValue(domain.OnSystem)
	config.Expiry = types.StringValue(domain.Expiry)
	config.NextDue = types.StringValue(domain.NextDue)
	config.ClonedTo = types.StringValue(domain.ClonedTo)
	config.ServiceID = types.Int64Value(domain.Service)
	config.SubscriptionID = types.Int64Value(domain.SubscriptionID)
	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}

// --------------------------------------------------------------- easydns_domains

type DomainsDataSource struct{ client *Client }

type DomainsDataSourceModel struct {
	User    types.String         `tfsdk:"user"`
	Domains []DomainSummaryModel `tfsdk:"domains"`
}

type DomainSummaryModel struct {
	Domain types.String `tfsdk:"domain"`
	Link   types.String `tfsdk:"link"`
}

func NewDomainsDataSource() datasource.DataSource { return &DomainsDataSource{} }

func (d *DomainsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domains"
}

func (d *DomainsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists every domain associated with an EasyDNS user, sorted by domain name.",
		Attributes: map[string]schema.Attribute{
			"user": schema.StringAttribute{
				Description: "The user whose domains are listed. Defaults to the authenticated account.",
				Optional:    true,
			},
			"domains": schema.ListNestedAttribute{
				Description: "The domains, sorted by name.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"domain": schema.StringAttribute{Description: "The domain name.", Computed: true},
					"link":   schema.StringAttribute{Description: "The EasyDNS API URL for this domain.", Computed: true},
				}},
			},
		},
	}
}

func (d *DomainsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *DomainsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config DomainsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, domains, err := d.client.ListUserDomains(ctx, config.User.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing domains", fmt.Sprintf("Could not list domains: %s", err))
		return
	}

	config.Domains = make([]DomainSummaryModel, len(domains))
	for index, domain := range domains {
		config.Domains[index] = DomainSummaryModel{
			Domain: types.StringValue(domain.Domain),
			Link:   types.StringValue(domain.Link),
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}

// ------------------------------------ easydns_domain_registration_statuses

type DomainRegistrationStatusesDataSource struct{ client *Client }

type DomainRegistrationStatusesModel struct {
	Statuses []RegistrationStatusModel `tfsdk:"statuses"`
}

type RegistrationStatusModel struct {
	Domain          types.String `tfsdk:"domain"`
	Reglock         types.Bool   `tfsdk:"reglock"`
	Renewal         types.String `tfsdk:"renewal"`
	AutoRenew       types.Bool   `tfsdk:"auto_renew"`
	LetExpire       types.Bool   `tfsdk:"let_expire"`
	LetExpireFailed types.Bool   `tfsdk:"let_expire_failed"`
	Expiry          types.String `tfsdk:"expiry"`
	LocalRegistrar  types.Bool   `tfsdk:"local_registrar"`
	SupportsReglock types.Bool   `tfsdk:"supports_reglock"`
}

func NewDomainRegistrationStatusesDataSource() datasource.DataSource {
	return &DomainRegistrationStatusesDataSource{}
}

func (d *DomainRegistrationStatusesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain_registration_statuses"
}

func (d *DomainRegistrationStatusesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the registrar lock and renewal policy of every domain on the authenticated account, sorted by domain name. The auto-renewal card identifier is deliberately not exposed here.",
		Attributes: map[string]schema.Attribute{
			"statuses": schema.ListNestedAttribute{
				Description: "Registration statuses, sorted by domain.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"domain":            schema.StringAttribute{Description: "The domain name.", Computed: true},
					"reglock":           schema.BoolAttribute{Description: "Whether the registrar lock is on.", Computed: true},
					"renewal":           schema.StringAttribute{Description: "Renewal action: remind, renew, or expire.", Computed: true},
					"auto_renew":        schema.BoolAttribute{Description: "Whether automated renewal payment is configured.", Computed: true},
					"let_expire":        schema.BoolAttribute{Description: "Whether the domain is set to expire at the registry.", Computed: true},
					"let_expire_failed": schema.BoolAttribute{Description: "Whether EasyDNS failed to read let_expire from the registry.", Computed: true},
					"expiry":            schema.StringAttribute{Description: "Expiry date, or the date service is next due.", Computed: true},
					"local_registrar":   schema.BoolAttribute{Description: "Whether EasyDNS is the registrar of record.", Computed: true},
					"supports_reglock":  schema.BoolAttribute{Description: "Whether this TLD supports registrar locking.", Computed: true},
				}},
			},
		},
	}
}

func (d *DomainRegistrationStatusesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *DomainRegistrationStatusesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config DomainRegistrationStatusesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	statuses, err := d.client.ListRegistrationStatuses(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading registration statuses", fmt.Sprintf("Could not read registration statuses: %s", err))
		return
	}

	config.Statuses = make([]RegistrationStatusModel, len(statuses))
	for index, status := range statuses {
		config.Statuses[index] = RegistrationStatusModel{
			Domain:          types.StringValue(status.Domain),
			Reglock:         types.BoolValue(status.Reglock),
			Renewal:         types.StringValue(status.Renewal),
			AutoRenew:       types.BoolValue(status.AutoRenew),
			LetExpire:       types.BoolValue(status.LetExpire),
			LetExpireFailed: types.BoolValue(status.LetExpireFailed),
			Expiry:          types.StringValue(status.Expiry),
			LocalRegistrar:  types.BoolValue(status.LocalRegistrar),
			SupportsReglock: types.BoolValue(status.SupportsReglock),
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}

// ---------------------------------------------- easydns_domain_nameservers

type DomainNameserversDataSource struct{ client *Client }

type DomainNameserversDataSourceModel struct {
	Domain      types.String `tfsdk:"domain"`
	Nameservers types.Set    `tfsdk:"nameservers"`
}

func NewDomainNameserversDataSource() datasource.DataSource { return &DomainNameserversDataSource{} }

func (d *DomainNameserversDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain_nameservers"
}

func (d *DomainNameserversDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches the nameservers currently delegated for a domain.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				Description: "The domain to look up.",
				Required:    true,
				Validators:  []validator.String{DomainNameValidator()},
			},
			"nameservers": schema.SetAttribute{
				Description: "The delegated nameservers. Order is not significant.",
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *DomainNameserversDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *DomainNameserversDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config DomainNameserversDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	nameservers, err := d.client.GetDomainNameservers(ctx, config.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading nameservers",
			fmt.Sprintf("Could not read nameservers for %s: %s", config.Domain.ValueString(), err))
		return
	}

	value, diagnostics := types.SetValueFrom(ctx, types.StringType, nameservers)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.Nameservers = value
	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}

// ---------------------------------------------------- easydns_glue_records

type GlueRecordsDataSource struct{ client *Client }

type GlueRecordsDataSourceModel struct {
	Domain types.String          `tfsdk:"domain"`
	Glue   []GlueRecordModelItem `tfsdk:"glue_records"`
}

type GlueRecordModelItem struct {
	Host types.String `tfsdk:"host"`
	IPv4 types.String `tfsdk:"ipv4"`
	IPv6 types.String `tfsdk:"ipv6"`
}

func NewGlueRecordsDataSource() datasource.DataSource { return &GlueRecordsDataSource{} }

func (d *GlueRecordsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_glue_records"
}

func (d *GlueRecordsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches every registry glue record a domain provides, sorted by host.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				Description: "The domain that provides the glue.",
				Required:    true,
				Validators:  []validator.String{DomainNameValidator()},
			},
			"glue_records": schema.ListNestedAttribute{
				Description: "Glue records, sorted by host.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"host": schema.StringAttribute{Description: "The nameserver hostname.", Computed: true},
					"ipv4": schema.StringAttribute{Description: "Published IPv4 address, or null.", Computed: true},
					"ipv6": schema.StringAttribute{Description: "Published IPv6 address, or null.", Computed: true},
				}},
			},
		},
	}
}

func (d *GlueRecordsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *GlueRecordsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config GlueRecordsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	records, err := d.client.ListGlueRecords(ctx, config.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading glue records",
			fmt.Sprintf("Could not read glue records for %s: %s", config.Domain.ValueString(), err))
		return
	}

	config.Glue = make([]GlueRecordModelItem, len(records))
	for index, record := range records {
		config.Glue[index] = GlueRecordModelItem{
			Host: types.StringValue(record.Host),
			IPv4: optionalString(record.IPv4),
			IPv6: optionalString(record.IPv6),
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
