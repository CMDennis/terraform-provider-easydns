package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
)

// RenewalAction is the documented renewal policy for a domain.
type RenewalAction string

const (
	RenewalRemind RenewalAction = "remind"
	RenewalRenew  RenewalAction = "renew"
	RenewalExpire RenewalAction = "expire"
)

func ParseRenewalAction(value string) (RenewalAction, error) {
	action := RenewalAction(value)
	switch action {
	case RenewalRemind, RenewalRenew, RenewalExpire:
		return action, nil
	default:
		return "", fmt.Errorf("renewal must be remind, renew, or expire, got %q", value)
	}
}

// RegistrationStatus is the reglock and renewal policy for one domain.
type RegistrationStatus struct {
	Domain          string
	Reglock         bool
	Renewal         string
	AutoRenew       bool
	AutoRenewCardID string
	LetExpire       bool
	LetExpireFailed bool
	Expiry          string
	LocalRegistrar  bool
	SupportsReglock bool
	// SupportsReglockReported distinguishes "the TLD cannot reglock" from
	// "EasyDNS did not say", which observed responses omit entirely.
	SupportsReglockReported bool
}

type apiRegStatus struct {
	Domain    string       `json:"domain"`
	Reglock   flexibleBool `json:"reglock"`
	Renewal   string       `json:"renewal"`
	AutoRenew flexibleBool `json:"auto_renew"`
	// EasyDNS returns false, not a string, when no card is on file.
	AutoRenewCardID nullableString `json:"auto_renew_card_id"`
	LetExpire       flexibleBool   `json:"let_expire"`
	LetExpireFailed flexibleBool   `json:"let_expire_failed"`
	Expiry          nullableString `json:"expiry"`
	LocalRegistrar  flexibleBool   `json:"local_registrar"`
	SupportsReglock flexibleBool   `json:"supports_reglock"`
}

func (status apiRegStatus) model(domain string) RegistrationStatus {
	name := status.Domain
	if name == "" {
		name = domain
	}
	return RegistrationStatus{
		Domain:                  name,
		Reglock:                 status.Reglock.Value,
		Renewal:                 status.Renewal,
		AutoRenew:               status.AutoRenew.Value,
		AutoRenewCardID:         status.AutoRenewCardID.Value,
		LetExpire:               status.LetExpire.Value,
		LetExpireFailed:         status.LetExpireFailed.Value,
		Expiry:                  status.Expiry.Value,
		LocalRegistrar:          status.LocalRegistrar.Value,
		SupportsReglock:         status.SupportsReglock.Value,
		SupportsReglockReported: status.SupportsReglock.Set,
	}
}

// regStatusCollection decodes the getRegStatus data object. The pinned
// contract types the response as a single-domain object while the operation
// returns a user's whole list, so all three plausible shapes are accepted: an
// object keyed by domain name, an array of objects that name their own domain,
// and a bare single-domain object.
type regStatusCollection []RegistrationStatus

func (collection *regStatusCollection) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*collection = nil
		return nil
	}

	if data[0] == '[' {
		var entries []apiRegStatus
		if err := decodeJSON(data, &entries); err != nil {
			return err
		}
		statuses := make(regStatusCollection, 0, len(entries))
		for _, entry := range entries {
			statuses = append(statuses, entry.model(""))
		}
		*collection = statuses
		return nil
	}

	if data[0] != '{' {
		return fmt.Errorf("unexpected EasyDNS registration status shape")
	}

	var keyed map[string]json.RawMessage
	if err := decodeJSON(data, &keyed); err != nil {
		return err
	}
	// Observed shape: data is an envelope carrying the domain-keyed map under
	// "domains" alongside scalar siblings such as "user". Descend into it and
	// let the domain-keyed branch below do the work.
	if nested, ok := keyed["domains"]; ok {
		return collection.UnmarshalJSON(nested)
	}
	// A bare single-domain object carries scalar policy fields at its root; a
	// domain-keyed map carries an object per key.
	if _, hasPolicy := keyed["reglock"]; hasPolicy {
		var single apiRegStatus
		if err := decodeJSON(data, &single); err != nil {
			return err
		}
		*collection = regStatusCollection{single.model("")}
		return nil
	}

	statuses := make(regStatusCollection, 0, len(keyed))
	for domain, raw := range keyed {
		// Scalar siblings such as "user" are envelope metadata, not domains.
		if trimmed := bytes.TrimSpace(raw); len(trimmed) == 0 || trimmed[0] != '{' {
			continue
		}
		var entry apiRegStatus
		if err := decodeJSON(raw, &entry); err != nil {
			return err
		}
		statuses = append(statuses, entry.model(domain))
	}
	sort.SliceStable(statuses, func(left, right int) bool { return statuses[left].Domain < statuses[right].Domain })
	*collection = statuses
	return nil
}

