package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource                   = &MailmapsDataSource{}
	_ datasource.DataSource                   = &CurrentUserDataSource{}
	_ datasource.DataSource                   = &ServiceDataSource{}
	_ datasource.DataSource                   = &SubscriptionServiceDataSource{}
	_ datasource.DataSource                   = &DomainPricingDataSource{}
	_ datasource.DataSourceWithValidateConfig = &DomainPricingDataSource{}
)

// -------------------------------------------------------- easydns_mailmaps

type MailmapsDataSource struct{ client *Client }

type MailmapsDataSourceModel struct {
	Domain   types.String       `tfsdk:"domain"`
	Mailmaps []MailmapListModel `tfsdk:"mailmaps"`
}

type MailmapListModel struct {
	ID           types.String `tfsdk:"id"`
	Alias        types.String `tfsdk:"alias"`
	Host         types.String `tfsdk:"host"`
	Email        types.String `tfsdk:"email"`
	Destinations types.Set    `tfsdk:"destinations"`
	Active       types.Bool   `tfsdk:"active"`
	LastModified types.String `tfsdk:"last_modified"`
}

func NewMailmapsDataSource() datasource.DataSource { return &MailmapsDataSource{} }

func (d *MailmapsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mailmaps"
}

func (d *MailmapsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists every EasyDNS mail-forwarding map for a domain in stable numeric-ID order.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{Description: "Domain whose mailmaps are listed.", Required: true, Validators: []validator.String{DomainNameValidator()}},
			"mailmaps": schema.ListNestedAttribute{
				Description: "Mailmaps returned for the domain.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":            schema.StringAttribute{Description: "Immutable numeric mailmap ID.", Computed: true},
					"alias":         schema.StringAttribute{Description: "Source-address local part.", Computed: true},
					"host":          schema.StringAttribute{Description: "Relative mail host; @ means the domain apex.", Computed: true},
					"email":         schema.StringAttribute{Description: "Fully-qualified source address.", Computed: true},
					"destinations":  schema.SetAttribute{Description: "Forwarding destinations.", Computed: true, ElementType: types.StringType},
					"active":        schema.BoolAttribute{Description: "Whether forwarding is active.", Computed: true},
					"last_modified": schema.StringAttribute{Description: "Timestamp last reported by EasyDNS.", Computed: true},
				}},
			},
		},
	}
}

