package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &GeoRegionsDataSource{}

type GeoRegionsDataSource struct {
	client *Client
}

type GeoRegionsDataSourceModel struct {
	Start         types.Int64      `tfsdk:"start"`
	Max           types.Int64      `tfsdk:"max"`
	ReturnedCount types.Int64      `tfsdk:"returned_count"`
	Total         types.Int64      `tfsdk:"total"`
	Regions       []GeoRegionModel `tfsdk:"regions"`
}

type GeoRegionModel struct {
	ID       types.Int64  `tfsdk:"id"`
	GeoCode  types.String `tfsdk:"geo_code"`
	Location types.String `tfsdk:"location"`
}

func NewGeoRegionsDataSource() datasource.DataSource {
	return &GeoRegionsDataSource{}
}

func (d *GeoRegionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_geo_regions"
}

func (d *GeoRegionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches EasyDNS geographic regions, following all pages unless start or max requests one page.",
		Attributes: map[string]schema.Attribute{
			"start": schema.Int64Attribute{
				Description: "Optional first result index. Setting start or max requests one page.", Optional: true,
				Validators: []validator.Int64{NonNegativeValidator("start")},
			},
			"max": schema.Int64Attribute{
				Description: "Optional maximum results in one requested page.", Optional: true,
				Validators: []validator.Int64{PositiveValidator("max")},
			},
			"returned_count": schema.Int64Attribute{Description: "Number of regions returned.", Computed: true},
			"total":          schema.Int64Attribute{Description: "Total regions reported by EasyDNS.", Computed: true},
			"regions": schema.ListNestedAttribute{
				Description: "Geo regions sorted by numeric ID.", Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":       schema.Int64Attribute{Description: "Numeric geo-region ID.", Computed: true},
					"geo_code": schema.StringAttribute{Description: "Short EasyDNS geo-region code.", Computed: true},
					"location": schema.StringAttribute{Description: "Human-readable region location.", Computed: true},
				}},
			},
		},
	}
}

func (d *GeoRegionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *GeoRegionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config GeoRegionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var page *PageOptions
	if !config.Start.IsNull() || !config.Max.IsNull() {
		page = &PageOptions{Start: 0, Max: 100}
		if !config.Start.IsNull() {
			page.Start = config.Start.ValueInt64()
		}
		if !config.Max.IsNull() {
			page.Max = config.Max.ValueInt64()
		}
	}
	result, err := d.client.GetGeoRegions(ctx, page)
	if err != nil {
		resp.Diagnostics.AddError("Error reading geo regions", fmt.Sprintf("Could not read geo regions: %s", err))
		return
	}
	config.ReturnedCount = types.Int64Value(result.Count)
	config.Total = types.Int64Value(result.Total)
	config.Regions = make([]GeoRegionModel, len(result.Regions))
	for index, region := range result.Regions {
		config.Regions[index] = GeoRegionModel{ID: types.Int64Value(region.ID), GeoCode: types.StringValue(region.GeoCode), Location: types.StringValue(region.Location)}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
