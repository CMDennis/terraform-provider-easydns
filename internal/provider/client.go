package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client is the EasyDNS API client
type Client struct {
	BaseURL     string
	Token       string
	Key         string
	HTTPClient  *http.Client
	UseAsyncAPI bool
}

// NewClient creates a new EasyDNS API client
func NewClient(baseURL, token, key string, useAsyncAPI bool) *Client {
	return &Client{
		BaseURL:     baseURL,
		Token:       token,
		Key:         key,
		HTTPClient:  &http.Client{},
		UseAsyncAPI: useAsyncAPI,
	}
}

// Record represents a DNS record in EasyDNS
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

// apiRecord is the raw API response structure (fields are strings/nullable)
// Note: EasyDNS API is inconsistent - some fields come as strings, some as numbers, some as null
type apiRecord struct {
	ID        string      `json:"id,omitempty"`
	Domain    string      `json:"domain"`
	Host      string      `json:"host"`
	Type      string      `json:"type"`
	Rdata     string      `json:"rdata"`
	TTL       interface{} `json:"ttl,omitempty"`
	Prio      interface{} `json:"prio,omitempty"`
	GeozoneID interface{} `json:"geozone_id,omitempty"`
	LastMod   string      `json:"last_mod,omitempty"`
}

// toInt64 safely converts an interface{} to int64
func toInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int64:
		return val
	case int:
		return int64(val)
	case string:
		var i int64
		fmt.Sscanf(val, "%d", &i)
		return i
	case json.Number:
		i, _ := val.Int64()
		return i
	default:
		return 0
	}
}

// toRecord converts an API record to our internal Record type
func (a *apiRecord) toRecord() Record {
	return Record{
		ID:        a.ID,
		Domain:    a.Domain,
		Host:      a.Host,
		Type:      a.Type,
		Rdata:     a.Rdata,
		TTL:       toInt64(a.TTL),
		Prio:      toInt64(a.Prio),
		GeozoneID: toInt64(a.GeozoneID),
		LastMod:   a.LastMod,
	}
}

// apiRecordResponse is the API response for a single record operation
type apiRecordResponse struct {
	TM     int64     `json:"tm"`
	Data   apiRecord `json:"data"`
	Status int       `json:"status"`
}

// apiRecordsListResponse is the API response for listing records
type apiRecordsListResponse struct {
	TM     int64       `json:"tm"`
	Data   []apiRecord `json:"data"`
	Count  int         `json:"count"`
	Total  int         `json:"total"`
	Start  int         `json:"start"`
	Status int         `json:"status"`
}

// CreateRecordRequest is the request body for creating a record
type CreateRecordRequest struct {
	Domain    string `json:"domain"`
	Host      string `json:"host"`
	Type      string `json:"type"`
	Rdata     string `json:"rdata"`
	TTL       int64  `json:"ttl,omitempty"`
	Prio      int64  `json:"prio,omitempty"`
	GeozoneID int64  `json:"geozone_id,omitempty"`
}

// apiErrorResponse represents an error response from the EasyDNS API
type apiErrorResponse struct {
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) doRequest(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	url := fmt.Sprintf("%s%s", c.BaseURL, path)
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.Token, c.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Also check for error in response body (API sometimes returns 200 with error)
	var errResp apiErrorResponse
	if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != nil {
		return nil, fmt.Errorf("API error (code %d): %s", errResp.Error.Code, errResp.Error.Message)
	}

	return respBody, nil
}

// CreateRecord creates a new DNS record
// Uses async API endpoint if UseAsyncAPI is enabled
func (c *Client) CreateRecord(record CreateRecordRequest) (*Record, error) {
	var path string
	if c.UseAsyncAPI {
		path = fmt.Sprintf("/zones/async/ux/records/add/%s/%s", record.Domain, record.Type)
	} else {
		path = fmt.Sprintf("/zones/records/add/%s/%s", record.Domain, record.Type)
	}

	body := map[string]interface{}{
		"host":  record.Host,
		"rdata": record.Rdata,
	}
	// Async API requires type in the body
	if c.UseAsyncAPI {
		body["type"] = record.Type
	}
	if record.TTL > 0 {
		body["ttl"] = record.TTL
	}
	if record.Prio > 0 {
		body["prio"] = record.Prio
	}

	respBody, err := c.doRequest(http.MethodPut, path, body)
	if err != nil {
		return nil, err
	}

	var response apiRecordResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	result := response.Data.toRecord()
	return &result, nil
}

