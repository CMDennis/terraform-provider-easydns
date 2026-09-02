package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure RecordDataSource satisfies the DataSource interface
var _ datasource.DataSource = &RecordDataSource{}

// RecordDataSource defines the data source implementation
type RecordDataSource struct {
	client *Client
}

// RecordDataSourceModel describes the data source data model
type RecordDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	Domain    types.String `tfsdk:"domain"`
	Host      types.String `tfsdk:"host"`
	Type      types.String `tfsdk:"type"`
	Rdata     types.String `tfsdk:"rdata"`
	TTL       types.Int64  `tfsdk:"ttl"`
	Prio      types.Int64  `tfsdk:"prio"`
	GeozoneID types.Int64  `tfsdk:"geozone_id"`
	LastMod   types.String `tfsdk:"last_mod"`
}

// NewRecordDataSource creates a new data source
func NewRecordDataSource() datasource.DataSource {
	return &RecordDataSource{}
}

func (d *RecordDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_record"
}

func (d *RecordDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a DNS record from EasyDNS by domain, host, and type.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the DNS record.",
				Computed:    true,
			},
			"domain": schema.StringAttribute{
				Description: "The domain/zone to search in (e.g., 'example.com').",
				Required:    true,
			},
			"host": schema.StringAttribute{
				Description: "The hostname/subdomain to find. Use '@' for the root domain.",
				Required:    true,
			},
			"type": schema.StringAttribute{
				Description: "The DNS record type (A, AAAA, CNAME, MX, TXT, etc.).",
				Required:    true,
			},
			"rdata": schema.StringAttribute{
				Description: "The record data (IP address, hostname, text value, etc.).",
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
			"geozone_id": schema.Int64Attribute{
				Description: "EasyDNS geo-region ID, or zero for a global record.",
				Computed:    true,
			},
			"last_mod": schema.StringAttribute{
				Description: "The last modification timestamp of the record.",
				Computed:    true,
			},
		},
	}
}

func (d *RecordDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RecordDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config RecordDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain := config.Domain.ValueString()
	host := config.Host.ValueString()
	recordType := config.Type.ValueString()

	records, err := d.client.GetRecords(ctx, domain)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading DNS records",
			fmt.Sprintf("Could not read records for domain %s: %s", domain, err),
		)
		return
	}

	found, matchErr := findRecordByHostAndType(records, host, recordType)
	if matchErr != nil {
		summary := "Record lookup was not unique"
		var match *recordMatchError
		if errors.As(matchErr, &match) {
			summary = match.summary
		}
		resp.Diagnostics.AddError(summary, matchErr.Error())
		return
	}

	// Map to state
	config.ID = types.StringValue(found.ID)
	config.Rdata = types.StringValue(found.Rdata)
	config.TTL = types.Int64Value(found.TTL)
	config.Prio = types.Int64Value(found.Prio)
	config.GeozoneID = types.Int64Value(found.GeozoneID)
	config.LastMod = types.StringValue(found.LastMod)

	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}

func findRecordByHostAndType(records []Record, host, recordType string) (*Record, error) {
	matches := make([]Record, 0, 1)
	for _, record := range records {
		if NormalizeHost(record.Host) == NormalizeHost(host) && NormalizeRecordType(record.Type) == NormalizeRecordType(recordType) {
			matches = append(matches, record)
		}
	}
	if len(matches) == 0 {
		return nil, &recordMatchError{
			summary: "Record not found",
			detail:  fmt.Sprintf("no %s record found for host %q", recordType, host),
		}
	}
	if len(matches) > 1 {
		return nil, &recordMatchError{
			summary: "Record lookup was not unique",
			detail:  fmt.Sprintf("found %d %s records for host %q; use easydns_records and select by record ID", len(matches), recordType, host),
		}
	}
	return &matches[0], nil
}

// recordMatchError carries the diagnostic summary that fits the failure, so a
// singleton lookup that found nothing is not reported as an ambiguity.
type recordMatchError struct {
	summary string
	detail  string
}

func (err *recordMatchError) Error() string {
	return err.detail
}
