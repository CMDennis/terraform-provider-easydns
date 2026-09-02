package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure ZoneDataSource satisfies the DataSource interface
var _ datasource.DataSource = &ZoneDataSource{}

// ZoneDataSource defines the data source implementation
type ZoneDataSource struct {
	client *Client
}

// ZoneDataSourceModel describes the data source data model
type ZoneDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Domain   types.String `tfsdk:"domain"`
	Exists   types.Bool   `tfsdk:"exists"`
	OnSystem types.Bool   `tfsdk:"on_system"`
	Expiry   types.String `tfsdk:"expiry"`
	NextDue  types.String `tfsdk:"next_due"`
	Service  types.String `tfsdk:"service"`
}

// NewZoneDataSource creates a new data source
func NewZoneDataSource() datasource.DataSource {
	return &ZoneDataSource{}
}

func (d *ZoneDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone"
}

func (d *ZoneDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:        "Fetches information about a DNS zone from EasyDNS.",
		DeprecationMessage: "The easydns_zone data source is deprecated and will be removed in v2.0.0. Use the easydns_domain data source, which returns the same fields plus cloned_to and subscription_id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The zone identifier.",
				Computed:    true,
			},
			"domain": schema.StringAttribute{
				Description: "The domain name of the zone.",
				Required:    true,
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

func (d *ZoneDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *ZoneDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config ZoneDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain := config.Domain.ValueString()

	zone, err := d.client.GetZone(ctx, domain)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading zone",
			fmt.Sprintf("Could not read zone %s: %s", domain, err),
		)
		return
	}

	// Map to state. `domain` is a required argument and is deliberately left as
	// configured; Terraform rejects a data source that alters its own arguments.
	config.ID = types.StringValue(zone.ID)
	config.Exists = types.BoolValue(zone.Exists)
	config.OnSystem = types.BoolValue(zone.OnSystem)
	config.Expiry = types.StringValue(zone.Expiry)
	config.NextDue = types.StringValue(zone.NextDue)
	config.Service = types.StringValue(zone.Service)

	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
