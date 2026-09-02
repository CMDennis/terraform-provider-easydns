package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// Domain is the EasyDNS view of a domain on the system. Registration-specific
// policy lives in RegistrationStatus; this model stays close to
// getDomainInfo's documented data object.
type Domain struct {
	ID             string
	Domain         string
	Exists         bool
	OnSystem       bool
	Expiry         string
	NextDue        string
	ClonedTo       string
	Service        int64
	SubscriptionID int64
}

type apiDomainInfo struct {
	ID       flexibleString `json:"id"`
	Domain   string         `json:"domain"`
	Exists   string         `json:"exists"`
	OnSystem string         `json:"onsystem"`
	Expiry   nullableString `json:"expiry"`
	NextDue  string         `json:"next_due"`
	ClonedTo nullableString `json:"cloned_to"`
	Service  flexibleInt64  `json:"service"`
	SubBlock flexibleInt64  `json:"sub_block"`
}

type apiDomainInfoResponse struct {
	Msg    string         `json:"msg"`
	TM     flexibleInt64  `json:"tm"`
	Data   *apiDomainInfo `json:"data"`
	Status flexibleInt64  `json:"status"`
}

func (info apiDomainInfo) model(fallbackDomain string) Domain {
	domain := info.Domain
	if domain == "" {
		domain = fallbackDomain
	}
	id := info.ID.Value
	if id == "" {
		id = domain
	}
	return Domain{
		ID:             id,
		Domain:         domain,
		Exists:         isAffirmative(info.Exists),
		OnSystem:       isAffirmative(info.OnSystem),
		Expiry:         info.Expiry.Value,
		NextDue:        info.NextDue,
		ClonedTo:       info.ClonedTo.Value,
		Service:        info.Service.Value,
		SubscriptionID: info.SubBlock.Value,
	}
}

// isAffirmative reads the API's "Y"/"N" flags. EasyDNS also returns these as
// booleans and as 1/0 in places, so all three spellings are accepted.
func isAffirmative(value string) bool {
	switch value {
	case "Y", "y", "yes", "YES", "true", "TRUE", "1":
		return true
	default:
		return false
	}
}

func (c *Client) GetDomain(ctx context.Context, domain string) (*Domain, error) {
	normalized, err := NormalizeDomain(domain)
	if err != nil {
		return nil, err
	}
	var response apiDomainInfoResponse
	if err := c.doJSON(ctx, http.MethodGet, c.endpoint("domain", normalized), nil, &response, requestOptions{safeToRetry: true}); err != nil {
		return nil, err
	}
	if response.Data == nil {
		return nil, &NotFoundError{Resource: "domain", ID: normalized}
	}
	result := response.Data.model(normalized)
	// EasyDNS answers for a domain it does not host with an envelope whose
	// onsystem flag is negative rather than with a 404.
	if !result.OnSystem {
		return nil, &NotFoundError{Resource: "domain", ID: normalized}
	}
	return &result, nil
}

// DomainSummary is one entry of a user's domain list.
type DomainSummary struct {
	Domain string
	Link   string
}

type apiDomainListEntry struct {
	Name string `json:"name"`
	Link string `json:"link"`
}

// apiUserDomainListData decodes the observed shape, in which each domain is a
// numerically keyed sibling of the user field rather than a member of the
// documented index array. Both forms are accepted.
type apiUserDomainListData struct {
	User    string
	Entries []apiDomainListEntry
}