func (d *MailmapsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *MailmapsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config MailmapsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	mailmaps, err := d.client.ListMailmaps(ctx, config.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing mailmaps", fmt.Sprintf("Could not list mailmaps for %s: %s", config.Domain.ValueString(), err))
		return
	}
	config.Mailmaps = make([]MailmapListModel, len(mailmaps))
	for index, mailmap := range mailmaps {
		destinations, diagnostics := types.SetValueFrom(ctx, types.StringType, mailmap.Destinations)
		resp.Diagnostics.Append(diagnostics...)
		config.Mailmaps[index] = MailmapListModel{
			ID: types.StringValue(mailmap.ID), Alias: types.StringValue(mailmap.Alias), Host: types.StringValue(mailmap.Host),
			Email: types.StringValue(mailmap.Email()), Destinations: destinations, Active: types.BoolValue(mailmap.Active),
			LastModified: types.StringValue(mailmap.LastModified),
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}

// ---------------------------------------------------- easydns_current_user

type CurrentUserDataSource struct{ client *Client }

type currentUserState struct {
	ID           types.String `tfsdk:"id"`
	User         types.String `tfsdk:"user"`
	FirstName    types.String `tfsdk:"first_name"`
	LastName     types.String `tfsdk:"last_name"`
	Organization types.String `tfsdk:"organization"`
	Address1     types.String `tfsdk:"address1"`
	Address2     types.String `tfsdk:"address2"`
	Address3     types.String `tfsdk:"address3"`
	City         types.String `tfsdk:"city"`
	State        types.String `tfsdk:"state"`
	Country      types.String `tfsdk:"country"`
	PostalCode   types.String `tfsdk:"postal_code"`
	Currency     types.String `tfsdk:"currency"`
	Phone        types.String `tfsdk:"phone"`
	Cellphone    types.String `tfsdk:"cellphone"`
	Fax          types.String `tfsdk:"fax"`
	Email        types.String `tfsdk:"email"`
	Email2       types.String `tfsdk:"email2"`
	NoticesEmail types.String `tfsdk:"notices_email"`
	PublicEmail  types.String `tfsdk:"public_email"`
	AlertsEmail  types.String `tfsdk:"alerts_email"`
	URL          types.String `tfsdk:"url"`
	OptOut       types.Int64  `tfsdk:"opt_out"`
	Beta         types.Int64  `tfsdk:"beta"`
}

func NewCurrentUserDataSource() datasource.DataSource { return &CurrentUserDataSource{} }

func (d *CurrentUserDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_current_user"
}

func (d *CurrentUserDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	pii := func(description string) schema.StringAttribute {
		return schema.StringAttribute{Description: description, Computed: true, Sensitive: true}
	}
	resp.Schema = schema.Schema{
		Description: "Reads metadata for the authenticated EasyDNS account. Identity, address, telephone, email, and URL fields are sensitive because they contain account PII.",
		Attributes: map[string]schema.Attribute{
			"id": pii("Authenticated EasyDNS username, used as the data source ID."), "user": pii("Authenticated EasyDNS username."),
			"first_name": pii("Account contact first name."), "last_name": pii("Account contact last name."), "organization": pii("Account organization."),
			"address1": pii("First address line."), "address2": pii("Second address line."), "address3": pii("Third address line."),
			"city": pii("Account city."), "state": pii("Account state or province."), "country": pii("Two-letter account country code."),
			"postal_code": pii("Account postal code."), "phone": pii("Account telephone number."), "cellphone": pii("Account cellular number."),
			"fax": pii("Account fax number."), "email": pii("Primary contact email."), "email2": pii("Secondary contact email."),
			"notices_email": pii("Generic-notice destination."), "public_email": pii("Public contact destination."),
			"alerts_email": pii("Alert destination."), "url": pii("URL associated with the account."),
			"currency": schema.StringAttribute{Description: "Default account currency.", Computed: true},
			"opt_out":  schema.Int64Attribute{Description: "Non-essential communication opt-out flag.", Computed: true},
			"beta":     schema.Int64Attribute{Description: "Beta-feature access level.", Computed: true},
		},
	}
}

func (d *CurrentUserDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *CurrentUserDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	user, err := d.client.GetCurrentUser(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading current user", fmt.Sprintf("Could not read the authenticated account: %s", err))
		return
	}
	state := currentUserState{
		ID: types.StringValue(user.Username), User: types.StringValue(user.Username), FirstName: types.StringValue(user.FirstName), LastName: types.StringValue(user.LastName),
		Organization: types.StringValue(user.Organization), Address1: types.StringValue(user.Address1), Address2: types.StringValue(user.Address2), Address3: types.StringValue(user.Address3),
		City: types.StringValue(user.City), State: types.StringValue(user.State), Country: types.StringValue(user.Country), PostalCode: types.StringValue(user.PostalCode),
		Currency: types.StringValue(user.Currency), Phone: types.StringValue(user.Phone), Cellphone: types.StringValue(user.Cellphone), Fax: types.StringValue(user.Fax),
		Email: types.StringValue(user.Email), Email2: types.StringValue(user.Email2), NoticesEmail: types.StringValue(user.NoticesEmail),
		PublicEmail: types.StringValue(user.PublicEmail), AlertsEmail: types.StringValue(user.AlertsEmail), URL: types.StringValue(user.URL),
		OptOut: types.Int64Value(user.OptOut), Beta: types.Int64Value(user.Beta),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// --------------------------------------------------------- easydns_service

type ServiceDataSource struct{ client *Client }

type ServiceDataSourceModel struct {
	ServiceID   types.Int64  `tfsdk:"service_id"`
	Name        types.String `tfsdk:"name"`
	Period      types.Int64  `tfsdk:"period"`
	Enterprise  types.Bool   `tfsdk:"enterprise"`
	Description types.String `tfsdk:"description"`
}

func NewServiceDataSource() datasource.DataSource { return &ServiceDataSource{} }

func (d *ServiceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service"
}

func (d *ServiceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Reads one EasyDNS service description.", Attributes: map[string]schema.Attribute{
		"service_id": schema.Int64Attribute{Description: "Positive EasyDNS service ID.", Required: true, Validators: []validator.Int64{PositiveValidator("service_id")}},
		"name":       schema.StringAttribute{Description: "Service name.", Computed: true}, "period": schema.Int64Attribute{Description: "Service term, usually in months.", Computed: true},
		"enterprise": schema.BoolAttribute{Description: "Whether this is an enterprise service.", Computed: true}, "description": schema.StringAttribute{Description: "Service description.", Computed: true},
	}}
}

func (d *ServiceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *ServiceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config ServiceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	service, err := d.client.GetServiceDescription(ctx, config.ServiceID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Error reading service", fmt.Sprintf("Could not read service %d: %s", config.ServiceID.ValueInt64(), err))
		return
	}
	config.ServiceID = types.Int64Value(service.ServiceID)
	config.Name = types.StringValue(service.Name)
	config.Period = types.Int64Value(service.Period)
	config.Enterprise = types.BoolValue(service.Enterprise)
	config.Description = types.StringValue(service.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}

// -------------------------------------------- easydns_subscription_service

type SubscriptionServiceDataSource struct{ client *Client }

type SubscriptionServiceDataSourceModel struct {
	SubscriptionID types.Int64  `tfsdk:"subscription_id"`
	ServiceID      types.Int64  `tfsdk:"service_id"`
	Name           types.String `tfsdk:"name"`
	Period         types.Int64  `tfsdk:"period"`
	Enterprise     types.Bool   `tfsdk:"enterprise"`
	Description    types.String `tfsdk:"description"`
	Size           types.Int64  `tfsdk:"size"`
}

func NewSubscriptionServiceDataSource() datasource.DataSource {
	return &SubscriptionServiceDataSource{}
}
func (d *SubscriptionServiceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subscription_service"
}
func (d *SubscriptionServiceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Reads the EasyDNS service attached to a subscription block.", Attributes: map[string]schema.Attribute{
		"subscription_id": schema.Int64Attribute{Description: "Positive EasyDNS subscription block ID.", Required: true, Validators: []validator.Int64{PositiveValidator("subscription_id")}},
		"service_id":      schema.Int64Attribute{Description: "Service ID supplied by the subscription.", Computed: true}, "name": schema.StringAttribute{Description: "Service name.", Computed: true},
		"period": schema.Int64Attribute{Description: "Subscription term, usually in months.", Computed: true}, "enterprise": schema.BoolAttribute{Description: "Whether this is an enterprise service.", Computed: true},
		"description": schema.StringAttribute{Description: "Service description.", Computed: true}, "size": schema.Int64Attribute{Description: "Number of domains supported by the block.", Computed: true},
	}}
}
func (d *SubscriptionServiceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}
func (d *SubscriptionServiceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config SubscriptionServiceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	service, err := d.client.GetSubscriptionServiceDescription(ctx, config.SubscriptionID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Error reading subscription service", fmt.Sprintf("Could not read subscription %d: %s", config.SubscriptionID.ValueInt64(), err))
		return
	}
	config.SubscriptionID = types.Int64Value(service.SubscriptionID)
	config.ServiceID = types.Int64Value(service.ServiceID)
	config.Name = types.StringValue(service.Name)
	config.Period = types.Int64Value(service.Period)
	config.Enterprise = types.BoolValue(service.Enterprise)
	config.Description = types.StringValue(service.Description)
	config.Size = types.Int64Value(service.Size)
	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}

// -------------------------------------------------- easydns_domain_pricing

type DomainPricingDataSource struct{ client *Client }

type DomainPricingDataSourceModel struct {
	Domain    types.String         `tfsdk:"domain"`
	Service   types.String         `tfsdk:"service"`
	MinTerm   types.Int64          `tfsdk:"min_term"`
	MaxTerm   types.Int64          `tfsdk:"max_term"`
	Available types.Bool           `tfsdk:"available"`
	TLD       types.String         `tfsdk:"tld"`
	Services  []PricedServiceModel `tfsdk:"services"`
}

type PricedServiceModel struct {
	ID              types.Int64  `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Code            types.String `tfsdk:"code"`
	Currency        types.String `tfsdk:"currency"`
	Price           types.String `tfsdk:"price"`
	Premium         types.Bool   `tfsdk:"premium"`
	PricePeriod     types.Int64  `tfsdk:"price_period"`
	PricePeriodName types.String `tfsdk:"price_period_name"`
	Tax1            types.String `tfsdk:"tax1"`
	Tax2            types.String `tfsdk:"tax2"`
	Tax3            types.String `tfsdk:"tax3"`
}

func NewDomainPricingDataSource() datasource.DataSource { return &DomainPricingDataSource{} }
func (d *DomainPricingDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain_pricing"
}
func (d *DomainPricingDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Reads domain availability and account-specific service pricing. Prices and taxes are strings so decimal amounts round-trip without binary floating-point changes.", Attributes: map[string]schema.Attribute{
		"domain":    schema.StringAttribute{Description: "Domain to check.", Required: true, Validators: []validator.String{DomainNameValidator()}},
		"service":   schema.StringAttribute{Description: "Optional service-code filter.", Optional: true, Validators: []validator.String{DomainServiceValidator()}},
		"min_term":  schema.Int64Attribute{Description: "Optional minimum supported term.", Optional: true, Validators: []validator.Int64{PositiveValidator("min_term")}},
		"max_term":  schema.Int64Attribute{Description: "Optional maximum supported term.", Optional: true, Validators: []validator.Int64{PositiveValidator("max_term")}},
		"available": schema.BoolAttribute{Description: "Whether the domain is available for registration.", Computed: true}, "tld": schema.StringAttribute{Description: "Domain top-level label.", Computed: true},
		"services": schema.ListNestedAttribute{Description: "Available services sorted by service ID and price period.", Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{Description: "Service ID.", Computed: true}, "name": schema.StringAttribute{Description: "Service name.", Computed: true}, "code": schema.StringAttribute{Description: "Service code.", Computed: true},
			"currency": schema.StringAttribute{Description: "Price currency.", Computed: true}, "price": schema.StringAttribute{Description: "Exact service price for the period.", Computed: true}, "premium": schema.BoolAttribute{Description: "Whether premium domain pricing applies.", Computed: true},
			"price_period": schema.Int64Attribute{Description: "Number of pricing periods.", Computed: true}, "price_period_name": schema.StringAttribute{Description: "Period unit, such as year or month.", Computed: true},
			"tax1": schema.StringAttribute{Description: "Exact regional tax amount 1.", Computed: true}, "tax2": schema.StringAttribute{Description: "Exact regional tax amount 2.", Computed: true}, "tax3": schema.StringAttribute{Description: "Exact regional tax amount 3.", Computed: true},
		}}},
	}}
}

func (d *DomainPricingDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}
func (d *DomainPricingDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var config DomainPricingDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !config.MinTerm.IsNull() && !config.MinTerm.IsUnknown() && !config.MaxTerm.IsNull() && !config.MaxTerm.IsUnknown() && config.MinTerm.ValueInt64() > config.MaxTerm.ValueInt64() {
		resp.Diagnostics.AddAttributeError(path.Root("min_term"), "Invalid Pricing Term Range", "min_term cannot be greater than max_term.")
	}
}
func (d *DomainPricingDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config DomainPricingDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	request := DomainPricingRequest{Domain: config.Domain.ValueString(), Service: config.Service.ValueString(), MinTerm: config.MinTerm.ValueInt64(), MaxTerm: config.MaxTerm.ValueInt64()}
	pricing, err := d.client.GetDomainPricing(ctx, request)
	if err != nil {
		resp.Diagnostics.AddError("Error reading domain pricing", fmt.Sprintf("Could not price %s: %s", config.Domain.ValueString(), err))
		return
	}
	config.Available = types.BoolValue(pricing.Available)
	config.TLD = types.StringValue(pricing.TLD)
	config.Services = make([]PricedServiceModel, len(pricing.Services))
	for index, service := range pricing.Services {
		config.Services[index] = PricedServiceModel{ID: types.Int64Value(service.ID), Name: types.StringValue(service.Name), Code: types.StringValue(service.Code), Currency: types.StringValue(service.Currency), Price: types.StringValue(service.Price), Premium: types.BoolValue(service.Premium), PricePeriod: types.Int64Value(service.PricePeriod), PricePeriodName: types.StringValue(service.PricePeriodName), Tax1: types.StringValue(service.Tax1), Tax2: types.StringValue(service.Tax2), Tax3: types.StringValue(service.Tax3)}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
