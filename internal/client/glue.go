package client

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"sort"
)

// GlueRecord is a registry glue record: a nameserver hostname inside a domain
// plus the addresses the registry publishes for it.
type GlueRecord struct {
	Domain      string
	Host        string
	IPv4        string
	IPv6        string
	RegistryHas bool
}

type apiGlueItem struct {
	Name      string `json:"name"`
	Domain    string `json:"domain"`
	IPAddress string `json:"ipaddress"`
	IPv6      string `json:"ipv6"`
}

type apiGlueListResponse struct {
	Status flexibleInt64 `json:"status"`
	TM     flexibleInt64 `json:"tm"`
	Msg    string        `json:"msg"`
	Data   struct {
		Domain      string                 `json:"domain"`
		Total       flexibleInt64          `json:"total"`
		GlueRecords oneOrMany[apiGlueItem] `json:"glue_records"`
	} `json:"data"`
}

type apiGlueMutationResponse struct {
	Status flexibleInt64 `json:"status"`
	TM     flexibleInt64 `json:"tm"`
	Msg    string        `json:"msg"`
	Data   *apiGlueItem  `json:"data"`
}

type apiGlueStatusResponse struct {
	Status flexibleInt64 `json:"status"`
	TM     flexibleInt64 `json:"tm"`
	Msg    string        `json:"msg"`
	Data   struct {
		Domain string        `json:"domain"`
		FQDN   string        `json:"fqdn"`
		Exists flexibleInt64 `json:"exists"`
	} `json:"data"`
}

// The create/update response names the host as nameserver_name while the list
// response names it name; decoding accepts either.
func (item *apiGlueItem) UnmarshalJSON(data []byte) error {
	var wire struct {
		Name           string `json:"name"`
		NameserverName string `json:"nameserver_name"`
		Domain         string `json:"domain"`
		IPAddress      string `json:"ipaddress"`
		IPv6           string `json:"ipv6"`
	}
	if err := decodeJSON(data, &wire); err != nil {
		return err
	}
	item.Name = wire.Name
	if item.Name == "" {
		item.Name = wire.NameserverName
	}
	item.Domain = wire.Domain
	item.IPAddress = wire.IPAddress
	item.IPv6 = wire.IPv6
	return nil
}

func (item apiGlueItem) model(fallbackDomain string) (GlueRecord, error) {
	host, err := NormalizeDomain(item.Name)
	if err != nil {
		return GlueRecord{}, fmt.Errorf("invalid glue host %q: %w", item.Name, err)
	}
	domain := item.Domain
	if domain == "" {
		domain = fallbackDomain
	}
	normalizedDomain, err := NormalizeDomain(domain)
	if err != nil {
		return GlueRecord{}, fmt.Errorf("invalid glue domain %q: %w", domain, err)
	}
	record := GlueRecord{Domain: normalizedDomain, Host: host}
	if item.IPAddress != "" {
		address, err := normalizeIPv4(item.IPAddress)
		if err != nil {
			return GlueRecord{}, err
		}
		record.IPv4 = address
	}
	if item.IPv6 != "" {
		address, err := normalizeIPv6(item.IPv6)
		if err != nil {
			return GlueRecord{}, err
		}
		record.IPv6 = address
	}
	return record, nil
}

func normalizeIPv4(value string) (string, error) {
	address, err := netip.ParseAddr(value)
	if err != nil || !address.Is4() {
		return "", fmt.Errorf("invalid glue IPv4 address %q", value)
	}
	return address.String(), nil
}

func normalizeIPv6(value string) (string, error) {
	address, err := netip.ParseAddr(value)
	if err != nil || !address.Is6() || address.Is4In6() {
		return "", fmt.Errorf("invalid glue IPv6 address %q", value)
	}
	return address.String(), nil
}

// ListGlueRecords returns every glue record for a domain, sorted by host.
func (c *Client) ListGlueRecords(ctx context.Context, domain string) ([]GlueRecord, error) {
	normalized, err := NormalizeDomain(domain)
	if err != nil {
		return nil, err
	}
	var response apiGlueListResponse
	if err := c.doJSON(ctx, http.MethodGet, c.endpoint("domains", "glue", normalized), nil, &response, requestOptions{safeToRetry: true}); err != nil {
		return nil, err
	}
	records := make([]GlueRecord, 0, len(response.Data.GlueRecords))
	for _, item := range response.Data.GlueRecords {
		record, err := item.model(normalized)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	sort.SliceStable(records, func(left, right int) bool { return records[left].Host < records[right].Host })
	return records, nil
}

// GetGlueRecord selects one host from the domain's glue collection.
func (c *Client) GetGlueRecord(ctx context.Context, domain, host string) (*GlueRecord, error) {
	normalizedHost, err := NormalizeDomain(host)
	if err != nil {
		return nil, fmt.Errorf("invalid glue host %q: %w", host, err)
	}
	records, err := c.ListGlueRecords(ctx, domain)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if record.Host == normalizedHost {
			result := record
			return &result, nil
		}
	}
	return nil, &NotFoundError{Resource: "glue record", ID: normalizedHost}
}

// GlueRecordRequest is the desired address set for one glue host. At least one
// address family must be present.
type GlueRecordRequest struct {
	Domain string
	Host   string
	IPv4   string
	IPv6   string
}

