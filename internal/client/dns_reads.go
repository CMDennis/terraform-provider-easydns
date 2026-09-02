package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
)

type ParsedRecord struct {
	Record
	URL       string
	OrigRdata string
}

type apiParsedRecord struct {
	apiRecord
	URL       string `json:"url"`
	OrigRdata string `json:"orig_rdata"`
}

type apiParsedRecordsResponse struct {
	Data   oneOrMany[apiParsedRecord] `json:"data"`
	Status flexibleInt64              `json:"status"`
}

func (c *Client) GetParsedRecords(ctx context.Context, domain string) ([]ParsedRecord, error) {
	normalized, err := NormalizeDomain(domain)
	if err != nil {
		return nil, err
	}
	var response apiParsedRecordsResponse
	if err := c.doJSON(ctx, http.MethodGet, c.endpoint("zones", "records", "parsed", normalized), nil, &response, requestOptions{safeToRetry: true}); err != nil {
		return nil, err
	}
	records := make([]ParsedRecord, len(response.Data))
	for index, wire := range response.Data {
		records[index] = ParsedRecord{Record: wire.model(), URL: wire.URL, OrigRdata: wire.OrigRdata}
	}
	sort.SliceStable(records, func(left, right int) bool {
		if records[left].ID == records[right].ID {
			return records[left].Rdata < records[right].Rdata
		}
		return lessRecordID(records[left].ID, records[right].ID)
	})
	return records, nil
}

type ZoneSOA struct {
	Domain string
	Serial int64
}

type apiZoneSOA struct {
	Domain string        `json:"domain"`
	SOA    flexibleInt64 `json:"soa"`
}

type apiZoneSOAResponse struct {
	apiZoneSOA
	Data *apiZoneSOA `json:"data"`
}

func (c *Client) GetZoneSOA(ctx context.Context, domain string) (*ZoneSOA, error) {
	normalized, err := NormalizeDomain(domain)
	if err != nil {
		return nil, err
	}
	var response apiZoneSOAResponse
	if err := c.doJSON(ctx, http.MethodGet, c.endpoint("zones", "records", "soa", normalized), nil, &response, requestOptions{safeToRetry: true}); err != nil {
		return nil, err
	}
	value := response.apiZoneSOA
	if response.Data != nil {
		value = *response.Data
	}
	if value.Domain == "" {
		value.Domain = normalized
	}
	if !value.SOA.Set {
		return nil, fmt.Errorf("EasyDNS SOA response did not include a serial")
	}
	return &ZoneSOA{Domain: value.Domain, Serial: value.SOA.Value}, nil
}

type PageOptions struct {
	Start int64
	Max   int64
}

type GeoRegion struct {
	ID       int64
	GeoCode  string
	Location string
}

type GeoRegionsResult struct {
	Regions []GeoRegion
	Count   int64
	Total   int64
	Start   int64
	Max     int64
}

type apiGeoRegion struct {
	ID       flexibleInt64 `json:"id"`
	GeoCode  string        `json:"geo_code"`
	Location string        `json:"location"`
}

type apiGeoRegionsResponse struct {
	Data   oneOrMany[apiGeoRegion] `json:"data"`
	Count  flexibleInt64           `json:"count"`
	Total  flexibleInt64           `json:"total"`
	Start  flexibleInt64           `json:"start"`
	Max    flexibleInt64           `json:"max"`
	Status flexibleInt64           `json:"status"`
}

func (c *Client) GetGeoRegions(ctx context.Context, page *PageOptions) (*GeoRegionsResult, error) {
	if page != nil && (page.Start < 0 || page.Max < 1) {
		return nil, fmt.Errorf("geo-region page requires start >= 0 and max >= 1")
	}
	const defaultPageSize = int64(100)
	const maximumPages = 1000
	start := int64(0)
	pageSize := defaultPageSize
	if page != nil {
		start, pageSize = page.Start, page.Max
	}
	result := &GeoRegionsResult{Start: start, Max: pageSize}

	for pageNumber := 0; pageNumber < maximumPages; pageNumber++ {
		endpoint := c.endpoint("zones", "geo", "region", "list")
		query := url.Values{}
		query.Set("start", strconv.FormatInt(start, 10))
		query.Set("max", strconv.FormatInt(pageSize, 10))
		endpoint.RawQuery = query.Encode()

		var response apiGeoRegionsResponse
		if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &response, requestOptions{safeToRetry: true}); err != nil {
			return nil, err
		}
		for _, wire := range response.Data {
			result.Regions = append(result.Regions, GeoRegion{ID: wire.ID.Value, GeoCode: wire.GeoCode, Location: wire.Location})
		}
		result.Count += int64(len(response.Data))
		if response.Total.Set {
			result.Total = response.Total.Value
		} else {
			result.Total = result.Count
		}
		if page != nil || result.Count >= result.Total {
			break
		}
		count := int64(len(response.Data))
		if response.Count.Set {
			count = response.Count.Value
		}
		if count <= 0 {
			return nil, fmt.Errorf("EasyDNS geo-region pagination made no progress at start %d", start)
		}
		if response.Start.Set {
			start = response.Start.Value + count
		} else {
			start += count
		}
		if response.Max.Set && response.Max.Value > 0 {
			pageSize = response.Max.Value
		}
	}
	if result.Total > result.Count && page == nil {
		return nil, fmt.Errorf("EasyDNS geo-region pagination exceeded %d pages", maximumPages)
	}
	sort.SliceStable(result.Regions, func(left, right int) bool { return result.Regions[left].ID < result.Regions[right].ID })
	return result, nil
}
