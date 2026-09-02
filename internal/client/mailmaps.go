package client

import (
	"context"
	"fmt"
	"net/http"
	"net/mail"
	"sort"
	"strconv"
	"strings"
)

// Mailmap is one EasyDNS mail-forwarding rule.
type Mailmap struct {
	ID           string
	Domain       string
	Alias        string
	Host         string
	Destinations []string
	Active       bool
	LastModified string
}

// Email returns the fully-qualified source address used by the update path.
func (mailmap Mailmap) Email() string {
	return mailmapEmail(mailmap.Alias, mailmap.Host, mailmap.Domain)
}

type apiMailmapItem struct {
	Active       flexibleBool   `json:"active"`
	Alias        string         `json:"alias"`
	Destination  string         `json:"destination"`
	Domain       string         `json:"domain"`
	Host         string         `json:"host"`
	LastModified string         `json:"last_modified"`
	ID           flexibleString `json:"mailmap_id"`
}

type apiMailmapListResponse struct {
	Status flexibleInt64 `json:"status"`
	TM     flexibleInt64 `json:"tm"`
	Data   struct {
		Domain   string                    `json:"domain"`
		Mailmaps oneOrMany[apiMailmapItem] `json:"mailmaps"`
	} `json:"data"`
}

// ListMailmaps returns every mailmap for a domain in stable numeric-ID order.
func (c *Client) ListMailmaps(ctx context.Context, domain string) ([]Mailmap, error) {
	normalized, err := NormalizeDomain(domain)
	if err != nil {
		return nil, err
	}
	var response apiMailmapListResponse
	if err := c.doJSON(ctx, http.MethodGet, c.endpoint("mail", "maps", normalized), nil, &response, requestOptions{safeToRetry: true}); err != nil {
		return nil, err
	}

	result := make([]Mailmap, 0, len(response.Data.Mailmaps))
	for _, item := range response.Data.Mailmaps {
		mailmap, err := item.model(normalized)
		if err != nil {
			return nil, err
		}
		result = append(result, mailmap)
	}
	sort.SliceStable(result, func(left, right int) bool { return lessRecordID(result[left].ID, result[right].ID) })
	return result, nil
}

func (item apiMailmapItem) model(fallbackDomain string) (Mailmap, error) {
	domain := item.Domain
	if domain == "" {
		domain = fallbackDomain
	}
	normalizedDomain, err := NormalizeDomain(domain)
	if err != nil {
		return Mailmap{}, fmt.Errorf("invalid mailmap domain %q: %w", domain, err)
	}
	host, err := normalizeMailmapHost(item.Host)
	if err != nil {
		return Mailmap{}, err
	}
	alias, err := normalizeReturnedMailmapAlias(item.Alias)
	if err != nil {
		return Mailmap{}, err
	}
	destinations, err := normalizeDestinations(strings.Split(item.Destination, ","))
	if err != nil {
		return Mailmap{}, err
	}
	if item.ID.Value == "" {
		return Mailmap{}, fmt.Errorf("mailmap response omitted mailmap_id")
	}
	return Mailmap{
		ID:           item.ID.Value,
		Domain:       normalizedDomain,
		Alias:        alias,
		Host:         host,
		Destinations: destinations,
		Active:       item.Active.Value,
		LastModified: item.LastModified,
	}, nil
}

func normalizeReturnedMailmapAlias(value string) (string, error) {
	value = strings.TrimSpace(value)
	if at := strings.IndexByte(value, '@'); at >= 0 {
		value = value[:at]
	}
	return normalizeMailmapAlias(value)
}

// GetMailmap selects one immutable API ID from the domain collection.
func (c *Client) GetMailmap(ctx context.Context, domain, id string) (*Mailmap, error) {
	normalizedID, err := normalizePositiveID(id, "mailmap")
	if err != nil {
		return nil, err
	}
	mailmaps, err := c.ListMailmaps(ctx, domain)
	if err != nil {
		return nil, err
	}
	for _, mailmap := range mailmaps {
		if mailmap.ID == normalizedID {
			result := mailmap
			return &result, nil
		}
	}
	return nil, &NotFoundError{Resource: "mailmap", ID: normalizedID}
}

