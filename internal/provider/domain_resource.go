package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &DomainResource{}
	_ resource.ResourceWithImportState    = &DomainResource{}
	_ resource.ResourceWithValidateConfig = &DomainResource{}
	_ resource.ResourceWithModifyPlan     = &DomainResource{}
)

// DomainResource manages a domain on the EasyDNS system. Registration and
// deletion are both guarded; see ADR-0003.
type DomainResource struct {
	client *Client
}

type DomainResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Domain             types.String `tfsdk:"domain"`
	Service            types.String `tfsdk:"service"`
	Term               types.Int64  `tfsdk:"term"`
	Currency           types.String `tfsdk:"currency"`
	DNSOnly            types.Bool   `tfsdk:"dns_only"`
	Premium            types.Bool   `tfsdk:"premium"`
	PremiumPrice       types.String `tfsdk:"premium_price"`
	MaxPremiumPrice    types.String `tfsdk:"max_premium_price"`
	Nameservers        types.Set    `tfsdk:"nameservers"`
	DomainGroup        types.String `tfsdk:"domain_group"`
	PrimaryNS          types.String `tfsdk:"primary_ns"`
	Contacts           types.Object `tfsdk:"contacts"`
	Extra              types.Map    `tfsdk:"extra"`
	DeletionProtection types.Bool   `tfsdk:"deletion_protection"`

	Exists         types.Bool   `tfsdk:"exists"`
	OnSystem       types.Bool   `tfsdk:"on_system"`
	Expiry         types.String `tfsdk:"expiry"`
	NextDue        types.String `tfsdk:"next_due"`
	ClonedTo       types.String `tfsdk:"cloned_to"`
	ServiceID      types.Int64  `tfsdk:"service_id"`
	SubscriptionID types.Int64  `tfsdk:"subscription_id"`
}

func NewDomainResource() resource.Resource {
	return &DomainResource{}
}

func (r *DomainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *DomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Adds a domain to EasyDNS for DNS-only service or, behind an explicit opt-in, registers it at the registry.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The domain identity, equal to the normalized domain name.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"domain": schema.StringAttribute{
				Description: "The domain name, for example 'example.com'.",
				Required:    true,
				Validators:  []validator.String{DomainNameValidator()},
			},
			"service": schema.StringAttribute{
				Description: "EasyDNS service level: 'lite', 'dns', 'pro', or 'enterprise'.",
				Required:    true,
				Validators:  []validator.String{DomainServiceValidator()},
			},
			"term": schema.Int64Attribute{
				Description: "Service term in years, 1 through 10. For a registration this is also the registration term.",
				Required:    true,
				Validators:  []validator.Int64{DomainTermValidator()},
			},
			"currency": schema.StringAttribute{
				Description: "Billing currency for the creation invoice: 'CAD' or 'USD'.",
				Required:    true,
				Validators:  []validator.String{DomainCurrencyValidator()},
			},
			"dns_only": schema.BoolAttribute{
				Description: "When true the domain is added for DNS service only and is never registered at a registry. Defaults to true. This is not the same as free: EasyDNS invoices the DNS service itself, so creating any domain draws on the account balance.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"premium": schema.BoolAttribute{
				Description: "Acknowledges that the registry prices this domain as premium. Requires max_premium_price.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"premium_price": schema.StringAttribute{
				Description: "The verified premium price sent to EasyDNS, taken from the pricing endpoint. Must not exceed max_premium_price.",
				Optional:    true,
			},
			"max_premium_price": schema.StringAttribute{
				Description: "The highest premium price this configuration accepts. A premium_price above it fails during planning rather than at the registry.",
				Optional:    true,
			},
			"nameservers": schema.SetAttribute{
				Description: "Optional delegation to apply at creation, up to six hosts. Ongoing delegation belongs to easydns_domain_nameservers.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"domain_group": schema.StringAttribute{
				Description: "An existing domain group to assign the new domain to.",
				Optional:    true,
			},
			"primary_ns": schema.StringAttribute{
				Description: "Primary nameserver addresses for a secondary domain, separated by semicolons. Setting this makes the domain secondary and starts a zone transfer.",
				Optional:    true,
			},
			"contacts": schema.SingleNestedAttribute{
				Description: "Registrant contacts. Required to register a domain and rejected for a DNS-only domain. These fields are personal data and are stored in Terraform state.",
				Optional:    true,
				Sensitive:   true,
				Attributes: map[string]schema.Attribute{
					"owner":   contactSchema("Registrant contact. Required for a registration."),
					"admin":   contactSchema("Administrative contact."),
					"tech":    contactSchema("Technical contact."),
					"billing": contactSchema("Billing contact."),
				},
			},
			"extra": schema.MapAttribute{
				Description: "Documented TLD-specific registration fields, such as registrant_type for .FR or app_purpose for .US. Values are passed through unchanged and may contain personal data.",
				Optional:    true,
				Sensitive:   true,
				ElementType: types.StringType,
			},
			"deletion_protection": schema.BoolAttribute{
				Description: "Refuses `terraform destroy` for this domain while true. Defaults to true. Set it to false and apply before destroying.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},

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

func contactSchema(description string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Description: description,
		Optional:    true,
		Attributes: map[string]schema.Attribute{
			"first_name":  schema.StringAttribute{Description: "Given name.", Required: true},
			"last_name":   schema.StringAttribute{Description: "Family name.", Required: true},
			"org_name":    schema.StringAttribute{Description: "Organization.", Optional: true},
			"address1":    schema.StringAttribute{Description: "Street address.", Required: true},
			"address2":    schema.StringAttribute{Description: "Second address line.", Optional: true},
			"city":        schema.StringAttribute{Description: "City.", Required: true},
			"state":       schema.StringAttribute{Description: "Province or state.", Required: true},
			"country":     schema.StringAttribute{Description: "Two-letter ISO 3166-1 country code.", Required: true},
			"postal_code": schema.StringAttribute{Description: "Postal or ZIP code.", Required: true},
			"phone":       schema.StringAttribute{Description: "Phone number in E.164 form, for example '+1.4165550100'.", Required: true},
			"email":       schema.StringAttribute{Description: "Email address.", Required: true},
			"language":    schema.StringAttribute{Description: "Contact language, 'en' or 'fr'. Required for .CA registrations.", Optional: true},
			"cpr":         schema.StringAttribute{Description: "Canadian Presence Requirement type. .CA registrations only.", Optional: true},
		},
	}
}

func (r *DomainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *Client, got: %T.", req.ProviderData))
		return
	}
	r.client = client
}

