package client

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
)

type PrimaryNameserverResult struct {
	Domain string
	Master string
}

type apiPrimaryNameserverResponse struct {
	Status flexibleInt64 `json:"status"`
	Msg    string        `json:"msg"`
	Data   struct {
		Domain string `json:"domain"`
		Master string `json:"master"`
	} `json:"data"`
}

// SetPrimaryNameserver changes a domain to secondary DNS. It is issued once
// and never automatically retried because the endpoint has no refresh model.
func (c *Client) SetPrimaryNameserver(ctx context.Context, domain, master string) (*PrimaryNameserverResult, error) {
	normalizedDomain, err := NormalizeDomain(domain)
	if err != nil {
		return nil, err
	}
	address, err := netip.ParseAddr(master)
	if err != nil {
		return nil, fmt.Errorf("invalid primary nameserver address %q", master)
	}
	var response apiPrimaryNameserverResponse
	err = c.doJSON(ctx, http.MethodPost, c.endpoint("domains", "primary_ns", normalizedDomain), map[string]any{"master": address.String()}, &response,
		requestOptions{semantics: requestSemanticsMutation})
	if err != nil {
		return nil, err
	}
	return &PrimaryNameserverResult{Domain: response.Data.Domain, Master: response.Data.Master}, nil
}

// ForceZoneReload requests immediate zone regeneration. Although the API uses
// GET, this is an imperative side effect and is deliberately never retried.
func (c *Client) ForceZoneReload(ctx context.Context, domain string) error {
	normalizedDomain, err := NormalizeDomain(domain)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, http.MethodGet, c.endpoint("zones", "reload", normalizedDomain, "force"), nil, nil,
		requestOptions{semantics: requestSemanticsMutation})
}
