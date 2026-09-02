package client

import (
	"context"
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
	Exists   string         `json:"exists"`
	OnSystem string         `json:"onsystem"`
	Expiry   nullableString `json:"expiry"`
	NextDue  string         `json:"next_due"`
	Service  flexibleString `json:"service"`
}

type apiZoneResponse struct {
	Msg    string        `json:"msg"`
	TM     flexibleInt64 `json:"tm"`
	Data   apiZone       `json:"data"`
	Status flexibleInt64 `json:"status"`
}

func (c *Client) GetZone(ctx context.Context, domain string) (*Zone, error) {
	var response apiZoneResponse
	if err := c.doJSON(ctx, http.MethodGet, c.endpoint("domain", domain), nil, &response, requestOptions{safeToRetry: true}); err != nil {
		return nil, err
	}

	zone := &Zone{
		ID:       response.Data.ID.Value,
		Domain:   response.Data.Domain,
		Exists:   response.Data.Exists == "Y",
		OnSystem: response.Data.OnSystem == "Y",
		Expiry:   response.Data.Expiry.Value,
		NextDue:  response.Data.NextDue,
		Service:  response.Data.Service.Value,
	}
	return zone, nil
}
