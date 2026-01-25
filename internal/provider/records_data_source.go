package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure RecordsDataSource satisfies the DataSource interface
var _ datasource.DataSource = &RecordsDataSource{}

// RecordsDataSource defines the data source for listing all records
type RecordsDataSource struct {
	client *Client
}

// RecordsDataSourceModel describes the data source data model
type RecordsDataSourceModel struct {
	Domain  types.String  `tfsdk:"domain"`
	Records []RecordModel `tfsdk:"records"`
}

// RecordModel is the model for individual records in the list
type RecordModel struct {
	ID      types.String `tfsdk:"id"`
	Host    types.String `tfsdk:"host"`
	Type    types.String `tfsdk:"type"`
	Rdata   types.String `tfsdk:"rdata"`
	TTL     types.Int64  `tfsdk:"ttl"`
	Prio    types.Int64  `tfsdk:"prio"`
	LastMod types.String `tfsdk:"last_mod"`
}

// NewRecordsDataSource creates a new data source
func NewRecordsDataSource() datasource.DataSource {
	return &RecordsDataSource{}
}

func (d *RecordsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_records"
}

func (d *RecordsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches all DNS records for a domain from EasyDNS.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				Description: "The domain/zone to list records for (e.g., 'example.com').",
				Required:    true,
			},
			"records": schema.ListNestedAttribute{
				Description: "List of DNS records in the domain.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "The unique identifier of the DNS record.",
							Computed:    true,
						},
						"host": schema.StringAttribute{
							Description: "The hostname/subdomain.",
							Computed:    true,
						},
						"type": schema.StringAttribute{
							Description: "The DNS record type.",
							Computed:    true,
						},
						"rdata": schema.StringAttribute{
							Description: "The record data.",
							Computed:    true,
						},
						"ttl": schema.Int64Attribute{
							Description: "Time to live in seconds.",
							Computed:    true,
						},
						"prio": schema.Int64Attribute{
							Description: "Priority for MX and SRV records.",
							Computed:    true,
						},
						"last_mod": schema.StringAttribute{
							Description: "The last modification timestamp.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *RecordsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RecordsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config RecordsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain := config.Domain.ValueString()

	records, err := d.client.GetRecords(domain)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading DNS records",
			fmt.Sprintf("Could not read records for domain %s: %s", domain, err),
		)
		return
	}

	// Map to state
	config.Records = make([]RecordModel, len(records))
	for i, record := range records {
		config.Records[i] = RecordModel{
			ID:      types.StringValue(record.ID),
			Host:    types.StringValue(record.Host),
			Type:    types.StringValue(record.Type),
			Rdata:   types.StringValue(record.Rdata),
			TTL:     types.Int64Value(record.TTL),
			Prio:    types.Int64Value(record.Prio),
			LastMod: types.StringValue(record.LastMod),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
