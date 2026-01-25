//go:build integration

package provider

import (
	"fmt"
	"os"
	"testing"
)

// Run with: EASYDNS_API_TOKEN=xxx EASYDNS_API_KEY=xxx go test -tags=integration -v ./internal/provider -run TestIntegration

func getTestClient(t *testing.T) *Client {
	token := os.Getenv("EASYDNS_API_TOKEN")
	key := os.Getenv("EASYDNS_API_KEY")

	if token == "" || key == "" {
		t.Skip("EASYDNS_API_TOKEN and EASYDNS_API_KEY must be set")
	}

	// Use sandbox by default, set EASYDNS_ENVIRONMENT=production for prod
	baseURL := "https://sandbox.rest.easydns.net"
	if os.Getenv("EASYDNS_ENVIRONMENT") == "production" {
		baseURL = "https://rest.easydns.net"
	}

	// Check for async API setting
	useAsyncAPI := os.Getenv("EASYDNS_USE_ASYNC_API") == "true" || os.Getenv("EASYDNS_USE_ASYNC_API") == "1"

	return NewClient(baseURL, token, key, useAsyncAPI)
}

func TestIntegrationGetZone(t *testing.T) {
	client := getTestClient(t)

	// Test getting zone info - you'll need a domain you own
	domain := os.Getenv("EASYDNS_TEST_DOMAIN")
	if domain == "" {
		t.Skip("EASYDNS_TEST_DOMAIN must be set")
	}

	zone, err := client.GetZone(domain)
	if err != nil {
		t.Fatalf("GetZone failed: %v", err)
	}

	fmt.Printf("Zone info for %s:\n", domain)
	fmt.Printf("  ID:       %s\n", zone.ID)
	fmt.Printf("  Domain:   %s\n", zone.Domain)
	fmt.Printf("  Exists:   %v\n", zone.Exists)
	fmt.Printf("  OnSystem: %v\n", zone.OnSystem)
	fmt.Printf("  Expiry:   %s\n", zone.Expiry)
	fmt.Printf("  Service:  %s\n", zone.Service)
}

func TestIntegrationGetRecords(t *testing.T) {
	client := getTestClient(t)

	domain := os.Getenv("EASYDNS_TEST_DOMAIN")
	if domain == "" {
		t.Skip("EASYDNS_TEST_DOMAIN must be set")
	}

	records, err := client.GetRecords(domain)
	if err != nil {
		t.Fatalf("GetRecords failed: %v", err)
	}

	fmt.Printf("Found %d records for %s:\n", len(records), domain)
	for _, r := range records {
		fmt.Printf("  [%s] %s.%s -> %s (TTL: %d)\n", r.Type, r.Host, r.Domain, r.Rdata, r.TTL)
	}
}

func TestIntegrationRecordCRUD(t *testing.T) {
	client := getTestClient(t)

	domain := os.Getenv("EASYDNS_TEST_DOMAIN")
	if domain == "" {
		t.Skip("EASYDNS_TEST_DOMAIN must be set")
	}

	// Create a test record
	createReq := CreateRecordRequest{
		Domain: domain,
		Host:   "tftest",
		Type:   "A",
		Rdata:  "192.0.2.123",
		TTL:    300,
	}

	fmt.Println("Creating test record...")
	record, err := client.CreateRecord(createReq)
	if err != nil {
		t.Fatalf("CreateRecord failed: %v", err)
	}
	fmt.Printf("Created record: ID=%s, %s.%s -> %s\n", record.ID, record.Host, record.Domain, record.Rdata)

	// Update the record
	updateReq := CreateRecordRequest{
		Domain: domain,
		Host:   "tftest",
		Type:   "A",
		Rdata:  "192.0.2.124",
		TTL:    600,
	}

	fmt.Println("Updating test record...")
	updated, err := client.UpdateRecord(record.ID, updateReq)
	if err != nil {
		t.Fatalf("UpdateRecord failed: %v", err)
	}
	fmt.Printf("Updated record: %s.%s -> %s (TTL: %d)\n", updated.Host, updated.Domain, updated.Rdata, updated.TTL)

	// Delete the record
	fmt.Println("Deleting test record...")
	err = client.DeleteRecord(domain, record.ID)
	if err != nil {
		t.Fatalf("DeleteRecord failed: %v", err)
	}
	fmt.Println("Record deleted successfully")
}
