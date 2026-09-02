package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoJSONSetsAuthHeadersAndBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		username, password, ok := request.BasicAuth()
		if !ok || username != "test-token" || password != "test-key" {
			t.Errorf("basic auth=(%q,%q,%v)", username, password, ok)
		}
		if got := request.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept=%q", got)
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type=%q", got)
		}
		if got := request.Header.Get("User-Agent"); got != "phase1-test" {
			t.Errorf("User-Agent=%q", got)
		}
		body, _ := io.ReadAll(request.Body)
		if !bytes.Contains(body, []byte(`"hello":"world"`)) {
			t.Errorf("body=%s", body)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, func(config *Config) {
		config.UserAgent = "phase1-test"
	})
	var output struct {
		OK bool `json:"ok"`
	}
	if err := client.doJSON(context.Background(), http.MethodPost, client.endpoint("test"), map[string]string{"hello": "world"}, &output, requestOptions{}); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if !output.OK {
		t.Fatalf("output=%+v", output)
	}
}

func TestDoJSONParsesErrorEnvelopeOnSuccessStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"error":{"code":"123","message":"bad request"},"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	err := client.doJSON(context.Background(), http.MethodGet, client.endpoint("test"), nil, nil, requestOptions{safeToRetry: true})
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("error=%T %v, want *APIError", err, err)
	}
	if apiError.Code != 123 || apiError.Message != "bad request" {
		t.Fatalf("API error=%+v", apiError)
	}
}

func TestHTTPErrorDoesNotEchoUnstructuredResponseBody(t *testing.T) {
	t.Parallel()

	const secret = "customer-secret-response-data"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusForbidden)
		_, _ = response.Write([]byte(secret))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	err := client.doJSON(context.Background(), http.MethodGet, client.endpoint("test"), nil, nil, requestOptions{safeToRetry: true})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked response body: %v", err)
	}
	if !strings.Contains(err.Error(), "403") {
		t.Fatalf("error=%v", err)
	}
}

func TestHTTPErrorParsesStructuredMessageShape(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusConflict)
		_, _ = response.Write([]byte(`{"status":409,"msg":"record already exists"}`))
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	err := client.doJSON(context.Background(), http.MethodPut, client.endpoint("test"), map[string]bool{"write": true}, nil, requestOptions{})
	if err == nil || !strings.Contains(err.Error(), "record already exists") || IsAmbiguousWrite(err) {
		t.Fatalf("error=%T %v", err, err)
	}
}

func TestSafeRequestRetriesRetryableStatusAndHonorsRetryAfter(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			response.Header().Set("Retry-After", "2")
			response.WriteHeader(420)
			_, _ = response.Write([]byte(`{"error":{"code":420,"message":"Enhance your calm"}}`))
			return
		}
		_, _ = response.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	fake := newFakeTime()
	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, func(config *Config) {
		config.Clock = fake
		config.Waiter = fake
		config.RetryPolicy = RetryPolicy{
			MaxAttempts:  3,
			InitialDelay: time.Second,
			MaxDelay:     5 * time.Second,
			Jitter:       func(delay time.Duration) time.Duration { return delay },
		}
	})
	var output map[string]bool
	if err := client.doJSON(context.Background(), http.MethodGet, client.endpoint("test"), nil, &output, requestOptions{safeToRetry: true}); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if requests.Load() != 2 {
		t.Fatalf("requests=%d, want 2", requests.Load())
	}
	waits := fake.Waits()
	if len(waits) != 1 || waits[0] != 2*time.Second {
		t.Fatalf("waits=%v, want [2s]", waits)
	}
}

func TestWriteRequestDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		response.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, func(config *Config) {
		config.RetryPolicy.MaxAttempts = 4
	})
	err := client.doJSON(context.Background(), http.MethodPost, client.endpoint("test"), map[string]bool{"write": true}, nil, requestOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if requests.Load() != 1 {
		t.Fatalf("requests=%d, write was retried", requests.Load())
	}
}

func TestThrottledWriteIsExplicitFailureNotAmbiguous(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(420)
		_, _ = response.Write([]byte(`{"error":{"code":420,"message":"Enhance your calm"}}`))
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	err := client.doJSON(context.Background(), http.MethodPut, client.endpoint("write"), map[string]bool{"write": true}, nil, requestOptions{})
	if err == nil || IsAmbiguousWrite(err) {
		t.Fatalf("error=%T %v, want explicit non-ambiguous throttling error", err, err)
	}
}