func (data *apiUserDomainListData) UnmarshalJSON(raw []byte) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		*data = apiUserDomainListData{}
		return nil
	}

	var fields map[string]json.RawMessage
	if err := decodeJSON(raw, &fields); err != nil {
		return err
	}

	if user, ok := fields["user"]; ok {
		// The user field is a plain string; ignore it if it is anything else.
		_ = decodeJSON(user, &data.User)
	}

	// Documented form: an index array.
	if index, ok := fields["index"]; ok {
		var entries oneOrMany[apiDomainListEntry]
		if err := decodeJSON(index, &entries); err != nil {
			return err
		}
		data.Entries = entries
		return nil
	}

	// Observed form: numerically keyed siblings. Sort the keys so the result
	// is deterministic before the caller sorts by domain name.
	keys := make([]string, 0, len(fields))
	for key := range fields {
		if key == "user" {
			continue
		}
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(left, right int) bool { return lessRecordID(keys[left], keys[right]) })
	for _, key := range keys {
		value := bytes.TrimSpace(fields[key])
		if len(value) == 0 || value[0] != '{' {
			continue
		}
		var entry apiDomainListEntry
		if err := decodeJSON(value, &entry); err != nil {
			return err
		}
		if entry.Name != "" {
			data.Entries = append(data.Entries, entry)
		}
	}
	return nil
}

type apiUserDomainListResponse struct {
	Status flexibleInt64         `json:"status"`
	TM     flexibleInt64         `json:"tm"`
	Msg    string                `json:"msg"`
	Total  flexibleInt64         `json:"total"`
	Count  flexibleInt64         `json:"count"`
	Data   apiUserDomainListData `json:"data"`
}

// ListUserDomains returns every domain the named user controls, sorted by
// domain name. An empty user selects the authenticated account.
//
// The endpoint takes a real username in the path and rejects a placeholder:
// sandbox observation shows /domains/list/self answering HTTP 400 "Username
// provided does not match provided credentials". An empty user is therefore
// resolved by asking /user who the credentials belong to.
func (c *Client) ListUserDomains(ctx context.Context, user string) (string, []DomainSummary, error) {
	account := strings.TrimSpace(user)
	if account == "" {
		current, err := c.GetCurrentUser(ctx)
		if err != nil {
			return "", nil, fmt.Errorf("resolve the authenticated EasyDNS user: %w", err)
		}
		if current.Username == "" {
			return "", nil, fmt.Errorf("EasyDNS did not report a username for the authenticated account; set the user argument explicitly")
		}
		account = current.Username
	}
	var response apiUserDomainListResponse
	if err := c.doJSON(ctx, http.MethodGet, c.endpoint("domains", "list", account), nil, &response, requestOptions{safeToRetry: true}); err != nil {
		return "", nil, err
	}
	domains := make([]DomainSummary, 0, len(response.Data.Entries))
	for _, entry := range response.Data.Entries {
		if entry.Name == "" {
			continue
		}
		domains = append(domains, DomainSummary{Domain: entry.Name, Link: entry.Link})
	}
	sort.SliceStable(domains, func(left, right int) bool { return domains[left].Domain < domains[right].Domain })
	return response.Data.User, domains, nil
}

// DomainService is the EasyDNS service level a domain is created under.
type DomainService string

const (
	DomainServiceLite       DomainService = "lite"
	DomainServiceDNS        DomainService = "dns"
	DomainServicePro        DomainService = "pro"
	DomainServiceEnterprise DomainService = "enterprise"
)

// DomainCurrency is the billing currency for a domain creation invoice.
type DomainCurrency string

const (
	DomainCurrencyCAD DomainCurrency = "CAD"
	DomainCurrencyUSD DomainCurrency = "USD"
)

// Contact is one registrant, admin, tech, or billing contact. Every field is
// PII and must be treated as sensitive by callers.
type Contact struct {
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	OrgName    string `json:"org_name,omitempty"`
	Address1   string `json:"address1"`
	Address2   string `json:"address2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state"`
	Country    string `json:"country"`
	PostalCode string `json:"postal_code"`
	Phone      string `json:"phone"`
	Email      string `json:"email"`
	// Language and CPR are documented for the owner contact only and are
	// required for .CA registrations.
	Language string `json:"language,omitempty"`
	CPR      string `json:"cpr,omitempty"`
}

type ContactSet struct {
	Owner   *Contact `json:"owner,omitempty"`
	Admin   *Contact `json:"admin,omitempty"`
	Tech    *Contact `json:"tech,omitempty"`
	Billing *Contact `json:"billing,omitempty"`
}

