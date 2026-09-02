package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
)

type Record struct {
	ID        string
	Domain    string
	Host      string
	Type      string
	Rdata     string
	TTL       int64
	Prio      int64
	GeozoneID int64
	LastMod   string
}

type CreateRecordRequest struct {
	Domain    string `json:"domain"`
	Host      string `json:"host"`
	Type      string `json:"type"`
	Rdata     string `json:"rdata"`
	TTL       int64  `json:"ttl,omitempty"`
	Prio      int64  `json:"prio,omitempty"`
	GeozoneID int64  `json:"geozone_id,omitempty"`
}

type apiRecord struct {
	ID        flexibleString `json:"id"`
	Domain    string         `json:"domain"`
	Host      string         `json:"host"`
	Type      string         `json:"type"`
	Rdata     string         `json:"rdata"`
	TTL       flexibleInt64  `json:"ttl"`
	Prio      flexibleInt64  `json:"prio"`
	GeozoneID flexibleInt64  `json:"geozone_id"`
	LastMod   string         `json:"last_mod"`
}

func (record apiRecord) model() Record {
	ttl := record.TTL.Value
	if !record.TTL.Set {
		ttl = 600
	}
	return Record{
		ID:        record.ID.Value,
		Domain:    record.Domain,
		Host:      record.Host,
		Type:      record.Type,
		Rdata:     record.Rdata,
		TTL:       ttl,
		Prio:      record.Prio.Value,
		GeozoneID: record.GeozoneID.Value,
		LastMod:   record.LastMod,
	}
}

type apiRecordResponse struct {
	Data   *apiRecord    `json:"data"`
	Status flexibleInt64 `json:"status"`
	TM     flexibleInt64 `json:"tm"`
	Msg    string        `json:"msg"`
}

type apiRecordsListResponse struct {
	Data   oneOrMany[apiRecord] `json:"data"`
	Count  flexibleInt64        `json:"count"`
	Total  flexibleInt64        `json:"total"`
	Start  flexibleInt64        `json:"start"`
	Max    flexibleInt64        `json:"max"`
	Status flexibleInt64        `json:"status"`
}

func (c *Client) CreateRecord(ctx context.Context, record CreateRecordRequest) (*Record, error) {
	return c.CreateRecordWithMode(ctx, record, c.recordWriteMode)
}

func (c *Client) CreateRecordWithMode(ctx context.Context, record CreateRecordRequest, mode RecordWriteMode) (*Record, error) {
	if err := validateRecordWriteMode(mode); err != nil {
		return nil, err
	}
	normalized, err := normalizeCreateRecordRequest(record)
	if err != nil {
		return nil, fmt.Errorf("normalize record create request: %w", err)
	}
	before, err := c.GetRecords(ctx, normalized.Domain)
	if err != nil {
		return nil, fmt.Errorf("snapshot records before create: %w", err)
	}
	beforeIDs := make(map[string]struct{}, len(before))
	for _, existing := range before {
		beforeIDs[existing.ID] = struct{}{}
	}

	created, writeErr := c.createRecordOnce(ctx, normalized, mode)
	if writeErr != nil && !IsAmbiguousWrite(writeErr) {
		return nil, writeErr
	}
	preferredID := ""
	if created != nil {
		preferredID = created.ID
	}
	return c.reconcileCreatedRecord(ctx, normalized, beforeIDs, preferredID, writeErr)
}

func (c *Client) createRecordOnce(ctx context.Context, record CreateRecordRequest, mode RecordWriteMode) (*Record, error) {
	segments := []string{"zones", "records", "add", record.Domain, record.Type}
	if mode == RecordWriteModeAsynchronous {
		segments = []string{"zones", "async", "ux", "records", "add", record.Domain, record.Type}
	}

	body := map[string]any{
		"host":  record.Host,
		"rdata": record.Rdata,
	}
	if mode == RecordWriteModeAsynchronous {
		body["type"] = record.Type
	}
	if record.TTL > 0 {
		body["ttl"] = record.TTL
	}
	if record.Prio > 0 {
		body["prio"] = record.Prio
	}
	if record.GeozoneID > 0 {
		body["geozone_id"] = record.GeozoneID
	}

	var response apiRecordResponse
	if err := c.doJSON(ctx, http.MethodPut, c.endpoint(segments...), body, &response, requestOptions{}); err != nil {
		return nil, err
	}
	if response.Data == nil {
		return nil, markAmbiguousWrite(http.MethodPut, requestOptions{}, ErrEmptyResponse)
	}
	result := response.Data.model()
	return &result, nil
}

func (c *Client) GetRecords(ctx context.Context, domain string) ([]Record, error) {
	normalized, err := NormalizeDomain(domain)
	if err != nil {
		return nil, err
	}
	return c.listRecords(ctx, c.endpoint("zones", "records", "all", normalized))
}

func (c *Client) SearchRecords(ctx context.Context, domain, keyword string) ([]Record, error) {
	normalized, err := NormalizeDomain(domain)
	if err != nil {
		return nil, err
	}
	if keyword == "" {
		return nil, fmt.Errorf("record search keyword cannot be empty")
	}
	return c.listRecords(ctx, c.endpoint("zones", "records", "all", normalized, "search", keyword))
}

