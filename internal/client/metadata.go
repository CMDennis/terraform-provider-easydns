package client

import (
	"context"
	"fmt"
	"net/http"
	"sort"
)

// User contains the authenticated account metadata returned by /user. Most
// fields are PII and the Terraform data source marks them sensitive.
type User struct {
	Username, FirstName, LastName, Organization                string
	Address1, Address2, Address3, City, State, Country         string
	PostalCode, Currency, Phone, Cellphone, Fax                string
	Email, Email2, NoticesEmail, PublicEmail, AlertsEmail, URL string
	OptOut, Beta                                               int64
}

type apiUser struct {
	User         string        `json:"user"`
	FirstName    string        `json:"first_name"`
	LastName     string        `json:"last_name"`
	Organization string        `json:"org_name"`
	Address1     string        `json:"address1"`
	Address2     string        `json:"address2"`
	Address3     string        `json:"address3"`
	City         string        `json:"city"`
	State        string        `json:"state"`
	Country      string        `json:"country"`
	PostalCode   string        `json:"postal_code"`
	Currency     string        `json:"currency"`
	Phone        string        `json:"phone"`
	Cellphone    string        `json:"cellphone"`
	Fax          string        `json:"fax"`
	Email        string        `json:"email"`
	Email2       string        `json:"email2"`
	NoticesEmail string        `json:"notices_email"`
	PublicEmail  string        `json:"public_email"`
	AlertsEmail  string        `json:"alerts_email"`
	URL          string        `json:"url"`
	OptOut       flexibleInt64 `json:"opt_out"`
	Beta         flexibleInt64 `json:"beta"`
}

type apiUserResponse struct {
	Status flexibleString     `json:"status"`
	Msg    string             `json:"msg"`
	Data   oneOrMany[apiUser] `json:"data"`
}

func (c *Client) GetCurrentUser(ctx context.Context) (*User, error) {
	var response apiUserResponse
	if err := c.doJSON(ctx, http.MethodGet, c.endpoint("user"), nil, &response, requestOptions{safeToRetry: true}); err != nil {
		return nil, err
	}
	if len(response.Data) == 0 {
		return nil, &NotFoundError{Resource: "current user", ID: "self"}
	}
	item := response.Data[0]
	return &User{
		Username: item.User, FirstName: item.FirstName, LastName: item.LastName, Organization: item.Organization,
		Address1: item.Address1, Address2: item.Address2, Address3: item.Address3, City: item.City, State: item.State,
		Country: item.Country, PostalCode: item.PostalCode, Currency: item.Currency, Phone: item.Phone,
		Cellphone: item.Cellphone, Fax: item.Fax, Email: item.Email, Email2: item.Email2, NoticesEmail: item.NoticesEmail,
		PublicEmail: item.PublicEmail, AlertsEmail: item.AlertsEmail, URL: item.URL, OptOut: item.OptOut.Value, Beta: item.Beta.Value,
	}, nil
}

type ServiceDescription struct {
	ServiceID   int64
	Name        string
	Period      int64
	Enterprise  bool
	Description string
}

type apiServiceDescription struct {
	ServiceID   flexibleInt64 `json:"service_id"`
	Name        string        `json:"name"`
	Period      flexibleInt64 `json:"period"`
	Enterprise  flexibleBool  `json:"enterprise"`
	Description string        `json:"description"`
}

type apiServiceResponse struct {
	Status flexibleInt64         `json:"status"`
	TM     flexibleInt64         `json:"tm"`
	Msg    string                `json:"msg"`
	Data   apiServiceDescription `json:"data"`
}

func (c *Client) GetServiceDescription(ctx context.Context, serviceID int64) (*ServiceDescription, error) {
	if serviceID < 1 {
		return nil, fmt.Errorf("service ID must be positive")
	}
	var response apiServiceResponse
	if err := c.doJSON(ctx, http.MethodGet, c.endpoint("services", fmt.Sprint(serviceID), "description"), nil, &response, requestOptions{safeToRetry: true}); err != nil {
		return nil, err
	}
	result := &ServiceDescription{ServiceID: response.Data.ServiceID.Value, Name: response.Data.Name, Period: response.Data.Period.Value, Enterprise: response.Data.Enterprise.Value, Description: response.Data.Description}
	if result.ServiceID == 0 {
		result.ServiceID = serviceID
	}
	return result, nil
}

type SubscriptionServiceDescription struct {
	SubscriptionID int64
	ServiceID      int64
	Name           string
	Period         int64
	Enterprise     bool
	Description    string
	Size           int64
}

type apiSubscriptionDescription struct {
	SubscriptionID flexibleInt64 `json:"subscription_id"`
	ServiceID      flexibleInt64 `json:"service_id"`
	Name           string        `json:"name"`
	Period         flexibleInt64 `json:"period"`
	Enterprise     flexibleBool  `json:"enterprise"`
	Description    string        `json:"description"`
	Size           flexibleInt64 `json:"size"`
}

type apiSubscriptionResponse struct {
	Status flexibleInt64              `json:"status"`
	TM     flexibleInt64              `json:"tm"`
	Msg    string                     `json:"msg"`
	Data   apiSubscriptionDescription `json:"data"`
}

