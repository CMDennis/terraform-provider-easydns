package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ParsedRecordsDataSource{}

type ParsedRecordsDataSource struct {
	client *Client
}

type ParsedRecordsDataSourceModel struct {
	Domain  types.String        `tfsdk:"domain"`
	Records []ParsedRecordModel `tfsdk:"records"`
}

type ParsedRecordModel struct {
	ID        types.String `tfsdk:"id"`
	Host      types.String `tfsdk:"host"`
	Type      types.String `tfsdk:"type"`
	Rdata     types.String `tfsdk:"rdata"`
	TTL       types.Int64  `tfsdk:"ttl"`
	Prio      types.Int64  `tfsdk:"prio"`
	GeozoneID types.Int64  `tfsdk:"geozone_id"`
	LastMod   types.String `tfsdk:"last_mod"`
	URL       types.String `tfsdk:"url"`
	OrigRdata types.String `tfsdk:"orig_rdata"`
}

func NewParsedRecordsDataSource() datasource.DataSource {
	return &ParsedRecordsDataSource{}
}

func (d *ParsedRecordsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_parsed_records"
}

func (d *ParsedRecordsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	computedString := func(description string) schema.StringAttribute {
		return schema.StringAttribute{Description: description, Computed: true}
	}
	computedInt := func(description string) schema.Int64Attribute {
		return schema.Int64Attribute{Description: description, Computed: true}
	}
	resp.Schema = schema.Schema{
		Description: "Fetches EasyDNS zone records in their fully parsed zone-file form.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{Description: "Domain whose parsed records are returned.", Required: true},
			"records": schema.ListNestedAttribute{
				Description: "Parsed zone records. One stored record ID may produce multiple parsed entries.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":         computedString("EasyDNS stored record ID."),
					"host":       computedString("Parsed record host."),
					"type":       computedString("Parsed record type."),
					"rdata":      computedString("Parsed record data."),
					"ttl":        computedInt("Time to live in seconds."),
					"prio":       computedInt("Record priority."),
					"geozone_id": computedInt("EasyDNS geo-region ID."),
					"last_mod":   computedString("Last modification timestamp."),
					"url":        computedString("Original URL target for EasyDNS custom URL records."),
					"orig_rdata": computedString("Original stored rdata before EasyDNS parsing."),
				}},
			},
		},
	}
}

func (d *ParsedRecordsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ParsedRecordsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config ParsedRecordsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	records, err := d.client.GetParsedRecords(ctx, config.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading parsed DNS records", fmt.Sprintf("Could not read parsed records: %s", err))
		return
	}
	config.Records = make([]ParsedRecordModel, len(records))
	for index, record := range records {
		config.Records[index] = ParsedRecordModel{
			ID: types.StringValue(record.ID), Host: types.StringValue(record.Host), Type: types.StringValue(record.Type),
			Rdata: types.StringValue(record.Rdata), TTL: types.Int64Value(record.TTL), Prio: types.Int64Value(record.Prio),
			GeozoneID: types.Int64Value(record.GeozoneID), LastMod: types.StringValue(record.LastMod),
			URL: types.StringValue(record.URL), OrigRdata: types.StringValue(record.OrigRdata),
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
