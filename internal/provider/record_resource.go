package provider

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure RecordResource satisfies various resource interfaces
var (
	_ resource.Resource                   = &RecordResource{}
	_ resource.ResourceWithImportState    = &RecordResource{}
	_ resource.ResourceWithValidateConfig = &RecordResource{}
)

// RecordResource defines the resource implementation
type RecordResource struct {
	client *Client
}

// RecordResourceModel describes the resource data model
type RecordResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Domain    types.String `tfsdk:"domain"`
	Host      types.String `tfsdk:"host"`
	Type      types.String `tfsdk:"type"`
	Rdata     types.String `tfsdk:"rdata"`
	TTL       types.Int64  `tfsdk:"ttl"`
	Prio      types.Int64  `tfsdk:"prio"`
	GeozoneID types.Int64  `tfsdk:"geozone_id"`
	WriteMode types.String `tfsdk:"write_mode"`
	LastMod   types.String `tfsdk:"last_mod"`
}

// NewRecordResource creates a new record resource
func NewRecordResource() resource.Resource {
	return &RecordResource{}
}

func (r *RecordResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_record"
}

func (r *RecordResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a DNS record in EasyDNS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the DNS record.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"domain": schema.StringAttribute{
				Description: "The domain/zone for the DNS record (e.g., 'example.com').",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"host": schema.StringAttribute{
				Description: "The hostname/subdomain for the record. Use '@' for the root domain.",
				Required:    true,
				Validators: []validator.String{
					HostnameValidator(),
				},
			},
			"type": schema.StringAttribute{
				Description: "The DNS record type (A, AAAA, CNAME, MX, TXT, etc.).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					RecordTypeValidator(),
				},
			},
			"rdata": schema.StringAttribute{
				Description: "The record data (IP address, hostname, text value, etc.).",
				Required:    true,
				// IP validation is done in ValidateConfig based on record type
			},
			"ttl": schema.Int64Attribute{
				Description: "Time to live in seconds. Minimum depends on your EasyDNS plan.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(600),
				Validators: []validator.Int64{
					TTLValidator(),
				},
			},
			"prio": schema.Int64Attribute{
				Description: "Priority for MX and SRV records (0-100).",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
				Validators: []validator.Int64{
					PriorityValidator(),
				},
			},
			"geozone_id": schema.Int64Attribute{
				Description: "EasyDNS geo-region ID for this record. Zero disables geographic targeting.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
				Validators: []validator.Int64{
					NonNegativeValidator("geozone_id"),
				},
			},
			"write_mode": schema.StringAttribute{
				Description: "Optional record-level mutation mode override: 'synchronous' or 'asynchronous'.",
				Optional:    true,
				Validators: []validator.String{
					RecordWriteModeValidator(),
				},
			},
			"last_mod": schema.StringAttribute{
				Description: "The last modification timestamp of the record.",
				Computed:    true,
			},
		},
	}
}

func (r *RecordResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *RecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RecordResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := CreateRecordRequest{
		Domain:    plan.Domain.ValueString(),
		Host:      plan.Host.ValueString(),
		Type:      plan.Type.ValueString(),
		Rdata:     plan.Rdata.ValueString(),
		TTL:       plan.TTL.ValueInt64(),
		Prio:      plan.Prio.ValueInt64(),
		GeozoneID: plan.GeozoneID.ValueInt64(),
	}

	record, err := r.client.CreateRecordWithMode(ctx, createReq, r.writeMode(plan.WriteMode))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating DNS record",
			fmt.Sprintf("Could not create record: %s", err),
		)
		return
	}

	// Map response to state
	applyRecordToResourceModel(&plan, record)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *RecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state RecordResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	record, err := r.client.GetRecord(ctx, state.Domain.ValueString(), state.ID.ValueString())
	if err != nil {
		// If record not found, remove from state
		if IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading DNS record",
			fmt.Sprintf("Could not read record %s: %s", state.ID.ValueString(), err),
		)
		return
	}

	applyRecordToResourceModel(&state, record)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *RecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan RecordResourceModel
	var state RecordResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := CreateRecordRequest{
		Domain:    plan.Domain.ValueString(),
		Host:      plan.Host.ValueString(),
		Type:      plan.Type.ValueString(),
		Rdata:     plan.Rdata.ValueString(),
		TTL:       plan.TTL.ValueInt64(),
		Prio:      plan.Prio.ValueInt64(),
		GeozoneID: plan.GeozoneID.ValueInt64(),
	}

	record, err := r.client.UpdateRecordWithMode(ctx, state.ID.ValueString(), updateReq, r.writeMode(plan.WriteMode))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating DNS record",
			fmt.Sprintf("Could not update record %s: %s", state.ID.ValueString(), err),
		)
		return
	}

	// Map response to state
	applyRecordToResourceModel(&plan, record)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *RecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state RecordResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteRecordWithMode(ctx, state.Domain.ValueString(), state.ID.ValueString(), r.writeMode(state.WriteMode))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting DNS record",
			fmt.Sprintf("Could not delete record %s: %s", state.ID.ValueString(), err),
		)
		return
	}
}