// GetRecords retrieves all records for a domain
func (c *Client) GetRecords(domain string) ([]Record, error) {
	path := fmt.Sprintf("/zones/records/all/%s", domain)

	respBody, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response apiRecordsListResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	records := make([]Record, len(response.Data))
	for i, apiRec := range response.Data {
		records[i] = apiRec.toRecord()
	}
	return records, nil
}

// GetRecord retrieves a specific record by ID from a domain
func (c *Client) GetRecord(domain, recordID string) (*Record, error) {
	records, err := c.GetRecords(domain)
	if err != nil {
		return nil, err
	}

	for _, record := range records {
		if record.ID == recordID {
			return &record, nil
		}
	}

	return nil, fmt.Errorf("record %s not found in domain %s", recordID, domain)
}

// UpdateRecord updates an existing DNS record
// Uses async API endpoint if UseAsyncAPI is enabled
func (c *Client) UpdateRecord(recordID string, record CreateRecordRequest) (*Record, error) {
	var path string
	if c.UseAsyncAPI {
		path = fmt.Sprintf("/zones/async/ux/records/%s", recordID)
	} else {
		path = fmt.Sprintf("/zones/records/%s", recordID)
	}

	body := map[string]interface{}{
		"host":  record.Host,
		"rdata": record.Rdata,
		"type":  record.Type,
	}
	if record.TTL > 0 {
		body["ttl"] = record.TTL
	}
	if record.Prio >= 0 {
		body["prio"] = record.Prio
	}

	respBody, err := c.doRequest(http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}

	var response apiRecordResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	result := response.Data.toRecord()
	return &result, nil
}

// DeleteRecord deletes a DNS record
// Uses async API endpoint if UseAsyncAPI is enabled
func (c *Client) DeleteRecord(domain, recordID string) error {
	var path string
	if c.UseAsyncAPI {
		path = fmt.Sprintf("/zones/async/ux/records/%s/%s", domain, recordID)
	} else {
		path = fmt.Sprintf("/zones/records/%s/%s", domain, recordID)
	}

	_, err := c.doRequest(http.MethodDelete, path, nil)
	return err
}

// Zone represents a DNS zone in EasyDNS
type Zone struct {
	ID       string
	Domain   string
	Exists   bool
	OnSystem bool
	Expiry   string
	NextDue  string
	Service  string
}

// apiZone is the raw API response structure for zone info
type apiZone struct {
	ID       string      `json:"id"`
	Domain   string      `json:"domain"`
	Exists   string      `json:"exists"`   // "Y" or "N"
	OnSystem string      `json:"onsystem"` // "Y" or "N"
	Expiry   interface{} `json:"expiry"`   // can be string or false
	NextDue  string      `json:"next_due"`
	Service  string      `json:"service"`
}

// apiZoneResponse is the API response for zone info
type apiZoneResponse struct {
	Msg    string  `json:"msg"`
	TM     int64   `json:"tm"`
	Data   apiZone `json:"data"`
	Status int     `json:"status"`
}

// GetZone retrieves information about a zone
func (c *Client) GetZone(domain string) (*Zone, error) {
	path := fmt.Sprintf("/domain/%s", domain)

	respBody, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response apiZoneResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert expiry - can be string date or boolean false
	expiry := ""
	if str, ok := response.Data.Expiry.(string); ok {
		expiry = str
	}

	zone := &Zone{
		ID:       response.Data.ID,
		Domain:   response.Data.Domain,
		Exists:   response.Data.Exists == "Y",
		OnSystem: response.Data.OnSystem == "Y",
		Expiry:   expiry,
		NextDue:  response.Data.NextDue,
		Service:  response.Data.Service,
	}

	return zone, nil
}
