package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetZoneHandlesNumericServiceAndFalseExpiry(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/domain/example.invalid" {
			t.Errorf("request=%s %s", request.Method, request.URL.Path)
		}
		_, _ = response.Write([]byte(`{"msg":"OK","tm":1788307200,"data":{"id":"example.invalid","domain":"example.invalid","exists":"Y","onsystem":"N","expiry":false,"next_due":"2029-12-01","service":3857},"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	zone, err := client.GetZone(context.Background(), "example.invalid")
	if err != nil {
		t.Fatalf("GetZone: %v", err)
	}
	if zone.ID != "example.invalid" || !zone.Exists || zone.OnSystem || zone.Expiry != "" || zone.Service != "3857" {
		t.Fatalf("zone=%+v", zone)
	}
}

func TestGetZoneRetriesTransientRead(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			response.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = response.Write([]byte(`{"data":{"id":"example.invalid","domain":"example.invalid","exists":"Y","onsystem":"Y","expiry":"2030-01-01","next_due":"2029-12-01","service":"3857"},"status":200}`))
	}))
	defer server.Close()

	fake := newFakeTime()
	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, func(config *Config) {
		config.Clock = fake
		config.Waiter = fake
		config.RetryPolicy.MaxAttempts = 2
	})
	if _, err := client.GetZone(context.Background(), "example.invalid"); err != nil {
		t.Fatalf("GetZone: %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests=%d", requests)
	}
}
