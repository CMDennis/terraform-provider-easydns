package client

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"
)

type fakeTime struct {
	mu    sync.Mutex
	now   time.Time
	waits []time.Duration
}

func newFakeTime() *fakeTime {
	return &fakeTime{now: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
}

func (fake *fakeTime) Now() time.Time {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return fake.now
}

func (fake *fakeTime) Wait(ctx context.Context, delay time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if delay > 0 {
		fake.waits = append(fake.waits, delay)
		fake.now = fake.now.Add(delay)
	}
	return nil
}

func (fake *fakeTime) Waits() []time.Duration {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return append([]time.Duration(nil), fake.waits...)
}

func newTestClient(t *testing.T, baseURL string, httpClient *http.Client, mode RecordWriteMode, mutate func(*Config)) *Client {
	t.Helper()
	config := Config{
		BaseURL:                baseURL,
		Token:                  "test-token",
		Key:                    "test-key",
		RecordWriteMode:        mode,
		HTTPClient:             httpClient,
		DisableRateLimiting:    true,
		RecordPollInterval:     time.Millisecond,
		RecordReconcileTimeout: 3 * time.Millisecond,
		RetryPolicy: RetryPolicy{
			MaxAttempts:  1,
			InitialDelay: time.Millisecond,
			MaxDelay:     time.Millisecond,
			Jitter:       func(delay time.Duration) time.Duration { return delay },
		},
	}
	if mutate != nil {
		mutate(&config)
	}
	client, err := New(config)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	return client
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
