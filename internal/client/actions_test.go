package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestActionEndpointsUseContractPaths(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/zones/reload/example.invalid/force":
			response.WriteHeader(http.StatusOK)
		case request.Method == http.MethodPost && request.URL.Path == "/domains/primary_ns/example.invalid":
			var body map[string]string
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body["master"] != "192.0.2.1" {
				t.Errorf("primary nameserver body=%v error=%v", body, err)
			}
			_, _ = fmt.Fprint(response, `{"status":200,"msg":"OK","data":{"domain":"example.invalid","master":"192.0.2.1"}}`)
		default:
			http.Error(response, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	if err := client.ForceZoneReload(context.Background(), "Example.Invalid."); err != nil {
		t.Fatalf("force reload: %v", err)
	}
	result, err := client.SetPrimaryNameserver(context.Background(), "Example.Invalid.", "192.0.2.1")
	if err != nil || result.Domain != "example.invalid" || result.Master != "192.0.2.1" {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

func TestImperativeActionsAreNeverRetriedAndFailuresAreAmbiguous(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		response.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprint(response, `{"status":503,"msg":"uncertain"}`)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, func(config *Config) {
		config.RetryPolicy.MaxAttempts = 4
	})

	err := client.ForceZoneReload(context.Background(), "example.invalid")
	if attempts.Load() != 1 || !IsAmbiguousWrite(err) {
		t.Fatalf("force attempts=%d error=%T %v", attempts.Load(), err, err)
	}
	attempts.Store(0)
	_, err = client.SetPrimaryNameserver(context.Background(), "example.invalid", "192.0.2.1")
	if attempts.Load() != 1 || !IsAmbiguousWrite(err) {
		t.Fatalf("primary attempts=%d error=%T %v", attempts.Load(), err, err)
	}
}
