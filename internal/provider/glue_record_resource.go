package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &GlueRecordResource{}
	_ resource.ResourceWithImportState    = &GlueRecordResource{}
	_ resource.ResourceWithValidateConfig = &GlueRecordResource{}
)

// GlueRecordResource manages a registry glue record for a nameserver host
// inside a domain.
type GlueRecordResource struct {
	client *Client
}

type GlueRecordModel struct {
	ID                 types.String `tfsdk:"id"`
	Domain             types.String `tfsdk:"domain"`
	Host               types.String `tfsdk:"host"`
	IPv4               types.String `tfsdk:"ipv4"`
	IPv6               types.String `tfsdk:"ipv6"`
	RegistryConfigured types.Bool   `tfsdk:"registry_configured"`
}

func NewGlueRecordResource() resource.Resource {
	return &GlueRecordResource{}
}

func (r *GlueRecordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_glue_record"
}

func (r *GlueRecordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a registry glue record, which publishes addresses for a nameserver hosted inside the domain it serves.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Composite identity in the form 'domain:host'.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"domain": schema.StringAttribute{
				Description:   "The domain that provides the glue.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{DomainNameValidator()},
			},
			"host": schema.StringAttribute{
				Description:   "The fully qualified nameserver hostname, for example 'ns1.example.com'.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{DomainNameValidator()},
			},
			"ipv4": schema.StringAttribute{
				Description: "IPv4 address published for the host. At least one of ipv4 or ipv6 is required.",
				Optional:    true,
				Validators:  []validator.String{IPv4Validator()},
			},
			"ipv6": schema.StringAttribute{
				Description: "IPv6 address published for the host. At least one of ipv4 or ipv6 is required.",
				Optional:    true,
				Validators:  []validator.String{IPv6Validator()},
			},
			"registry_configured": schema.BoolAttribute{
				Description: "Whether the registry reports the glue record as in place. Registry propagation can lag a successful write.",
				Computed:    true,
			},
		},
	}
}

func (r *GlueRecordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *GlueRecordResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config GlueRecordModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if config.IPv4.IsNull() && config.IPv6.IsNull() {
		resp.Diagnostics.AddError("Glue Record Requires An Address",
			"Set ipv4, ipv6, or both. A glue record with no address has nothing to publish.")
	}
	if config.Domain.IsUnknown() || config.Host.IsUnknown() || config.Domain.IsNull() || config.Host.IsNull() {
		return
	}
	domain, domainErr := NormalizeDomain(config.Domain.ValueString())
	host, hostErr := NormalizeDomain(config.Host.ValueString())
	if domainErr != nil || hostErr != nil {
		return
	}
	// Glue only exists for a host inside the domain that provides it.
	if host != domain && !strings.HasSuffix(host, "."+domain) {
		resp.Diagnostics.AddAttributeError(path.Root("host"), "Glue Host Is Outside Its Domain",
			fmt.Sprintf("%s is not inside %s. A registry glue record can only be created for a nameserver within the domain that provides it.", host, domain))
	}
}

func (r *GlueRecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan GlueRecordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	record, err := r.client.CreateGlueRecord(ctx, glueRequestFromModel(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating glue record",
			fmt.Sprintf("Could not create glue for %s: %s", plan.Host.ValueString(), err))
		return
	}
	r.applyGlueToModel(ctx, &plan, record, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *GlueRecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state GlueRecordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	record, err := r.client.GetGlueRecord(ctx, state.Domain.ValueString(), state.Host.ValueString())
	if err != nil {
		if IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading glue record",
			fmt.Sprintf("Could not read glue for %s: %s", state.Host.ValueString(), err))
		return
	}
	r.applyGlueToModel(ctx, &state, record, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *GlueRecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan GlueRecordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	record, err := r.client.UpdateGlueRecord(ctx, glueRequestFromModel(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating glue record",
			fmt.Sprintf("Could not update glue for %s: %s", plan.Host.ValueString(), err))
		return
	}
	r.applyGlueToModel(ctx, &plan, record, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *GlueRecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state GlueRecordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteGlueRecord(ctx, state.Domain.ValueString(), state.Host.ValueString()); err != nil {
		// The message states a likely cause rather than asserting one: EasyDNS
		// also returns a generic "contact support" failure for glue deletions
		// that have nothing to do with an outstanding delegation.
		resp.Diagnostics.AddError("Error deleting glue record",
			fmt.Sprintf("Could not delete glue for %s: %s\n\n"+
				"A registry commonly refuses this while a domain in the same TLD still delegates to the host, so check for remaining references first. "+
				"If none exist, the failure is on the EasyDNS side and the glue record is still present.",
				state.Host.ValueString(), err))
	}
}

func (r *GlueRecordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	domain, host, err := parseGlueImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), domain)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("host"), host)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), domain+":"+host)...)
}

func parseGlueImportID(value string) (string, string, error) {
	const expected = "import ID must be in the form 'domain:host', for example 'example.com:ns1.example.com'"
	if strings.TrimSpace(value) != value || strings.Count(value, ":") != 1 {
		return "", "", fmt.Errorf("%s", expected)
	}
	parts := strings.SplitN(value, ":", 2)
	domain, err := NormalizeDomain(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("%s", expected)
	}
	host, err := NormalizeDomain(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("%s", expected)
	}
	if host != domain && !strings.HasSuffix(host, "."+domain) {
		return "", "", fmt.Errorf("%s: host must be inside the domain", expected)
	}
	return domain, host, nil
}

func glueRequestFromModel(model GlueRecordModel) GlueRecordRequest {
	return GlueRecordRequest{
		Domain: model.Domain.ValueString(),
		Host:   model.Host.ValueString(),
		IPv4:   model.IPv4.ValueString(),
		IPv6:   model.IPv6.ValueString(),
	}
}

func (r *GlueRecordResource) applyGlueToModel(ctx context.Context, model *GlueRecordModel, record *GlueRecord, diagnostics *diag.Diagnostics) {
	model.ID = types.StringValue(record.Domain + ":" + record.Host)
	model.Domain = types.StringValue(record.Domain)
	model.Host = types.StringValue(record.Host)
	model.IPv4 = optionalString(record.IPv4)
	model.IPv6 = optionalString(record.IPv6)

	// Registry propagation is reported separately and is allowed to lag; a
	// failure to read it must not fail the whole apply.
	configured, err := r.client.CheckRegistryGlue(ctx, record.Domain, record.Host)
	if err != nil {
		diagnostics.AddWarning("Registry Glue Status Unavailable",
			fmt.Sprintf("The glue record for %s was written, but its registry status could not be read: %s", record.Host, err))
		model.RegistryConfigured = types.BoolValue(false)
		return
	}
	model.RegistryConfigured = types.BoolValue(configured)
}

// optionalString keeps an absent address null rather than an empty string, so
// it round-trips against an unset optional attribute.
func optionalString(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}