func (r *DomainResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config DomainResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dnsOnly := config.DNSOnly.IsNull() || config.DNSOnly.ValueBool()
	// Observed against the sandbox: EasyDNS refuses this combination with
	// "DNS only domain creation is not supported for the lite service level".
	// Catching it here turns an apply-time API error into a planning error.
	if dnsOnly && config.Service.ValueString() == "lite" {
		resp.Diagnostics.AddAttributeError(path.Root("service"), "DNS-Only Is Not Available For The Lite Service Level",
			"EasyDNS does not support dns_only = true with service = \"lite\". Use \"dns\", \"pro\", or \"enterprise\" for a DNS-only domain, or set dns_only = false to register the domain.")
	}
	if dnsOnly {
		if !config.Contacts.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("contacts"), "Contacts Are Not Used For A DNS-Only Domain",
				"contacts apply only to a registration. Remove contacts, or set dns_only = false to register the domain.")
		}
		if !config.Extra.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("extra"), "TLD Fields Are Not Used For A DNS-Only Domain",
				"extra carries TLD registration fields. Remove it, or set dns_only = false to register the domain.")
		}
		if !config.Premium.IsNull() && config.Premium.ValueBool() {
			resp.Diagnostics.AddAttributeError(path.Root("premium"), "Premium Pricing Does Not Apply To A DNS-Only Domain",
				"A DNS-only domain is not registered, so it has no registry premium price.")
		}
		return
	}

	if config.Contacts.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("contacts"), "Registration Requires Contacts",
			"Registering a domain requires at least an owner contact. Set dns_only = true to add the domain for DNS service only.")
	}

	premium := !config.Premium.IsNull() && config.Premium.ValueBool()
	if premium && config.MaxPremiumPrice.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("max_premium_price"), "Premium Registration Requires A Price Ceiling",
			"Set max_premium_price to the highest premium price this configuration accepts. See ADR-0003.")
	}
	if premium && config.PremiumPrice.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("premium_price"), "Premium Registration Requires A Verified Price",
			"Set premium_price to the value returned by the EasyDNS pricing endpoint.")
	}
	if !premium && !config.PremiumPrice.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("premium_price"), "Premium Price Without The Premium Opt-In",
			"Set premium = true to acknowledge the registry's premium pricing, or remove premium_price.")
	}
	if premium && !config.PremiumPrice.IsNull() && !config.MaxPremiumPrice.IsNull() {
		if err := assertPremiumPriceWithinCeiling(config.PremiumPrice.ValueString(), config.MaxPremiumPrice.ValueString()); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("premium_price"), "Premium Price Exceeds The Accepted Maximum", err.Error())
		}
	}
}

