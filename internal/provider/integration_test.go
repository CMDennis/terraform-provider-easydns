//go:build integration

package provider

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"
)

// Run read-only sandbox tests with:
// TF_ACC=1 EASYDNS_ACC_SANDBOX=1 EASYDNS_API_TOKEN=xxx EASYDNS_API_KEY=xxx \
//   EASYDNS_TEST_DOMAIN=example.invalid go test -tags=integration -v ./internal/provider -run TestIntegration

func getTestClient(t *testing.T) *Client {
	if os.Getenv("TF_ACC") == "" || os.Getenv("EASYDNS_ACC_SANDBOX") != "1" {
		t.Skip("sandbox integration tests require TF_ACC and EASYDNS_ACC_SANDBOX=1")
	}

	token := os.Getenv("EASYDNS_API_TOKEN")
	key := os.Getenv("EASYDNS_API_KEY")

	if token == "" || key == "" {
		t.Skip("EASYDNS_API_TOKEN and EASYDNS_API_KEY must be set")
	}

	baseURL := sandboxURL
	if configuredURL := os.Getenv("EASYDNS_API_URL"); configuredURL != "" {
		baseURL = configuredURL
	}
	baseURL, err := validateAcceptanceBaseURL(baseURL)
	if err != nil {
		t.Fatalf("unsafe acceptance-test configuration: %v", err)
	}

	writeMode := RecordWriteModeSynchronous
	if configuredMode := os.Getenv("EASYDNS_RECORD_WRITE_MODE"); configuredMode != "" {
		writeMode, err = parseRecordWriteMode(configuredMode)
		if err != nil {
			t.Fatalf("unsafe acceptance-test record mode: %v", err)
		}
	}

	client, err := NewClientWithMode(baseURL, token, key, writeMode)
	if err != nil {
		t.Fatalf("configure client: %v", err)
	}
	return client
}

func TestIntegrationGetZone(t *testing.T) {
	client := getTestClient(t)

	// Test getting zone info - you'll need a domain you own
	domain := os.Getenv("EASYDNS_TEST_DOMAIN")
	if domain == "" {
		t.Skip("EASYDNS_TEST_DOMAIN must be set")
	}

	zone, err := client.GetZone(context.Background(), domain)
	if err != nil {
		t.Fatalf("GetZone failed: %v", err)
	}

	if zone.Domain == "" || zone.ID == "" {
		t.Fatalf("GetZone returned an incomplete zone identity")
	}
}

func TestIntegrationGetRecords(t *testing.T) {
	client := getTestClient(t)

	domain := os.Getenv("EASYDNS_TEST_DOMAIN")
	if domain == "" {
		t.Skip("EASYDNS_TEST_DOMAIN must be set")
	}

	records, err := client.GetRecords(context.Background(), domain)
	if err != nil {
		t.Fatalf("GetRecords failed: %v", err)
	}

	for _, record := range records {
		if record.ID == "" {
			t.Fatalf("GetRecords returned a record without an ID")
		}
	}
}

func TestIntegrationRecordCRUD(t *testing.T) {
	if os.Getenv("EASYDNS_ACC_ALLOW_MUTATIONS") != "sandbox-writes-only" {
		t.Skip("record mutation test requires EASYDNS_ACC_ALLOW_MUTATIONS=sandbox-writes-only")
	}

	client := getTestClient(t)

	domain := os.Getenv("EASYDNS_TEST_DOMAIN")
	if domain == "" {
		t.Skip("EASYDNS_TEST_DOMAIN must be set")
	}

	host := "tfacc-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	createReq := CreateRecordRequest{
		Domain: domain,
		Host:   host,
		Type:   "A",
		Rdata:  "192.0.2.123",
		TTL:    300,
	}

	record, err := client.CreateRecord(context.Background(), createReq)
	if err != nil {
		t.Fatalf("CreateRecord failed: %v", err)
	}
	t.Cleanup(func() {
		if err := client.DeleteRecord(context.Background(), domain, record.ID); err != nil && !IsNotFound(err) {
			t.Errorf("cleanup record %s: %v", record.ID, err)
		}
	})

	// Update the record
	updateReq := CreateRecordRequest{
		Domain: domain,
		Host:   host,
		Type:   "A",
		Rdata:  "192.0.2.124",
		TTL:    600,
	}

	updated, err := client.UpdateRecord(context.Background(), record.ID, updateReq)
	if err != nil {
		t.Fatalf("UpdateRecord failed: %v", err)
	}
	if updated.Rdata != updateReq.Rdata || updated.TTL != updateReq.TTL {
		t.Fatalf("updated record did not converge to the requested state")
	}

	// Delete the record
	err = client.DeleteRecord(context.Background(), domain, record.ID)
	if err != nil {
		t.Fatalf("DeleteRecord failed: %v", err)
	}
}
