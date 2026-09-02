package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/action"
	actionschema "github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ action.Action              = &ForceZoneReloadAction{}
	_ action.ActionWithConfigure = &ForceZoneReloadAction{}
	_ action.Action              = &SetPrimaryNameserverAction{}
	_ action.ActionWithConfigure = &SetPrimaryNameserverAction{}
)

func configureActionClient(req action.ConfigureRequest, resp *action.ConfigureResponse) *Client {
	if req.ProviderData == nil {
		return nil
	}
	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Action Configure Type", fmt.Sprintf("Expected *Client, got: %T.", req.ProviderData))
		return nil
	}
	return client
}

type ForceZoneReloadAction struct{ client *Client }

type ForceZoneReloadActionModel struct {
	Domain types.String `tfsdk:"domain"`
}

func NewForceZoneReloadAction() action.Action { return &ForceZoneReloadAction{} }

func (a *ForceZoneReloadAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_force_zone_reload"
}

func (a *ForceZoneReloadAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = actionschema.Schema{
		Description: "Forces immediate regeneration of a domain's zone. This imperative request is sent once and is never automatically retried.",
		Attributes: map[string]actionschema.Attribute{
			"domain": actionschema.StringAttribute{Description: "Domain whose zone should be regenerated.", Required: true, Validators: []validator.String{DomainNameValidator()}},
		},
	}
}

func (a *ForceZoneReloadAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	a.client = configureActionClient(req, resp)
}

func (a *ForceZoneReloadAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config ForceZoneReloadActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := a.client.ForceZoneReload(ctx, config.Domain.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error forcing zone reload", fmt.Sprintf("The request for %s failed: %s. Because this endpoint has a side effect, the provider did not retry it; verify the zone before invoking it again.", config.Domain.ValueString(), err))
	}
}

type SetPrimaryNameserverAction struct{ client *Client }

type SetPrimaryNameserverActionModel struct {
	Domain types.String `tfsdk:"domain"`
	Master types.String `tfsdk:"master"`
}

func NewSetPrimaryNameserverAction() action.Action { return &SetPrimaryNameserverAction{} }

func (a *SetPrimaryNameserverAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_set_primary_nameserver"
}

func (a *SetPrimaryNameserverAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = actionschema.Schema{
		Description: "Sets the primary nameserver address and changes a domain to secondary DNS. This imperative request is sent once and is never automatically retried.",
		Attributes: map[string]actionschema.Attribute{
			"domain": actionschema.StringAttribute{Description: "Domain to change to secondary DNS.", Required: true, Validators: []validator.String{DomainNameValidator()}},
			"master": actionschema.StringAttribute{Description: "IP address of the primary DNS server.", Required: true, Validators: []validator.String{IPAddressValidator()}},
		},
	}
}

func (a *SetPrimaryNameserverAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	a.client = configureActionClient(req, resp)
}

func (a *SetPrimaryNameserverAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config SetPrimaryNameserverActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := a.client.SetPrimaryNameserver(ctx, config.Domain.ValueString(), config.Master.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error setting primary nameserver", fmt.Sprintf("The request for %s failed: %s. Because this endpoint has a side effect, the provider did not retry it; verify the domain before invoking it again.", config.Domain.ValueString(), err))
	}
}
