package provider

import (
	"context"
	"fmt"
	"net/mail"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &MailmapResource{}
	_ resource.ResourceWithImportState    = &MailmapResource{}
	_ resource.ResourceWithValidateConfig = &MailmapResource{}
)

type MailmapResource struct{ client *Client }

type MailmapResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Domain       types.String `tfsdk:"domain"`
	Alias        types.String `tfsdk:"alias"`
	Host         types.String `tfsdk:"host"`
	Email        types.String `tfsdk:"email"`
	Destinations types.Set    `tfsdk:"destinations"`
	Active       types.Bool   `tfsdk:"active"`
	LastModified types.String `tfsdk:"last_modified"`
}

func NewMailmapResource() resource.Resource { return &MailmapResource{} }

func (r *MailmapResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mailmap"
}

func (r *MailmapResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an EasyDNS mail-forwarding map. Mutations are sent once and reconciled by reading the domain's mailmap collection.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Immutable numeric EasyDNS mailmap ID.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"domain": schema.StringAttribute{
				Description:   "Domain that owns the mailmap.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{DomainNameValidator()},
			},
			"alias": schema.StringAttribute{
				Description: "Local part of the source address, without @ or the domain.",
				Required:    true,
			},
			"host": schema.StringAttribute{
				Description: "Relative mail host. Use @ for the domain apex, or a hostname such as lists.",
				Required:    true,
				Validators:  []validator.String{HostnameValidator()},
			},
			"email": schema.StringAttribute{
				Description: "Fully-qualified source address derived from alias, host, and domain.",
				Computed:    true,
			},
			"destinations": schema.SetAttribute{
				Description: "One or more destination email addresses. Order is not significant.",
				Required:    true,
				ElementType: types.StringType,
			},
			"active": schema.BoolAttribute{
				Description:   "Whether forwarding is active. Defaults to true.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"last_modified": schema.StringAttribute{
				Description: "Timestamp last reported by EasyDNS.",
				Computed:    true,
			},
		},
	}
}

func (r *MailmapResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *MailmapResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config MailmapResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !config.Alias.IsNull() && !config.Alias.IsUnknown() {
		alias := config.Alias.ValueString()
		address, err := mail.ParseAddress(alias + "@example.invalid")
		if strings.TrimSpace(alias) != alias || alias == "" || strings.Contains(alias, "@") || err != nil || address.Address != alias+"@example.invalid" {
			resp.Diagnostics.AddAttributeError(path.Root("alias"), "Invalid Mailmap Alias", "alias must be a non-empty email local part without @ or a domain.")
		}
	}
	if config.Destinations.IsNull() || config.Destinations.IsUnknown() {
		return
	}
	var destinations []string
	resp.Diagnostics.Append(config.Destinations.ElementsAs(ctx, &destinations, false)...)
	if len(destinations) == 0 {
		resp.Diagnostics.AddAttributeError(path.Root("destinations"), "Mailmap Requires A Destination", "Set at least one destination email address.")
	}
	for _, destination := range destinations {
		address, err := mail.ParseAddress(destination)
		if strings.TrimSpace(destination) != destination || err != nil || address.Address != destination {
			resp.Diagnostics.AddAttributeError(path.Root("destinations"), "Invalid Mailmap Destination", fmt.Sprintf("%q is not a plain email address.", destination))
		}
	}
}

func (r *MailmapResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan MailmapResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	request := mailmapRequestFromModel(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	mailmap, err := r.client.CreateMailmap(ctx, request)
	if err != nil {
		resp.Diagnostics.AddError("Error creating mailmap", fmt.Sprintf("Could not create the mailmap for %s: %s", plan.Domain.ValueString(), err))
		return
	}
	applyMailmapToModel(ctx, &plan, mailmap, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *MailmapResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state MailmapResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	mailmap, err := r.client.GetMailmap(ctx, state.Domain.ValueString(), state.ID.ValueString())
	if err != nil {
		if IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading mailmap", fmt.Sprintf("Could not read mailmap %s: %s", state.ID.ValueString(), err))
		return
	}
	applyMailmapToModel(ctx, &state, mailmap, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *MailmapResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state MailmapResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	request := mailmapRequestFromModel(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	mailmap, err := r.client.UpdateMailmap(ctx, state.ID.ValueString(), state.Email.ValueString(), request)
	if err != nil {
		resp.Diagnostics.AddError("Error updating mailmap", fmt.Sprintf("Could not update mailmap %s: %s", state.ID.ValueString(), err))
		return
	}
	applyMailmapToModel(ctx, &plan, mailmap, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *MailmapResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state MailmapResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteMailmap(ctx, state.Domain.ValueString(), state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting mailmap", fmt.Sprintf("Could not delete mailmap %s: %s", state.ID.ValueString(), err))
	}
}

func (r *MailmapResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	domain, id, err := parseMailmapImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), domain)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func parseMailmapImportID(value string) (string, string, error) {
	const expected = "Import ID must be in the form 'domain:mailmap_id', for example 'example.com:1234'."
	if strings.TrimSpace(value) != value || strings.Count(value, ":") != 1 {
		return "", "", fmt.Errorf("%s", expected)
	}
	parts := strings.SplitN(value, ":", 2)
	domain, err := NormalizeDomain(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("%s", expected)
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || id < 1 {
		return "", "", fmt.Errorf("%s", expected)
	}
	return domain, strconv.FormatInt(id, 10), nil
}

func mailmapRequestFromModel(ctx context.Context, model MailmapResourceModel, diagnostics *diag.Diagnostics) MailmapRequest {
	var destinations []string
	diagnostics.Append(model.Destinations.ElementsAs(ctx, &destinations, false)...)
	active := true
	if !model.Active.IsNull() && !model.Active.IsUnknown() {
		active = model.Active.ValueBool()
	}
	return MailmapRequest{
		Domain: model.Domain.ValueString(), Alias: model.Alias.ValueString(), Host: model.Host.ValueString(),
		Destinations: destinations, Active: active,
	}
}

func applyMailmapToModel(ctx context.Context, model *MailmapResourceModel, mailmap *Mailmap, diagnostics *diag.Diagnostics) {
	model.ID = types.StringValue(mailmap.ID)
	model.Domain = types.StringValue(mailmap.Domain)
	model.Alias = types.StringValue(mailmap.Alias)
	model.Host = types.StringValue(mailmap.Host)
	model.Email = types.StringValue(mailmap.Email())
	model.Active = types.BoolValue(mailmap.Active)
	model.LastModified = types.StringValue(mailmap.LastModified)
	destinations, destinationDiagnostics := types.SetValueFrom(ctx, types.StringType, mailmap.Destinations)
	diagnostics.Append(destinationDiagnostics...)
	model.Destinations = destinations
}