// CreateDomainRequest covers both documented request bodies. DNSOnly selects
// BodyDomainCreateDNSOnly; otherwise the registration body is sent.
type CreateDomainRequest struct {
	Domain       string
	Service      DomainService
	Term         int64
	Currency     DomainCurrency
	DNSOnly      bool
	Premium      bool
	PremiumPrice string
	Nameservers  []string
	DomainGroup  string
	PrimaryNS    string
	Contacts     *ContactSet
	// Extra carries documented TLD-specific registration fields. Keys are
	// passed through unchanged because each TLD defines its own set.
	Extra map[string]string
}

// DomainCreation is the invoice-bearing result of createDomain.
type DomainCreation struct {
	Domain    string
	Term      int64
	Service   int64
	TLD       string
	InvoiceID int64
	Currency  string
	User      string
}

type apiDomainCreateData struct {
	Domain   string         `json:"domain"`
	Term     flexibleInt64  `json:"term"`
	Service  flexibleInt64  `json:"service"`
	TLD      string         `json:"tld"`
	InvID    flexibleInt64  `json:"inv_id"`
	Currency string         `json:"currency"`
	User     flexibleString `json:"user"`
}

type apiDomainCreateResponse struct {
	Msg    string                         `json:"msg"`
	Data   oneOrMany[apiDomainCreateData] `json:"data"`
	Status flexibleInt64                  `json:"status"`
}

// CreateDomain adds a domain. The caller is responsible for the registration
// and premium opt-ins described in ADR-0003; this method only validates that
// the request it was handed is internally consistent.
func (c *Client) CreateDomain(ctx context.Context, request CreateDomainRequest) (*DomainCreation, error) {
	normalized, err := NormalizeDomain(request.Domain)
	if err != nil {
		return nil, err
	}
	body, err := buildCreateDomainBody(request)
	if err != nil {
		return nil, err
	}
	if !request.DNSOnly && !c.enableDomainRegistration {
		return nil, ErrDomainRegistrationDisabled
	}

	var response apiDomainCreateResponse
	writeErr := c.doJSON(ctx, http.MethodPut, c.endpoint("domains", "add", normalized), body, &response, requestOptions{})
	if writeErr != nil && !IsAmbiguousWrite(writeErr) {
		return nil, writeErr
	}
	if writeErr != nil || len(response.Data) == 0 {
		// The write may have reached EasyDNS. Reconcile by reading rather than
		// replaying a request that can register and bill a domain twice.
		return c.reconcileCreatedDomain(ctx, normalized, writeErr)
	}

	created := response.Data[0]
	result := &DomainCreation{
		Domain:    created.Domain,
		Term:      created.Term.Value,
		Service:   created.Service.Value,
		TLD:       created.TLD,
		InvoiceID: created.InvID.Value,
		Currency:  created.Currency,
		User:      created.User.Value,
	}
	if result.Domain == "" {
		result.Domain = normalized
	}
	return result, nil
}

// reconcileCreatedDomain confirms an ambiguous or bodiless creation by reading
// the domain back. It never re-sends the creation request.
func (c *Client) reconcileCreatedDomain(ctx context.Context, domain string, writeErr error) (*DomainCreation, error) {
	deadline := c.clock.Now().Add(c.recordReconcileTimeout)
	for {
		found, err := c.GetDomain(ctx, domain)
		if err == nil && found.OnSystem {
			return &DomainCreation{Domain: found.Domain, Service: found.Service}, nil
		}
		if err != nil && !IsNotFound(err) {
			return nil, fmt.Errorf("reconcile created domain: %w", err)
		}
		if err := c.waitForRecordPoll(ctx, deadline); err != nil {
			return nil, reconciliationError("domain creation", c.recordReconcileTimeout, writeErr, err)
		}
	}
}