// MailmapRequest is the complete desired configuration for a mailmap.
type MailmapRequest struct {
	Domain       string
	Alias        string
	Host         string
	Destinations []string
	Active       bool
}

func (request MailmapRequest) normalize() (MailmapRequest, map[string]any, error) {
	domain, err := NormalizeDomain(request.Domain)
	if err != nil {
		return MailmapRequest{}, nil, err
	}
	alias, err := normalizeMailmapAlias(request.Alias)
	if err != nil {
		return MailmapRequest{}, nil, err
	}
	host, err := normalizeMailmapHost(request.Host)
	if err != nil {
		return MailmapRequest{}, nil, err
	}
	destinations, err := normalizeDestinations(request.Destinations)
	if err != nil {
		return MailmapRequest{}, nil, err
	}
	normalized := MailmapRequest{Domain: domain, Alias: alias, Host: host, Destinations: destinations, Active: request.Active}
	body := map[string]any{
		"alias":       alias,
		"host":        host,
		"destination": strings.Join(destinations, ", "),
		"active":      boolToInt(request.Active),
	}
	return normalized, body, nil
}

func normalizeMailmapAlias(value string) (string, error) {
	if strings.TrimSpace(value) != value || value == "" || strings.Contains(value, "@") {
		return "", fmt.Errorf("mailmap alias %q must be the non-empty local part without @", value)
	}
	address, err := mail.ParseAddress(value + "@example.invalid")
	if err != nil || address.Address != value+"@example.invalid" {
		return "", fmt.Errorf("invalid mailmap alias %q", value)
	}
	return value, nil
}

func normalizeMailmapHost(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "@" {
		return value, nil
	}
	if value == "" || strings.Contains(value, "@") {
		return "", fmt.Errorf("invalid mailmap host %q", value)
	}
	// NormalizeDomain gives relative multi-label hosts the same IDNA and label
	// validation as every other hostname in the provider.
	return NormalizeDomain(value)
}

func normalizeDestinations(values []string) ([]string, error) {
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		address, err := mail.ParseAddress(value)
		if err != nil || address.Address != value {
			return nil, fmt.Errorf("invalid mailmap destination %q", value)
		}
		unique[value] = struct{}{}
	}
	if len(unique) == 0 {
		return nil, fmt.Errorf("a mailmap requires at least one destination")
	}
	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func mailmapEmail(alias, host, domain string) string {
	mailDomain := domain
	if host != "@" {
		mailDomain = host + "." + domain
	}
	return alias + "@" + mailDomain
}

