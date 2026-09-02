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
	_ resource.Resource                = &DomainNameserversResource{}
	_ resource.ResourceWithImportState = &DomainNameserversResource{}
)

// DomainNameserversResource manages the complete delegation set of a domain.
// Delegation order carries no meaning, so nameservers are a set.
type DomainNameserversResource struct {
	client *Client
}

type DomainNameserversModel struct {
	ID          types.String `tfsdk:"id"`
	Domain      types.String `tfsdk:"domain"`
	Nameservers types.Set    `tfsdk:"nameservers"`
}

func NewDomainNameserversResource() resource.Resource {
	return &DomainNameserversResource{}
}

func (r *DomainNameserversResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain_nameservers"
}

func (r *DomainNameserversResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the complete set of nameservers delegated for a domain. Destroying this resource stops managing the delegation and leaves it in place.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The managed domain.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"domain": schema.StringAttribute{
				Description:   "The domain whose delegation is managed.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{DomainNameValidator()},
			},
			"nameservers": schema.SetAttribute{
				Description: "The complete delegation set. EasyDNS requires between 2 and 10 nameservers. Order is not significant.",
				Required:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *DomainNameserversResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DomainNameserversResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DomainNameserversModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.applyDelegation(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *DomainNameserversResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DomainNameserversModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	nameservers, err := r.client.GetDomainNameservers(ctx, state.Domain.ValueString())
	if err != nil {
		if IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading nameservers",
			fmt.Sprintf("Could not read nameservers for %s: %s", state.Domain.ValueString(), err))
		return
	}

	value, diagnostics := types.SetValueFrom(ctx, types.StringType, nameservers)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ID = types.StringValue(state.Domain.ValueString())
	state.Nameservers = value
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *DomainNameserversResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DomainNameserversModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.applyDelegation(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Delete stops managing the delegation. Clearing a domain's nameservers would
// take it off the internet, so destroy never writes to the registry.
func (r *DomainNameserversResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"Delegation Left Unchanged",
		"Terraform stopped managing this domain's nameservers. The delegation at EasyDNS was not modified.",
	)
}

func (r *DomainNameserversResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	domain, err := NormalizeDomain(strings.TrimSpace(req.ID))
	if err != nil || strings.TrimSpace(req.ID) != req.ID {
		resp.Diagnostics.AddError("Invalid import ID", "Import ID must be a domain name such as 'example.com'.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), domain)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), domain)...)
}

func (r *DomainNameserversResource) applyDelegation(ctx context.Context, plan *DomainNameserversModel, diagnostics *diag.Diagnostics) {
	var desired []string
	diagnostics.Append(plan.Nameservers.ElementsAs(ctx, &desired, false)...)
	if diagnostics.HasError() {
		return
	}

	applied, err := r.client.SetDomainNameservers(ctx, plan.Domain.ValueString(), desired)
	if err != nil {
		diagnostics.AddAttributeError(path.Root("nameservers"), "Error applying nameservers",
			fmt.Sprintf("Could not set nameservers for %s: %s", plan.Domain.ValueString(), err))
		return
	}

	value, setDiagnostics := types.SetValueFrom(ctx, types.StringType, applied)
	diagnostics.Append(setDiagnostics...)
	if diagnostics.HasError() {
		return
	}
	plan.ID = types.StringValue(plan.Domain.ValueString())
	plan.Nameservers = value
}
