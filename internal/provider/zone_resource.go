package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure ZoneResource satisfies various resource interfaces
var (
	_ resource.Resource                = &ZoneResource{}
	_ resource.ResourceWithImportState = &ZoneResource{}
)

// ZoneResource defines the resource implementation
type ZoneResource struct {
	client *Client
}

// ZoneResourceModel describes the resource data model
type ZoneResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Domain   types.String `tfsdk:"domain"`
	Exists   types.Bool   `tfsdk:"exists"`
	OnSystem types.Bool   `tfsdk:"on_system"`
	Expiry   types.String `tfsdk:"expiry"`
	NextDue  types.String `tfsdk:"next_due"`
	Service  types.String `tfsdk:"service"`
}

// NewZoneResource creates a new zone resource
func NewZoneResource() resource.Resource {
	return &ZoneResource{}
}

func (r *ZoneResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone"
}

func (r *ZoneResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:        "Manages a DNS zone in EasyDNS. This resource is import-only - zones must be created in the EasyDNS dashboard, then imported into Terraform.",
		DeprecationMessage: "easydns_zone is deprecated and will be removed in v2.0.0. Use easydns_domain, which can add a domain rather than only adopt one, and exposes the same fields plus cloned_to and subscription_id. See the migration guide in the provider documentation.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The zone identifier (same as domain).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"domain": schema.StringAttribute{
				Description: "The domain name of the zone.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"exists": schema.BoolAttribute{
				Description: "Whether the zone exists.",
				Computed:    true,
			},
			"on_system": schema.BoolAttribute{
				Description: "Whether the zone is active on the EasyDNS system.",
				Computed:    true,
			},
			"expiry": schema.StringAttribute{
				Description: "The expiry date of the domain registration, if applicable.",
				Computed:    true,
			},
			"next_due": schema.StringAttribute{
				Description: "The next billing due date.",
				Computed:    true,
			},
			"service": schema.StringAttribute{
				Description: "The service ID associated with the zone.",
				Computed:    true,
			},
		},
	}
}

func (r *ZoneResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *ZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ZoneResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if the zone already exists - if so, treat this as an implicit import
	domain := plan.Domain.ValueString()
	zone, err := r.client.GetZone(ctx, domain)
	if err != nil {
		resp.Diagnostics.AddError(
			"Zone Not Found",
			fmt.Sprintf("Zone '%s' does not exist in EasyDNS. This resource is import-only. Please create the zone in the EasyDNS dashboard first, then use 'terraform import easydns_zone.name %s' to import it.", domain, domain),
		)
		return
	}

	if !zone.Exists {
		resp.Diagnostics.AddError(
			"Zone Not Found",
			fmt.Sprintf("Zone '%s' does not exist in EasyDNS. Please create it in the EasyDNS dashboard first.", domain),
		)
		return
	}

	// Zone exists - populate state
	plan.ID = types.StringValue(zone.ID)
	plan.Exists = types.BoolValue(zone.Exists)
	plan.OnSystem = types.BoolValue(zone.OnSystem)
	plan.Expiry = types.StringValue(zone.Expiry)
	plan.NextDue = types.StringValue(zone.NextDue)
	plan.Service = types.StringValue(zone.Service)

	resp.Diagnostics.AddWarning(
		"Zone Adopted",
		fmt.Sprintf("Zone '%s' already exists in EasyDNS and has been adopted into Terraform state. For new zones, please use 'terraform import' after creating them in the EasyDNS dashboard.", domain),
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ZoneResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone, err := r.client.GetZone(ctx, state.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading zone",
			fmt.Sprintf("Could not read zone %s: %s", state.Domain.ValueString(), err),
		)
		return
	}

	// Map to state
	state.ID = types.StringValue(zone.ID)
	state.Domain = types.StringValue(zone.Domain)
	state.Exists = types.BoolValue(zone.Exists)
	state.OnSystem = types.BoolValue(zone.OnSystem)
	state.Expiry = types.StringValue(zone.Expiry)
	state.NextDue = types.StringValue(zone.NextDue)
	state.Service = types.StringValue(zone.Service)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *ZoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Zone attributes are all computed except domain, which requires replace
	// So update should never be called, but implement it for safety
	var plan ZoneResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ZoneResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Don't actually delete the zone - just remove from state
	// Deleting a domain is destructive and should be done manually
	resp.Diagnostics.AddWarning(
		"Zone Not Deleted",
		fmt.Sprintf("Zone '%s' has been removed from Terraform state but NOT deleted from EasyDNS. To delete the zone, use the EasyDNS dashboard.", state.Domain.ValueString()),
	)
}

func (r *ZoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by domain name
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), req.ID)...)
}
