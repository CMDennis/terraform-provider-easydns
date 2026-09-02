package client

import (
	"context"
	"fmt"
	"net/http"
	"sort"
)

type apiDomainNameserversResponse struct {
	Status flexibleInt64 `json:"status"`
	TM     flexibleInt64 `json:"tm"`
	Msg    string        `json:"msg"`
	Data   struct {
		Domain      string            `json:"domain"`
		Nameservers oneOrMany[string] `json:"nameservers"`
	} `json:"data"`
}

// normalizeNameservers lower-cases each delegated host, drops one optional
// trailing dot, removes duplicates, and sorts the result. Delegation order
// carries no meaning, so a stable set is the comparable form.
func normalizeNameservers(nameservers []string) ([]string, error) {
	seen := make(map[string]struct{}, len(nameservers))
	normalized := make([]string, 0, len(nameservers))
	for _, nameserver := range nameservers {
		host, err := NormalizeDomain(nameserver)
		if err != nil {
			return nil, fmt.Errorf("invalid nameserver %q: %w", nameserver, err)
		}
		if _, duplicate := seen[host]; duplicate {
			return nil, fmt.Errorf("nameserver %q is listed more than once", host)
		}
		seen[host] = struct{}{}
		normalized = append(normalized, host)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func (c *Client) GetDomainNameservers(ctx context.Context, domain string) ([]string, error) {
	normalized, err := NormalizeDomain(domain)
	if err != nil {
		return nil, err
	}
	var response apiDomainNameserversResponse
	if err := c.doJSON(ctx, http.MethodGet, c.endpoint("domains", "ns", normalized), nil, &response, requestOptions{safeToRetry: true}); err != nil {
		return nil, err
	}
	return normalizeNameservers(response.Data.Nameservers)
}

// SetDomainNameservers replaces the complete delegation set. EasyDNS documents
// a minimum of two and a maximum of ten nameservers.
func (c *Client) SetDomainNameservers(ctx context.Context, domain string, nameservers []string) ([]string, error) {
	normalizedDomain, err := NormalizeDomain(domain)
	if err != nil {
		return nil, err
	}
	desired, err := normalizeNameservers(nameservers)
	if err != nil {
		return nil, err
	}
	if len(desired) < 2 || len(desired) > 10 {
		return nil, fmt.Errorf("EasyDNS requires between 2 and 10 nameservers, got %d", len(desired))
	}

	body := map[string]any{"nameservers": desired}
	var response apiDomainNameserversResponse
	writeErr := c.doJSON(ctx, http.MethodPost, c.endpoint("domains", "ns", normalizedDomain), body, &response, requestOptions{})
	if writeErr != nil && !IsAmbiguousWrite(writeErr) {
		return nil, writeErr
	}
	return c.reconcileNameservers(ctx, normalizedDomain, desired, writeErr)
}

// reconcileNameservers polls until the delegation matches what was requested.
// The update is never replayed.
func (c *Client) reconcileNameservers(ctx context.Context, domain string, desired []string, writeErr error) ([]string, error) {
	deadline := c.clock.Now().Add(c.recordReconcileTimeout)
	for {
		observed, err := c.GetDomainNameservers(ctx, domain)
		if err == nil && equalStringSets(observed, desired) {
			return observed, nil
		}
		if err != nil && !IsNotFound(err) {
			return nil, fmt.Errorf("reconcile domain nameservers: %w", err)
		}
		if err := c.waitForRecordPoll(ctx, deadline); err != nil {
			return nil, reconciliationError("nameserver update", c.recordReconcileTimeout, writeErr, err)
		}
	}
}

// equalStringSets compares two already-normalized, sorted collections.
func equalStringSets(left, right []string) bool {
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