func (request GlueRecordRequest) normalize() (GlueRecordRequest, map[string]any, error) {
	domain, err := NormalizeDomain(request.Domain)
	if err != nil {
		return GlueRecordRequest{}, nil, err
	}
	host, err := NormalizeDomain(request.Host)
	if err != nil {
		return GlueRecordRequest{}, nil, fmt.Errorf("invalid glue host %q: %w", request.Host, err)
	}
	if request.IPv4 == "" && request.IPv6 == "" {
		return GlueRecordRequest{}, nil, fmt.Errorf("a glue record requires an IPv4 address, an IPv6 address, or both")
	}

	normalized := GlueRecordRequest{Domain: domain, Host: host}
	body := map[string]any{"nameserver_name": host}
	if request.IPv4 != "" {
		address, err := normalizeIPv4(request.IPv4)
		if err != nil {
			return GlueRecordRequest{}, nil, err
		}
		normalized.IPv4 = address
		body["ipaddress"] = address
	}
	if request.IPv6 != "" {
		address, err := normalizeIPv6(request.IPv6)
		if err != nil {
			return GlueRecordRequest{}, nil, err
		}
		normalized.IPv6 = address
		body["ipv6"] = address
	}
	return normalized, body, nil
}

func (c *Client) CreateGlueRecord(ctx context.Context, request GlueRecordRequest) (*GlueRecord, error) {
	return c.writeGlueRecord(ctx, http.MethodPut, "glue creation", request)
}

func (c *Client) UpdateGlueRecord(ctx context.Context, request GlueRecordRequest) (*GlueRecord, error) {
	return c.writeGlueRecord(ctx, http.MethodPost, "glue update", request)
}

func (c *Client) writeGlueRecord(ctx context.Context, method, operation string, request GlueRecordRequest) (*GlueRecord, error) {
	desired, body, err := request.normalize()
	if err != nil {
		return nil, err
	}

	var response apiGlueMutationResponse
	writeErr := c.doJSON(ctx, method, c.endpoint("domains", "glue", desired.Domain), body, &response, requestOptions{})
	if writeErr != nil && !IsAmbiguousWrite(writeErr) {
		return nil, writeErr
	}
	return c.reconcileGlueRecord(ctx, operation, desired, writeErr)
}

// reconcileGlueRecord polls the domain's glue collection until the host shows
// the requested addresses. The mutation is issued exactly once.
func (c *Client) reconcileGlueRecord(ctx context.Context, operation string, desired GlueRecordRequest, writeErr error) (*GlueRecord, error) {
	deadline := c.clock.Now().Add(c.recordReconcileTimeout)
	for {
		record, err := c.GetGlueRecord(ctx, desired.Domain, desired.Host)
		if err == nil && record.IPv4 == desired.IPv4 && record.IPv6 == desired.IPv6 {
			return record, nil
		}
		if err != nil && !IsNotFound(err) {
			return nil, fmt.Errorf("reconcile %s: %w", operation, err)
		}
		if err := c.waitForRecordPoll(ctx, deadline); err != nil {
			return nil, reconciliationError(operation, c.recordReconcileTimeout, writeErr, err)
		}
	}
}

// DeleteGlueRecord removes a glue host. The registry refuses deletion while
// any domain in the same TLD still delegates to the host; that refusal is
// surfaced rather than retried.
func (c *Client) DeleteGlueRecord(ctx context.Context, domain, host string) error {
	normalizedDomain, err := NormalizeDomain(domain)
	if err != nil {
		return err
	}
	normalizedHost, err := NormalizeDomain(host)
	if err != nil {
		return fmt.Errorf("invalid glue host %q: %w", host, err)
	}

	writeErr := c.doJSON(ctx, http.MethodDelete, c.endpoint("domains", "glue", normalizedDomain, normalizedHost), nil, nil, requestOptions{})
	if IsNotFound(writeErr) {
		return nil
	}
	if writeErr != nil && !IsAmbiguousWrite(writeErr) {
		return writeErr
	}

	deadline := c.clock.Now().Add(c.recordReconcileTimeout)
	for {
		_, err := c.GetGlueRecord(ctx, normalizedDomain, normalizedHost)
		if IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reconcile glue deletion: %w", err)
		}
		if err := c.waitForRecordPoll(ctx, deadline); err != nil {
			return reconciliationError("glue deletion", c.recordReconcileTimeout, writeErr, err)
		}
	}
}

// CheckRegistryGlue reports whether the registry has published the glue host.
func (c *Client) CheckRegistryGlue(ctx context.Context, domain, host string) (bool, error) {
	normalizedDomain, err := NormalizeDomain(domain)
	if err != nil {
		return false, err
	}
	normalizedHost, err := NormalizeDomain(host)
	if err != nil {
		return false, fmt.Errorf("invalid glue host %q: %w", host, err)
	}
	var response apiGlueStatusResponse
	if err := c.doJSON(ctx, http.MethodGet, c.endpoint("domains", "glue", normalizedDomain, normalizedHost, "status"), nil, &response, requestOptions{safeToRetry: true}); err != nil {
		return false, err
	}
	return response.Data.Exists.Value == 1, nil
}