func (c *Client) listRecords(ctx context.Context, endpoint *url.URL) ([]Record, error) {
	const maximumPages = 1000
	allRecords := make([]Record, 0)
	start := int64(0)
	pageSize := int64(100)

	for page := 0; page < maximumPages; page++ {
		pageEndpoint := *endpoint
		if page > 0 {
			query := url.Values{}
			query.Set("start", strconv.FormatInt(start, 10))
			query.Set("max", strconv.FormatInt(pageSize, 10))
			pageEndpoint.RawQuery = query.Encode()
		}

		var response apiRecordsListResponse
		if err := c.doJSON(ctx, http.MethodGet, &pageEndpoint, nil, &response, requestOptions{safeToRetry: true}); err != nil {
			return nil, err
		}

		for _, apiRecord := range response.Data {
			allRecords = append(allRecords, apiRecord.model())
		}

		if !response.Total.Set || int64(len(allRecords)) >= response.Total.Value {
			sort.SliceStable(allRecords, func(left, right int) bool {
				return lessRecordID(allRecords[left].ID, allRecords[right].ID)
			})
			return allRecords, nil
		}
		count := response.Count.Value
		if !response.Count.Set || count <= 0 {
			count = int64(len(response.Data))
		}
		if count <= 0 {
			return nil, fmt.Errorf("EasyDNS records pagination made no progress at start %d", start)
		}
		if response.Max.Set && response.Max.Value > 0 {
			pageSize = response.Max.Value
		}
		if response.Start.Set {
			start = response.Start.Value + count
		} else {
			start += count
		}
	}

	return nil, fmt.Errorf("EasyDNS records pagination exceeded %d pages", maximumPages)
}

func (c *Client) GetRecord(ctx context.Context, domain, recordID string) (*Record, error) {
	records, err := c.GetRecords(ctx, domain)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if record.ID == recordID {
			result := record
			return &result, nil
		}
	}
	return nil, &NotFoundError{Resource: "record", ID: recordID}
}

func (c *Client) UpdateRecord(ctx context.Context, recordID string, record CreateRecordRequest) (*Record, error) {
	return c.UpdateRecordWithMode(ctx, recordID, record, c.recordWriteMode)
}

func (c *Client) UpdateRecordWithMode(ctx context.Context, recordID string, record CreateRecordRequest, mode RecordWriteMode) (*Record, error) {
	if err := validateRecordWriteMode(mode); err != nil {
		return nil, err
	}
	normalized, err := normalizeCreateRecordRequest(record)
	if err != nil {
		return nil, fmt.Errorf("normalize record update request: %w", err)
	}
	_, writeErr := c.updateRecordOnce(ctx, recordID, normalized, mode)
	if writeErr != nil && !IsAmbiguousWrite(writeErr) {
		return nil, writeErr
	}
	return c.reconcileUpdatedRecord(ctx, recordID, normalized, writeErr)
}

func (c *Client) updateRecordOnce(ctx context.Context, recordID string, record CreateRecordRequest, mode RecordWriteMode) (*Record, error) {
	segments := []string{"zones", "records", recordID}
	if mode == RecordWriteModeAsynchronous {
		segments = []string{"zones", "async", "ux", "records", recordID}
	}

	body := map[string]any{
		"host":  record.Host,
		"rdata": record.Rdata,
		"type":  record.Type,
		"prio":  record.Prio,
	}
	if record.Domain != "" {
		body["domain"] = record.Domain
	}
	if record.TTL > 0 {
		body["ttl"] = record.TTL
	}
	if record.GeozoneID > 0 {
		body["geozone_id"] = record.GeozoneID
	}

	var response apiRecordResponse
	if err := c.doJSON(ctx, http.MethodPost, c.endpoint(segments...), body, &response, requestOptions{}); err != nil {
		return nil, err
	}
	if response.Data == nil {
		return nil, markAmbiguousWrite(http.MethodPost, requestOptions{}, ErrEmptyResponse)
	}
	result := response.Data.model()
	return &result, nil
}

func (c *Client) DeleteRecord(ctx context.Context, domain, recordID string) error {
	return c.DeleteRecordWithMode(ctx, domain, recordID, c.recordWriteMode)
}

func (c *Client) DeleteRecordWithMode(ctx context.Context, domain, recordID string, mode RecordWriteMode) error {
	if err := validateRecordWriteMode(mode); err != nil {
		return err
	}
	normalized, err := NormalizeDomain(domain)
	if err != nil {
		return err
	}
	writeErr := c.deleteRecordOnce(ctx, normalized, recordID, mode)
	if IsNotFound(writeErr) {
		return nil
	}
	if writeErr != nil && !IsAmbiguousWrite(writeErr) {
		return writeErr
	}
	return c.reconcileDeletedRecord(ctx, normalized, recordID, writeErr)
}

func (c *Client) deleteRecordOnce(ctx context.Context, domain, recordID string, mode RecordWriteMode) error {
	segments := []string{"zones", "records", domain, recordID}
	if mode == RecordWriteModeAsynchronous {
		segments = []string{"zones", "async", "ux", "records", domain, recordID}
	}
	return c.doJSON(ctx, http.MethodDelete, c.endpoint(segments...), nil, nil, requestOptions{})
}

func validateRecordWriteMode(mode RecordWriteMode) error {
	if mode != RecordWriteModeSynchronous && mode != RecordWriteModeAsynchronous {
		return fmt.Errorf("invalid record write mode %q", mode)
	}
	return nil
}
