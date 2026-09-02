package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ZoneSOADataSource{}

type ZoneSOADataSource struct {
	client *Client
}

type ZoneSOADataSourceModel struct {
	Domain types.String `tfsdk:"domain"`
	Serial types.Int64  `tfsdk:"serial"`
}

func NewZoneSOADataSource() datasource.DataSource {
	return &ZoneSOADataSource{}
}

func (d *ZoneSOADataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone_soa"
}

func (d *ZoneSOADataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches the current EasyDNS SOA serial for a zone.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{Description: "Domain whose SOA serial is returned.", Required: true},
			"serial": schema.Int64Attribute{Description: "Current SOA serial.", Computed: true},
		},
	}
}

func (d *ZoneSOADataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *Client, got: %T.", req.ProviderData))
		return
	}
	d.client = client
}

func (d *ZoneSOADataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config ZoneSOADataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	soa, err := d.client.GetZoneSOA(ctx, config.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading zone SOA", fmt.Sprintf("Could not read SOA serial: %s", err))
		return
	}
	// `domain` is a required argument. Terraform requires a data source to
	// return configured arguments unchanged, so the normalized domain that the
	// client resolved is deliberately not written back over the configuration.
	config.Serial = types.Int64Value(soa.Serial)
	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
