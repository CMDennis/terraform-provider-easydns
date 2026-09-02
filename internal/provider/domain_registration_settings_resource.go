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
	_ resource.Resource                = &DomainRegistrationSettingsResource{}
	_ resource.ResourceWithImportState = &DomainRegistrationSettingsResource{}
)

// DomainRegistrationSettingsResource manages the registrar lock and renewal
// policy of an existing domain. It adopts the domain rather than creating it,
// and removing it from Terraform leaves the remote policy untouched.
type DomainRegistrationSettingsResource struct {
	client *Client
}

type DomainRegistrationSettingsModel struct {
	ID      types.String `tfsdk:"id"`
	Domain  types.String `tfsdk:"domain"`
	Reglock types.Bool   `tfsdk:"reglock"`
	Renewal types.String `tfsdk:"renewal"`

	AutoRenew       types.Bool   `tfsdk:"auto_renew"`
	AutoRenewCardID types.String `tfsdk:"auto_renew_card_id"`
	LetExpire       types.Bool   `tfsdk:"let_expire"`
	LetExpireFailed types.Bool   `tfsdk:"let_expire_failed"`
	Expiry          types.String `tfsdk:"expiry"`
	LocalRegistrar  types.Bool   `tfsdk:"local_registrar"`
	SupportsReglock types.Bool   `tfsdk:"supports_reglock"`
}

func NewDomainRegistrationSettingsResource() resource.Resource {
	return &DomainRegistrationSettingsResource{}
}

func (r *DomainRegistrationSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain_registration_settings"
}

func (r *DomainRegistrationSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the registrar lock and renewal policy of a domain that already exists on EasyDNS. Destroying this resource stops managing the policy and does not change it remotely.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The managed domain.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"domain": schema.StringAttribute{
				Description:   "The domain whose registration policy is managed.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{DomainNameValidator()},
			},
			"reglock": schema.BoolAttribute{
				Description: "Registrar lock. Ignored by EasyDNS on a TLD that reports supports_reglock = false.",
				Required:    true,
			},
			"renewal": schema.StringAttribute{
				Description: "Renewal action: 'remind', 'renew', or 'expire'.",
				Required:    true,
				Validators:  []validator.String{RenewalActionValidator()},
			},

			"auto_renew":         schema.BoolAttribute{Description: "Whether automated renewal payment is configured.", Computed: true},
			"auto_renew_card_id": schema.StringAttribute{Description: "Card identifier used for automated renewal, when configured.", Computed: true},
			"let_expire":         schema.BoolAttribute{Description: "Whether the domain is set to expire at the registry.", Computed: true},
			"let_expire_failed":  schema.BoolAttribute{Description: "Whether EasyDNS failed to read let_expire from the registry.", Computed: true},
			"expiry":             schema.StringAttribute{Description: "Expiry date, or the date service is next due.", Computed: true},
			"local_registrar":    schema.BoolAttribute{Description: "Whether EasyDNS is the registrar of record.", Computed: true},
			"supports_reglock":   schema.BoolAttribute{Description: "Whether this TLD supports registrar locking.", Computed: true},
		},
	}
}

func (r *DomainRegistrationSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DomainRegistrationSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DomainRegistrationSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *DomainRegistrationSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DomainRegistrationSettingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	status, err := r.client.GetRegistrationStatus(ctx, state.Domain.ValueString())
	if err != nil {
		if IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading registration settings",
			fmt.Sprintf("Could not read registration settings for %s: %s", state.Domain.ValueString(), err))
		return
	}
	applyRegistrationStatusToModel(&state, status)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *DomainRegistrationSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DomainRegistrationSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Delete stops managing the policy. EasyDNS has no operation that clears a
// reglock or renewal setting, and silently changing one on destroy would be a
// surprising registrar side effect.
func (r *DomainRegistrationSettingsResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"Registration Policy Left Unchanged",
		"Terraform stopped managing this domain's registrar lock and renewal policy. The remote settings were not modified.",
	)
}

func (r *DomainRegistrationSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	domain, err := NormalizeDomain(strings.TrimSpace(req.ID))
	if err != nil || strings.TrimSpace(req.ID) != req.ID {
		resp.Diagnostics.AddError("Invalid import ID", "Import ID must be a domain name such as 'example.com'.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), domain)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), domain)...)
}

func (r *DomainRegistrationSettingsResource) apply(ctx context.Context, plan *DomainRegistrationSettingsModel, diagnostics *diag.Diagnostics) {
	renewal, err := ParseRenewalAction(plan.Renewal.ValueString())
	if err != nil {
		diagnostics.AddError("Invalid renewal action", err.Error())
		return
	}

	status, err := r.client.SetRegistrationSettings(ctx, RegistrationSettingsRequest{
		Domain:  plan.Domain.ValueString(),
		Reglock: plan.Reglock.ValueBool(),
		Renewal: renewal,
	})
	if err != nil {
		diagnostics.AddError("Error applying registration settings",
			fmt.Sprintf("Could not apply registration settings for %s: %s", plan.Domain.ValueString(), err))
		return
	}

	if plan.Reglock.ValueBool() && !status.SupportsReglock {
		diagnostics.AddWarning(
			"Registrar Lock Is Not Supported For This TLD",
			fmt.Sprintf("%s reports supports_reglock = false, so the requested reglock was not applied at the registry. The renewal policy was still applied.", status.Domain),
		)
	}
	applyRegistrationStatusToModel(plan, status)
}

func applyRegistrationStatusToModel(model *DomainRegistrationSettingsModel, status *RegistrationStatus) {
	model.ID = types.StringValue(status.Domain)
	model.Domain = types.StringValue(status.Domain)
	model.Reglock = types.BoolValue(status.Reglock)
	model.Renewal = types.StringValue(status.Renewal)
	model.AutoRenew = types.BoolValue(status.AutoRenew)
	model.AutoRenewCardID = types.StringValue(status.AutoRenewCardID)
	model.LetExpire = types.BoolValue(status.LetExpire)
	model.LetExpireFailed = types.BoolValue(status.LetExpireFailed)
	model.Expiry = types.StringValue(status.Expiry)
	model.LocalRegistrar = types.BoolValue(status.LocalRegistrar)
	model.SupportsReglock = types.BoolValue(status.SupportsReglock)
}