func mailmapsEquivalent(mailmap Mailmap, desired MailmapRequest) bool {
	return mailmap.Domain == desired.Domain && mailmap.Alias == desired.Alias && mailmap.Host == desired.Host &&
		mailmap.Active == desired.Active && equalStrings(mailmap.Destinations, desired.Destinations)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// CreateMailmap issues exactly one PUT, then discovers the immutable ID by
// comparing the collection before and after the mutation.
func (c *Client) CreateMailmap(ctx context.Context, request MailmapRequest) (*Mailmap, error) {
	desired, body, err := request.normalize()
	if err != nil {
		return nil, err
	}
	before, err := c.ListMailmaps(ctx, desired.Domain)
	if err != nil {
		return nil, fmt.Errorf("snapshot mailmaps before creation: %w", err)
	}
	beforeIDs := make(map[string]struct{}, len(before))
	for _, mailmap := range before {
		beforeIDs[mailmap.ID] = struct{}{}
	}

	writeErr := c.doJSON(ctx, http.MethodPut, c.endpoint("mail", "maps", desired.Domain), body, nil, requestOptions{})
	if writeErr != nil && !IsAmbiguousWrite(writeErr) {
		return nil, writeErr
	}
	return c.reconcileCreatedMailmap(ctx, desired, beforeIDs, writeErr)
}

// UpdateMailmap replaces one mailmap. currentEmail is the fully-qualified
// address from prior state because the API identifies the old value in its
// path while the body contains the replacement.
func (c *Client) UpdateMailmap(ctx context.Context, id, currentEmail string, request MailmapRequest) (*Mailmap, error) {
	desired, body, err := request.normalize()
	if err != nil {
		return nil, err
	}
	normalizedID, err := normalizePositiveID(id, "mailmap")
	if err != nil {
		return nil, err
	}
	address, err := mail.ParseAddress(currentEmail)
	if err != nil || address.Address != currentEmail {
		return nil, fmt.Errorf("invalid current mailmap email %q", currentEmail)
	}

	writeErr := c.doJSON(ctx, http.MethodPost, c.endpoint("mail", "maps", desired.Domain, currentEmail), body, nil, requestOptions{})
	if writeErr != nil && !IsAmbiguousWrite(writeErr) {
		return nil, writeErr
	}
	return c.reconcileUpdatedMailmap(ctx, normalizedID, desired, writeErr)
}

func (c *Client) DeleteMailmap(ctx context.Context, domain, id string) error {
	normalizedDomain, err := NormalizeDomain(domain)
	if err != nil {
		return err
	}
	normalizedID, err := normalizePositiveID(id, "mailmap")
	if err != nil {
		return err
	}
	writeErr := c.doJSON(ctx, http.MethodDelete, c.endpoint("mail", "maps", normalizedDomain, normalizedID), nil, nil, requestOptions{})
	if IsNotFound(writeErr) {
		return nil
	}
	if writeErr != nil && !IsAmbiguousWrite(writeErr) {
		return writeErr
	}
	return c.reconcileDeletedMailmap(ctx, normalizedDomain, normalizedID, writeErr)
}

func normalizePositiveID(value, resource string) (string, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 1 {
		return "", fmt.Errorf("%s ID %q must be a positive integer", resource, value)
	}
	return strconv.FormatInt(parsed, 10), nil
}

func (c *Client) reconcileCreatedMailmap(ctx context.Context, desired MailmapRequest, beforeIDs map[string]struct{}, writeErr error) (*Mailmap, error) {
	deadline := c.clock.Now().Add(c.recordReconcileTimeout)
	for {
		mailmaps, err := c.ListMailmaps(ctx, desired.Domain)
		if err != nil {
			return nil, fmt.Errorf("reconcile mailmap creation: %w", err)
		}
		candidates := make([]Mailmap, 0, 1)
		for _, mailmap := range mailmaps {
			if _, existed := beforeIDs[mailmap.ID]; !existed && mailmapsEquivalent(mailmap, desired) {
				candidates = append(candidates, mailmap)
			}
		}
		if len(candidates) == 1 {
			return &candidates[0], nil
		}
		if len(candidates) > 1 {
			ids := make([]string, len(candidates))
			for index := range candidates {
				ids[index] = candidates[index].ID
			}
			sort.Slice(ids, func(left, right int) bool { return lessRecordID(ids[left], ids[right]) })
			return nil, &DuplicateMailmapCandidatesError{IDs: ids}
		}
		if err := c.waitForRecordPoll(ctx, deadline); err != nil {
			return nil, reconciliationError("mailmap creation", c.recordReconcileTimeout, writeErr, err)
		}
	}
}

func (c *Client) reconcileUpdatedMailmap(ctx context.Context, id string, desired MailmapRequest, writeErr error) (*Mailmap, error) {
	deadline := c.clock.Now().Add(c.recordReconcileTimeout)
	for {
		mailmap, err := c.GetMailmap(ctx, desired.Domain, id)
		if err == nil && mailmapsEquivalent(*mailmap, desired) {
			return mailmap, nil
		}
		if err != nil && !IsNotFound(err) {
			return nil, fmt.Errorf("reconcile mailmap update: %w", err)
		}
		if err := c.waitForRecordPoll(ctx, deadline); err != nil {
			return nil, reconciliationError("mailmap update", c.recordReconcileTimeout, writeErr, err)
		}
	}
}

func (c *Client) reconcileDeletedMailmap(ctx context.Context, domain, id string, writeErr error) error {
	deadline := c.clock.Now().Add(c.recordReconcileTimeout)
	for {
		_, err := c.GetMailmap(ctx, domain, id)
		if IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reconcile mailmap deletion: %w", err)
		}
		if err := c.waitForRecordPoll(ctx, deadline); err != nil {
			return reconciliationError("mailmap deletion", c.recordReconcileTimeout, writeErr, err)
		}
	}
}