func (c *Client) GetSubscriptionServiceDescription(ctx context.Context, subscriptionID int64) (*SubscriptionServiceDescription, error) {
	if subscriptionID < 1 {
		return nil, fmt.Errorf("subscription ID must be positive")
	}
	var response apiSubscriptionResponse
	if err := c.doJSON(ctx, http.MethodGet, c.endpoint("services", "subscription", fmt.Sprint(subscriptionID), "description"), nil, &response, requestOptions{safeToRetry: true}); err != nil {
		return nil, err
	}
	result := &SubscriptionServiceDescription{
		SubscriptionID: response.Data.SubscriptionID.Value, ServiceID: response.Data.ServiceID.Value,
		Name: response.Data.Name, Period: response.Data.Period.Value, Enterprise: response.Data.Enterprise.Value,
		Description: response.Data.Description, Size: response.Data.Size.Value,
	}
	if result.SubscriptionID == 0 {
		result.SubscriptionID = subscriptionID
	}
	return result, nil
}

type DomainPricingRequest struct {
	Domain  string
	Service string
	MinTerm int64
	MaxTerm int64
}

type DomainPricing struct {
	Domain    string
	Available bool
	TLD       string
	Services  []PricedService
}

type PricedService struct {
	ID, PricePeriod                                                int64
	Name, Code, Currency, Price, PricePeriodName, Tax1, Tax2, Tax3 string
	Premium                                                        bool
}

type apiPricedService struct {
	ID              flexibleInt64  `json:"id"`
	Name            string         `json:"name"`
	Code            string         `json:"code"`
	Currency        string         `json:"currency"`
	Price           flexibleString `json:"price"`
	Premium         flexibleBool   `json:"isPremium"`
	PricePeriod     flexibleInt64  `json:"pricePeriod"`
	PricePeriodName string         `json:"pricePeriodName"`
	Tax1            flexibleString `json:"tax1"`
	Tax2            flexibleString `json:"tax2"`
	Tax3            flexibleString `json:"tax3"`
}

type apiPricingResponse struct {
	Status flexibleInt64 `json:"status"`
	TM     flexibleInt64 `json:"tm"`
	Msg    string        `json:"msg"`
	Data   struct {
		Domain    string                      `json:"domain"`
		Available flexibleBool                `json:"avail"`
		TLD       string                      `json:"tld"`
		Services  oneOrMany[apiPricedService] `json:"services"`
	} `json:"data"`
}

// GetDomainPricing uses POST because the EasyDNS API defines a filter body,
// but the operation is read-only. Explicit read semantics allow bounded retry
// and prevent misleading AmbiguousWriteError values.
func (c *Client) GetDomainPricing(ctx context.Context, request DomainPricingRequest) (*DomainPricing, error) {
	domain, err := NormalizeDomain(request.Domain)
	if err != nil {
		return nil, err
	}
	if request.MinTerm < 0 || request.MaxTerm < 0 {
		return nil, fmt.Errorf("pricing terms cannot be negative")
	}
	if request.MinTerm > 0 && request.MaxTerm > 0 && request.MinTerm > request.MaxTerm {
		return nil, fmt.Errorf("minimum pricing term cannot exceed maximum term")
	}
	body := make(map[string]any)
	if request.Service != "" {
		switch request.Service {
		case "lite", "dns", "pro", "enterprise":
			body["service"] = request.Service
		default:
			return nil, fmt.Errorf("invalid pricing service %q", request.Service)
		}
	}
	if request.MinTerm > 0 {
		body["min_term"] = request.MinTerm
	}
	if request.MaxTerm > 0 {
		body["max_term"] = request.MaxTerm
	}
	var response apiPricingResponse
	if err := c.doJSON(ctx, http.MethodPost, c.endpoint("domains", "service", "check", domain), body, &response,
		requestOptions{safeToRetry: true, semantics: requestSemanticsRead}); err != nil {
		return nil, err
	}
	result := &DomainPricing{Domain: response.Data.Domain, Available: response.Data.Available.Value, TLD: response.Data.TLD}
	if result.Domain == "" {
		result.Domain = domain
	} else {
		normalizedResponseDomain, err := NormalizeDomain(result.Domain)
		if err != nil {
			return nil, fmt.Errorf("invalid pricing response domain %q: %w", result.Domain, err)
		}
		result.Domain = normalizedResponseDomain
	}
	result.Services = make([]PricedService, len(response.Data.Services))
	for index, item := range response.Data.Services {
		result.Services[index] = PricedService{
			ID: item.ID.Value, Name: item.Name, Code: item.Code, Currency: item.Currency, Price: item.Price.Value,
			Premium: item.Premium.Value, PricePeriod: item.PricePeriod.Value, PricePeriodName: item.PricePeriodName,
			Tax1: item.Tax1.Value, Tax2: item.Tax2.Value, Tax3: item.Tax3.Value,
		}
	}
	sort.SliceStable(result.Services, func(left, right int) bool {
		if result.Services[left].ID != result.Services[right].ID {
			return result.Services[left].ID < result.Services[right].ID
		}
		return result.Services[left].PricePeriod < result.Services[right].PricePeriod
	})
	return result, nil
}