func buildCreateDomainBody(request CreateDomainRequest) (map[string]any, error) {
	switch request.Service {
	case DomainServiceLite, DomainServiceDNS, DomainServicePro, DomainServiceEnterprise:
	default:
		return nil, fmt.Errorf("domain service must be lite, dns, pro, or enterprise, got %q", request.Service)
	}
	switch request.Currency {
	case DomainCurrencyCAD, DomainCurrencyUSD:
	default:
		return nil, fmt.Errorf("domain currency must be CAD or USD, got %q", request.Currency)
	}
	if request.Term < 1 || request.Term > 10 {
		return nil, fmt.Errorf("domain term must be between 1 and 10 years, got %d", request.Term)
	}
	if len(request.Nameservers) > 6 {
		return nil, fmt.Errorf("domain creation accepts at most 6 nameservers, got %d", len(request.Nameservers))
	}

	body := map[string]any{
		"service":  string(request.Service),
		"term":     request.Term,
		"currency": string(request.Currency),
	}
	if len(request.Nameservers) > 0 {
		nameservers, err := normalizeNameservers(request.Nameservers)
		if err != nil {
			return nil, err
		}
		body["nameservers"] = nameservers
	}
	if request.DomainGroup != "" {
		body["domain_group"] = request.DomainGroup
	}
	if request.PrimaryNS != "" {
		body["primary_ns"] = request.PrimaryNS
	}

	if request.DNSOnly {
		if request.Premium || request.PremiumPrice != "" {
			return nil, fmt.Errorf("premium pricing does not apply to a DNS-only domain")
		}
		if request.Contacts != nil {
			return nil, fmt.Errorf("contacts are only used when registering a domain")
		}
		if len(request.Extra) > 0 {
			return nil, fmt.Errorf("TLD registration fields are only used when registering a domain")
		}
		body["dns_only"] = 1
		return body, nil
	}

	body["dns_only"] = 0
	if request.Contacts == nil || request.Contacts.Owner == nil {
		return nil, fmt.Errorf("registering a domain requires at least an owner contact")
	}
	body["contacts"] = request.Contacts
	if request.Premium {
		if request.PremiumPrice == "" {
			return nil, fmt.Errorf("premium registration requires the verified premium price")
		}
		body["premium"] = 1
		body["premium_price"] = request.PremiumPrice
	} else if request.PremiumPrice != "" {
		return nil, fmt.Errorf("premium price was supplied without the premium opt-in")
	}
	if len(request.Extra) > 0 {
		body["extra"] = request.Extra
	}
	return body, nil
}

type apiDomainDeleteResponse struct {
	Msg    string        `json:"msg"`
	Status flexibleInt64 `json:"status"`
}

// DeleteDomain removes a domain from the system. Callers must enforce the
// deletion opt-ins in ADR-0003 before reaching this method.
func (c *Client) DeleteDomain(ctx context.Context, domain string) error {
	normalized, err := NormalizeDomain(domain)
	if err != nil {
		return err
	}
	var response apiDomainDeleteResponse
	writeErr := c.doJSON(ctx, http.MethodDelete, c.endpoint("domain", normalized), nil, &response, requestOptions{})
	if IsNotFound(writeErr) {
		return nil
	}
	if writeErr != nil && !IsAmbiguousWrite(writeErr) {
		return writeErr
	}
	return c.reconcileDeletedDomain(ctx, normalized, writeErr)
}

// reconcileDeletedDomain polls until the domain is no longer on the system.
//
// EasyDNS may answer the delete with HTTP 200 and keep reporting the domain as
// on-system afterwards, in which case this returns a diagnostic rather than
// reporting success. Treating the accepted delete as done would let Terraform
// forget a domain that still exists and still bills.
func (c *Client) reconcileDeletedDomain(ctx context.Context, domain string, writeErr error) error {
	deadline := c.clock.Now().Add(c.recordReconcileTimeout)
	for {
		_, err := c.GetDomain(ctx, domain)
		if IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reconcile deleted domain: %w", err)
		}
		if err := c.waitForRecordPoll(ctx, deadline); err != nil {
			if errors.Is(err, errReconciliationDeadline) && writeErr == nil {
				return &DomainDeletionNotObservableError{Domain: domain, Timeout: c.recordReconcileTimeout}
			}
			return reconciliationError("domain deletion", c.recordReconcileTimeout, writeErr, err)
		}
	}
}
