package client

import "fmt"

// A successful HTTP status alone cannot establish that a resource is absent.
// Empty collections are valid; omitted/null data and incomplete entries are not.
func validateReadEnvelope(status flexibleInt64, dataPresent bool) error {
	if !status.Set || status.Value < 200 || status.Value >= 300 || !dataPresent {
		return fmt.Errorf("invalid EasyDNS response: expected a success status and explicit data")
	}
	return nil
}

func validateResponseDomain(value, expected string) error {
	normalized, err := NormalizeDomain(value)
	if err != nil || normalized != expected {
		return fmt.Errorf("invalid EasyDNS response: missing or mismatched domain identity")
	}
	return nil
}