func TestRateLimiterSpacesEveryHTTPAttempt(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{}`))
	}))
	defer server.Close()

	fake := newFakeTime()
	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, func(config *Config) {
		config.DisableRateLimiting = false
		config.RequestInterval = time.Second
		config.Clock = fake
		config.Waiter = fake
	})

	for index := 0; index < 3; index++ {
		if err := client.doJSON(context.Background(), http.MethodGet, client.endpoint("test"), nil, nil, requestOptions{safeToRetry: true}); err != nil {
			t.Fatalf("request %d: %v", index, err)
		}
	}
	waits := fake.Waits()
	if len(waits) != 2 || waits[0] != time.Second || waits[1] != time.Second {
		t.Fatalf("waits=%v, want [1s 1s]", waits)
	}
}

func TestDoJSONHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	httpClient := &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return nil, request.Context().Err()
	})}
	client := newTestClient(t, "https://example.invalid", httpClient, RecordWriteModeSynchronous, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.doJSON(ctx, http.MethodGet, client.endpoint("test"), nil, nil, requestOptions{safeToRetry: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v, want context.Canceled", err)
	}
}

func TestDoJSONLimitsResponseSize(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"too":"large"}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, func(config *Config) {
		config.MaxResponseBodyBytes = 4
	})
	err := client.doJSON(context.Background(), http.MethodGet, client.endpoint("test"), nil, nil, requestOptions{safeToRetry: true})
	var tooLarge *ResponseTooLargeError
	if !errors.As(err, &tooLarge) {
		t.Fatalf("error=%T %v, want ResponseTooLargeError", err, err)
	}
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	if delay, ok := parseRetryAfter("3", now); !ok || delay != 3*time.Second {
		t.Fatalf("seconds=(%s,%v)", delay, ok)
	}
	if delay, ok := parseRetryAfter(now.Add(5*time.Second).Format(http.TimeFormat), now); !ok || delay != 5*time.Second {
		t.Fatalf("date=(%s,%v)", delay, ok)
	}
	if _, ok := parseRetryAfter("invalid", now); ok {
		t.Fatal("invalid Retry-After accepted")
	}
}

func TestSafeRequestRetriesTransportFailure(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	httpClient := &http.Client{Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		if requests.Add(1) == 1 {
			return nil, io.ErrUnexpectedEOF
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
		}, nil
	})}
	fake := newFakeTime()
	client := newTestClient(t, "https://example.invalid", httpClient, RecordWriteModeSynchronous, func(config *Config) {
		config.Clock = fake
		config.Waiter = fake
		config.RetryPolicy.MaxAttempts = 2
	})
	var output map[string]bool
	if err := client.doJSON(context.Background(), http.MethodGet, client.endpoint("test"), nil, &output, requestOptions{safeToRetry: true}); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if requests.Load() != 2 || !output["ok"] {
		t.Fatalf("requests=%d output=%v", requests.Load(), output)
	}
}

func TestWriteTransportFailureIsNotRetried(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	httpClient := &http.Client{Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		requests.Add(1)
		return nil, io.ErrUnexpectedEOF
	})}
	client := newTestClient(t, "https://example.invalid", httpClient, RecordWriteModeSynchronous, func(config *Config) {
		config.RetryPolicy.MaxAttempts = 4
	})
	err := client.doJSON(context.Background(), http.MethodPost, client.endpoint("test"), map[string]bool{"write": true}, nil, requestOptions{})
	if err == nil || requests.Load() != 1 {
		t.Fatalf("error=%v requests=%d", err, requests.Load())
	}
	if !IsAmbiguousWrite(err) || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("error=%T %v, want ambiguous write preserving transport cause", err, err)
	}
}

func TestMutationResponseDecodeFailureIsAmbiguous(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"data":`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	var output map[string]any
	err := client.doJSON(context.Background(), http.MethodPut, client.endpoint("test"), map[string]bool{"write": true}, &output, requestOptions{})
	if !IsAmbiguousWrite(err) {
		t.Fatalf("error=%T %v, want ambiguous write", err, err)
	}
}

func TestDoJSONEmptyAndMalformedSuccessResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "empty", body: "", want: ErrEmptyResponse.Error()},
		{name: "malformed", body: "{", want: "decode EasyDNS response"},
		{name: "status envelope error", body: `{"status":420}`, want: "API code 420"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				_, _ = response.Write([]byte(test.body))
			}))
			defer server.Close()
			client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
			var output map[string]any
			err := client.doJSON(context.Background(), http.MethodGet, client.endpoint("test"), nil, &output, requestOptions{safeToRetry: true})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want containing %q", err, test.want)
			}
		})
	}
}

func TestRetryDelayBackoffClampAndJitter(t *testing.T) {
	t.Parallel()

	client, err := New(Config{
		BaseURL:             "https://example.invalid",
		DisableRateLimiting: true,
		RetryPolicy: RetryPolicy{
			MaxAttempts:  5,
			InitialDelay: time.Second,
			MaxDelay:     3 * time.Second,
			Jitter:       func(delay time.Duration) time.Duration { return delay },
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := client.retryDelay(1, ""); got != time.Second {
		t.Fatalf("attempt 1=%s", got)
	}
	if got := client.retryDelay(4, ""); got != 3*time.Second {
		t.Fatalf("clamped delay=%s", got)
	}
	if got := client.retryDelay(1, "9"); got != 3*time.Second {
		t.Fatalf("Retry-After clamp=%s", got)
	}
	for index := 0; index < 20; index++ {
		got := defaultJitter(time.Second)
		if got < 800*time.Millisecond || got > 1200*time.Millisecond {
			t.Fatalf("jitter=%s", got)
		}
	}
	if defaultJitter(0) != 0 {
		t.Fatal("zero jitter delay was not zero")
	}
}

func TestSanitizeMessageBoundsAndRemovesControls(t *testing.T) {
	t.Parallel()

	message := "  bad\x00message " + strings.Repeat("x", 600)
	got := sanitizeMessage(message)
	if strings.ContainsRune(got, '\x00') || len(got) > 515 || !strings.HasSuffix(got, "...") {
		t.Fatalf("sanitized length=%d value=%q", len(got), got)
	}
}

func TestNewRequestErrorDoesNotExposeURLCredentials(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, "https://example.invalid", &http.Client{}, RecordWriteModeSynchronous, nil)
	badURL := &url.URL{Scheme: ":bad"}
	err := client.doJSON(context.Background(), http.MethodGet, badURL, nil, nil, requestOptions{safeToRetry: true})
	if err == nil || strings.Contains(err.Error(), "test-token") || strings.Contains(err.Error(), "test-key") {
		t.Fatalf("error=%v", err)
	}
}