func (r *RecordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	domain, recordID, err := parseRecordImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), domain)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), recordID)...)
}

func (r *RecordResource) writeMode(value types.String) RecordWriteMode {
	if value.IsNull() || value.IsUnknown() || value.ValueString() == "" {
		return r.client.RecordWriteMode()
	}
	return RecordWriteMode(value.ValueString())
}

func applyRecordToResourceModel(model *RecordResourceModel, record *Record) {
	domain, err := NormalizeDomain(record.Domain)
	if err != nil {
		domain = record.Domain
	}
	rdata, err := NormalizeRecordRdata(record.Type, record.Rdata)
	if err != nil {
		rdata = record.Rdata
	}
	desired := CreateRecordRequest{
		Domain: model.Domain.ValueString(), Host: model.Host.ValueString(), Type: model.Type.ValueString(),
		Rdata: model.Rdata.ValueString(), TTL: model.TTL.ValueInt64(), Prio: model.Prio.ValueInt64(), GeozoneID: model.GeozoneID.ValueInt64(),
	}
	if RecordsEquivalent(*record, desired) && !model.Rdata.IsNull() && !model.Rdata.IsUnknown() {
		rdata = model.Rdata.ValueString()
	}
	model.ID = types.StringValue(record.ID)
	model.Domain = types.StringValue(domain)
	model.Host = types.StringValue(NormalizeHost(record.Host))
	model.Type = types.StringValue(NormalizeRecordType(record.Type))
	model.Rdata = types.StringValue(rdata)
	model.TTL = types.Int64Value(record.TTL)
	model.Prio = types.Int64Value(record.Prio)
	model.GeozoneID = types.Int64Value(record.GeozoneID)
	model.LastMod = types.StringValue(record.LastMod)
}

func parseRecordImportID(value string) (string, string, error) {
	const expected = "Import ID must be in the form 'domain:record_id' with a positive numeric record ID."
	if strings.TrimSpace(value) != value || strings.Count(value, ":") != 1 {
		return "", "", fmt.Errorf("%s", expected)
	}
	parts := strings.SplitN(value, ":", 2)
	domain, err := NormalizeDomain(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("%s", expected)
	}
	recordID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || recordID <= 0 {
		return "", "", fmt.Errorf("%s", expected)
	}
	return domain, strconv.FormatInt(recordID, 10), nil
}

func (r *RecordResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config RecordResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Skip validation if values are unknown (e.g., from other resources)
	if config.Type.IsUnknown() || config.Rdata.IsUnknown() {
		return
	}

	recordType := strings.ToUpper(config.Type.ValueString())
	rdata := config.Rdata.ValueString()

	// Validate IP address format based on record type
	// Note: EasyDNS has special values like "PARK" for parked domains
	switch recordType {
	case "A":
		// Allow EasyDNS special values
		if rdata != "PARK" {
			ip := net.ParseIP(rdata)
			if ip == nil || ip.To4() == nil {
				resp.Diagnostics.AddError(
					"Invalid IPv4 Address",
					fmt.Sprintf("A record rdata '%s' is not a valid IPv4 address.", rdata),
				)
			}
		}
	case "AAAA":
		ip := net.ParseIP(rdata)
		if ip == nil || ip.To4() != nil {
			resp.Diagnostics.AddError(
				"Invalid IPv6 Address",
				fmt.Sprintf("AAAA record rdata '%s' is not a valid IPv6 address.", rdata),
			)
		}
	}

	// Warn if priority is set for non-MX/SRV records
	if !config.Prio.IsNull() && !config.Prio.IsUnknown() {
		prio := config.Prio.ValueInt64()
		if prio != 0 && !priorityRecordTypes[recordType] {
			resp.Diagnostics.AddWarning(
				"Priority Ignored",
				fmt.Sprintf("Priority is only used for MX and SRV records. It will be ignored for %s records.", recordType),
			)
		}
	}
}