// ModifyPlan enforces two safety rules from ADR-0003: registration requires
// the provider opt-in, and an immutable field never triggers an automatic
// destroy-and-recreate of a real domain.
func (r *DomainResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		// Destroy plan. Deletion protection is enforced in Delete, where the
		// state value is authoritative.
		return
	}

	var plan DomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	registering := !plan.DNSOnly.IsNull() && !plan.DNSOnly.ValueBool()
	if registering && r.client != nil && !r.client.DomainRegistrationEnabled() {
		resp.Diagnostics.AddError(
			"Domain Registration Is Disabled",
			"This configuration registers "+plan.Domain.ValueString()+" at a registry, which is billable and cannot be undone by Terraform. "+
				"Set enable_domain_registration = true on the easydns provider to allow it, or set dns_only = true.",
		)
		return
	}

	if req.State.Raw.IsNull() {
		return
	}

	var state DomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// EasyDNS has no update operation for these; replacing them would mean
	// deleting and re-adding a real domain, so a change is refused instead.
	immutable := []struct {
		name    string
		planned string
		current string
	}{
		{"domain", plan.Domain.ValueString(), state.Domain.ValueString()},
		{"service", plan.Service.ValueString(), state.Service.ValueString()},
		{"currency", plan.Currency.ValueString(), state.Currency.ValueString()},
		{"dns_only", plan.DNSOnly.String(), state.DNSOnly.String()},
	}
	for _, field := range immutable {
		if field.planned != field.current {
			resp.Diagnostics.AddAttributeError(
				path.Root(field.name),
				"Immutable Domain Attribute Changed",
				fmt.Sprintf("%s cannot be changed after the domain is created (%s to %s). EasyDNS has no update operation for it, and Terraform will not delete and re-add a real domain to apply the change. "+
					"Restore the previous value, or remove the resource from state with `terraform state rm` and manage the new configuration deliberately.",
					field.name, field.current, field.planned),
			)
		}
	}
	if plan.Term.ValueInt64() != state.Term.ValueInt64() {
		resp.Diagnostics.AddAttributeError(
			path.Root("term"),
			"Immutable Domain Attribute Changed",
			fmt.Sprintf("term cannot be changed after creation (%d to %d). Renewal policy is managed by easydns_domain_registration_settings.",
				state.Term.ValueInt64(), plan.Term.ValueInt64()),
		)
	}
}

func (r *DomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request, diagnostics := buildDomainRequest(ctx, plan)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.CreateDomain(ctx, request); err != nil {
		if errors.Is(err, ErrDomainRegistrationDisabled) {
			resp.Diagnostics.AddError("Domain Registration Is Disabled",
				"Set enable_domain_registration = true on the easydns provider to register domains, or set dns_only = true.")
			return
		}
		resp.Diagnostics.AddError("Error creating domain", fmt.Sprintf("Could not create %s: %s", request.Domain, err))
		return
	}

	domain, err := r.client.GetDomain(ctx, request.Domain)
	if err != nil {
		resp.Diagnostics.AddError("Error reading the created domain",
			fmt.Sprintf("%s was created but could not be read back: %s", request.Domain, err))
		return
	}
	applyDomainToResourceModel(&plan, domain)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *DomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain, err := r.client.GetDomain(ctx, state.Domain.ValueString())
	if err != nil {
		if IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading domain", fmt.Sprintf("Could not read %s: %s", state.Domain.ValueString(), err))
		return
	}
	applyDomainToResourceModel(&state, domain)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update only records Terraform-side changes. Every remote attribute of a
// domain is immutable and is refused during planning.
func (r *DomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain, err := r.client.GetDomain(ctx, plan.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading domain", fmt.Sprintf("Could not read %s: %s", plan.Domain.ValueString(), err))
		return
	}
	applyDomainToResourceModel(&plan, domain)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *DomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.DeletionProtection.IsNull() || state.DeletionProtection.ValueBool() {
		resp.Diagnostics.AddError(
			"Domain Deletion Is Protected",
			fmt.Sprintf("%s has deletion_protection enabled. Deleting a domain removes DNS and registrar service and cannot be undone by Terraform. "+
				"Set deletion_protection = false and apply that change before destroying. Terraform will not silently drop the domain from state instead.",
				state.Domain.ValueString()),
		)
		return
	}

	if err := r.client.DeleteDomain(ctx, state.Domain.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting domain", fmt.Sprintf("Could not delete %s: %s", state.Domain.ValueString(), err))
	}
}

func (r *DomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	domain, err := NormalizeDomain(strings.TrimSpace(req.ID))
	if err != nil || strings.TrimSpace(req.ID) != req.ID {
		resp.Diagnostics.AddError("Invalid import ID", "Import ID must be a domain name such as 'example.com'.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), domain)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), domain)...)
	// Imported domains keep the safe default rather than inheriting an
	// unprotected value from an empty state.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("deletion_protection"), true)...)
}

func applyDomainToResourceModel(model *DomainResourceModel, domain *Domain) {
	model.ID = types.StringValue(domain.Domain)
	model.Domain = types.StringValue(domain.Domain)
	model.Exists = types.BoolValue(domain.Exists)
	model.OnSystem = types.BoolValue(domain.OnSystem)
	model.Expiry = types.StringValue(domain.Expiry)
	model.NextDue = types.StringValue(domain.NextDue)
	model.ClonedTo = types.StringValue(domain.ClonedTo)
	model.ServiceID = types.Int64Value(domain.Service)
	model.SubscriptionID = types.Int64Value(domain.SubscriptionID)
}
