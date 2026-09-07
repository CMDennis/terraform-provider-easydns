package client

import (
	"context"
	"fmt"
	"net/http"
)

type Zone struct {
	ID       string
	Domain   string
	Exists   bool
	OnSystem bool
	Expiry   string
	NextDue  string
	Service  string
}

type apiZone struct {
	ID       flexibleString `json:"id"`
	Domain   string         `json:"domain"`
	Exists   flexibleBool   `json:"exists"`
	OnSystem flexibleBool   `json:"onsystem"`
	Expiry   nullableString `json:"expiry"`
	NextDue  string         `json:"next_due"`
	Service  flexibleString `json:"service"`
}

type apiZoneResponse struct {
	Msg    string        `json:"msg"`
	TM     flexibleInt64 `json:"tm"`
	Data   *apiZone      `json:"data"`
	Status flexibleInt64 `json:"status"`
}

func (c *Client) GetZone(ctx context.Context, domain string) (*Zone, error) {
	normalized, err := NormalizeDomain(domain)
	if err != nil {
		return nil, err
	}
	var response apiZoneResponse
	if err := c.doJSON(ctx, http.MethodGet, c.endpoint("domain", normalized), nil, &response, requestOptions{safeToRetry: true}); err != nil {
		return nil, err
	}
	if err := validateReadEnvelope(response.Status, response.Data != nil); err != nil {
		return nil, err
	}
	if err := validateResponseDomain(response.Data.Domain, normalized); err != nil {
		return nil, err
	}
	if !response.Data.Exists.Set || !response.Data.OnSystem.Set || response.Data.ID.Value == "" {
		return nil, fmt.Errorf("invalid EasyDNS zone response: missing identity or presence flags")
	}

	zone := &Zone{
		ID:       response.Data.ID.Value,
		Domain:   response.Data.Domain,
		Exists:   response.Data.Exists.Value,
		OnSystem: response.Data.OnSystem.Value,
		Expiry:   response.Data.Expiry.Value,
		NextDue:  response.Data.NextDue,
		Service:  response.Data.Service.Value,
	}
	return zone, nil
}