type apiRegStatusResponse struct {
	Msg    string              `json:"msg"`
	TM     flexibleInt64       `json:"tm"`
	Data   regStatusCollection `json:"data"`
	Status flexibleInt64       `json:"status"`
}

// ListRegistrationStatuses returns the reglock and renewal policy for every
// domain the authenticated account controls, sorted by domain name.
func (c *Client) ListRegistrationStatuses(ctx context.Context) ([]RegistrationStatus, error) {
	var response apiRegStatusResponse
	if err := c.doJSON(ctx, http.MethodGet, c.endpoint("domains", "regstatus"), nil, &response, requestOptions{safeToRetry: true}); err != nil {
		return nil, err
	}
	statuses := make([]RegistrationStatus, 0, len(response.Data))
	for _, status := range response.Data {
		if status.Domain == "" {
			continue
		}
		normalized, err := NormalizeDomain(status.Domain)
		if err != nil {
			return nil, fmt.Errorf("invalid domain in registration status: %w", err)
		}
		status.Domain = normalized
		statuses = append(statuses, status)
	}
	sort.SliceStable(statuses, func(left, right int) bool { return statuses[left].Domain < statuses[right].Domain })
	return statuses, nil
}

// GetRegistrationStatus selects one domain from the account-wide listing.
func (c *Client) GetRegistrationStatus(ctx context.Context, domain string) (*RegistrationStatus, error) {
	normalized, err := NormalizeDomain(domain)
	if err != nil {
		return nil, err
	}
	statuses, err := c.ListRegistrationStatuses(ctx)
	if err != nil {
		return nil, err
	}
	for _, status := range statuses {
		if status.Domain == normalized {
			result := status
			return &result, nil
		}
	}
	return nil, &NotFoundError{Resource: "registration status", ID: normalized}
}

// RegistrationSettingsRequest is a desired reglock and renewal policy.
type RegistrationSettingsRequest struct {
	Domain  string
	Reglock bool
	Renewal RenewalAction
}

// SetRegistrationSettings applies the policy and reconciles it by reading.
//
// The pinned contract types the request as an array of per-domain objects
// while its own example shows an object keyed by domain. Sandbox observation
// settled it: the array form is refused with HTTP 406 "List of domains to
// modify provided in invalid format", so the example is correct and the
// schema is wrong. This client sends the domain-keyed object.
func (c *Client) SetRegistrationSettings(ctx context.Context, request RegistrationSettingsRequest) (*RegistrationStatus, error) {
	normalized, err := NormalizeDomain(request.Domain)
	if err != nil {
		return nil, err
	}
	renewal, err := ParseRenewalAction(string(request.Renewal))
	if err != nil {
		return nil, err
	}

	body := map[string]any{
		normalized: map[string]any{
			"reglock": request.Reglock,
			"renewal": string(renewal),
		},
	}

	var response apiRegStatusResponse
	writeErr := c.doJSON(ctx, http.MethodPost, c.endpoint("domains", "regstatus"), body, &response, requestOptions{})
	if writeErr != nil && !IsAmbiguousWrite(writeErr) {
		return nil, writeErr
	}

	desired := RegistrationSettingsRequest{Domain: normalized, Reglock: request.Reglock, Renewal: renewal}
	return c.reconcileRegistrationSettings(ctx, desired, writeErr)
}

func (c *Client) reconcileRegistrationSettings(ctx context.Context, desired RegistrationSettingsRequest, writeErr error) (*RegistrationStatus, error) {
	deadline := c.clock.Now().Add(c.recordReconcileTimeout)
	for {
		status, err := c.GetRegistrationStatus(ctx, desired.Domain)
		if err == nil && registrationSettingsMatch(*status, desired) {
			return status, nil
		}
		if err != nil && !IsNotFound(err) {
			return nil, fmt.Errorf("reconcile registration settings: %w", err)
		}
		if err := c.waitForRecordPoll(ctx, deadline); err != nil {
			return nil, reconciliationError("registration settings update", c.recordReconcileTimeout, writeErr, err)
		}
	}
}

// registrationSettingsMatch ignores reglock on a TLD that cannot lock, because
// the remote value never changes there and would otherwise poll forever.
func registrationSettingsMatch(status RegistrationStatus, desired RegistrationSettingsRequest) bool {
	if status.Renewal != string(desired.Renewal) {
		return false
	}
	// Only an explicit "this TLD cannot reglock" excuses the check. EasyDNS
	// omits supports_reglock entirely in observed responses, and treating that
	// silence as unsupported would report an unapplied reglock as success.
	if status.SupportsReglockReported && !status.SupportsReglock {
		return true
	}
	return status.Reglock == desired.Reglock
}
