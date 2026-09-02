package client

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewValidatesConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{name: "missing scheme", config: Config{BaseURL: "example.invalid"}, want: "scheme"},
		{name: "missing host", config: Config{BaseURL: "https:///path"}, want: "host"},
		{name: "credentials in URL", config: Config{BaseURL: "https://user:pass@example.invalid"}, want: "must not include credentials"},
		{name: "query in URL", config: Config{BaseURL: "https://example.invalid?x=1"}, want: "query or fragment"},
		{name: "invalid mode", config: Config{BaseURL: "https://example.invalid", RecordWriteMode: "later"}, want: "write mode"},
		{name: "negative timeout", config: Config{BaseURL: "https://example.invalid", HTTPTimeout: -1}, want: "timeout"},
		{name: "negative request interval", config: Config{BaseURL: "https://example.invalid", RequestInterval: -1}, want: "request interval"},
		{name: "negative record poll interval", config: Config{BaseURL: "https://example.invalid", RecordPollInterval: -1}, want: "record poll interval"},
		{name: "negative record reconciliation timeout", config: Config{BaseURL: "https://example.invalid", RecordReconcileTimeout: -1}, want: "reconciliation timeout"},
		{name: "invalid attempts", config: Config{BaseURL: "https://example.invalid", RetryPolicy: RetryPolicy{MaxAttempts: -1}}, want: "attempts"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := New(test.config)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("New() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestNewAppliesSafeDefaultsWithoutMutatingProvidedHTTPClient(t *testing.T) {
	t.Parallel()

	provided := &http.Client{}
	client, err := New(Config{BaseURL: "https://example.invalid/api/", HTTPClient: provided})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if provided.Timeout != 0 {
		t.Fatalf("provided HTTP client was mutated: timeout=%s", provided.Timeout)
	}
	if client.httpClient.Timeout != DefaultHTTPTimeout {
		t.Fatalf("client timeout=%s, want %s", client.httpClient.Timeout, DefaultHTTPTimeout)
	}
	if client.requestInterval != DefaultRequestInterval {
		t.Fatalf("request interval=%s, want %s", client.requestInterval, DefaultRequestInterval)
	}
	if client.recordWriteMode != RecordWriteModeSynchronous {
		t.Fatalf("mode=%q, want synchronous", client.recordWriteMode)
	}
	if client.RecordWriteMode() != RecordWriteModeSynchronous || client.recordPollInterval != DefaultRecordPollInterval || client.recordReconcileTimeout != DefaultRecordReconcileTimeout {
		t.Fatalf("record lifecycle defaults mode=%q poll=%s timeout=%s", client.RecordWriteMode(), client.recordPollInterval, client.recordReconcileTimeout)
	}
	if got := client.endpoint("domain", "example.invalid").String(); got != "https://example.invalid/api/domain/example.invalid" {
		t.Fatalf("endpoint=%q", got)
	}
}

func TestEndpointEscapesEachPathSegment(t *testing.T) {
	t.Parallel()

	client, err := New(Config{BaseURL: "https://example.invalid"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	got := client.endpoint("domain", "example.invalid/../../other").String()
	want := "https://example.invalid/domain/example.invalid%2F..%2F..%2Fother"
	if got != want {
		t.Fatalf("endpoint=%q, want %q", got, want)
	}
}

func TestCustomHTTPTimeoutIsApplied(t *testing.T) {
	t.Parallel()

	client, err := New(Config{BaseURL: "https://example.invalid", HTTPTimeout: 12 * time.Second})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if client.httpClient.Timeout != 12*time.Second {
		t.Fatalf("timeout=%s", client.httpClient.Timeout)
	}
}

func TestRealClockAndWaiter(t *testing.T) {
	t.Parallel()

	if (realClock{}).Now().IsZero() {
		t.Fatal("real clock returned zero time")
	}
	waiter := realWaiter{}
	if err := waiter.Wait(context.Background(), 0); err != nil {
		t.Fatalf("zero wait: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waiter.Wait(ctx, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled wait=%v", err)
	}
}
