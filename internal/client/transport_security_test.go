package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAPIEndpointRequiresHTTPSOutsideLiteralLoopback(t *testing.T) {
	for _, endpoint := range []string{
		"http://example.invalid", "http://192.0.2.1", "http://localhost",
		"http://127.0.0.1.example.invalid", "https://synthetic-secret%zz@example.invalid",
	} {
		if _, err := New(Config{BaseURL: endpoint}); err == nil || strings.Contains(err.Error(), "synthetic-secret") {
			t.Fatalf("unsafe endpoint accepted or URL echoed: %v", err)
		}
	}
	for _, endpoint := range []string{"https://example.invalid", "http://127.0.0.1:1234", "http://[::1]:1234"} {
		if _, err := New(Config{BaseURL: endpoint}); err != nil {
			t.Fatalf("valid endpoint rejected: %v", err)
		}
	}
}

func TestRedirectsNeverDispatchASecondRequest(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			for _, location := range []string{"/elsewhere", "http://api.example.invalid/leak", "https://other.invalid/leak"} {
				t.Run(fmt.Sprintf("%d/%s/%s", status, method, location), func(t *testing.T) {
					calls := 0
					provided := &http.Client{
						CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
							t.Error("unsafe caller redirect policy was invoked")
							return nil
						},
						Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
							calls++
							if calls != 1 || request.URL.Host != "api.example.invalid" || request.URL.Scheme != "https" || request.Method != method {
								t.Error("redirect dispatched credentials or changed/replayed a request")
								return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("{}")), Request: request}, nil
							}
							return &http.Response{StatusCode: status, Header: http.Header{"Location": []string{location}}, Body: io.NopCloser(strings.NewReader("{}")), Request: request}, nil
						}),
					}
					client := newTestClient(t, "https://api.example.invalid", provided, RecordWriteModeSynchronous, nil)
					err := client.doJSON(context.Background(), method, client.endpoint("test"), map[string]string{"contact": "synthetic@example.invalid"}, nil, requestOptions{})
					if err == nil || calls != 1 {
						t.Fatalf("calls=%d, expected one rejected redirect; error=%v", calls, err)
					}
				})
			}
		}
	}
}

func TestDiagnosticsRedactBothAPIErrorEnvelopes(t *testing.T) {
	const token, key = "synthetic-token-123456", "synthetic-key-789012"
	encoded := base64.StdEncoding.EncodeToString([]byte(token + ":" + key))
	for _, prefix := range []int{0, 500} {
		message := strings.Repeat("x", prefix) + token + " " + key + " Basic " + encoded
		for _, status := range []int{200, 403, 503} {
			for _, nested := range []bool{false, true} {
				var envelope any = map[string]any{"status": 403, "msg": message}
				if nested {
					envelope = map[string]any{"error": map[string]any{"code": 403, "message": message}}
				}
				body, err := json.Marshal(envelope)
				if err != nil {
					t.Fatal(err)
				}
				client := newTestClient(t, "https://example.invalid", &http.Client{Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
				})}, RecordWriteModeSynchronous, func(config *Config) { config.Token, config.Key = token, key })
				err = client.doJSON(context.Background(), http.MethodGet, client.endpoint("test"), nil, nil, requestOptions{})
				if err == nil {
					t.Fatal("expected API error")
				}
				for _, secret := range []string{token, key, encoded, "synthetic-token"} {
					if strings.Contains(fmt.Sprintf("wrapped: %v", err), secret) {
						t.Fatal("credential appeared in a diagnostic")
					}
				}
				var apiError *APIError
				if !errors.As(err, &apiError) || apiError.Code != 403 {
					t.Fatalf("lost safe API code: %v", err)
				}
			}
		}
	}
}

func TestSensitiveEndpointsSuppressRemotePersonalData(t *testing.T) {
	const personal = "Synthetic Person, synthetic@example.invalid, 123 Example Street"
	for _, endpoint := range [][]string{{"user"}, {"mail", "maps", "example.invalid"}, {"domains", "add", "example.invalid"}, {"domains", "list", "example-user"}, {"domains", "regstatus"}} {
		for _, status := range []int{200, 403} {
			client := newTestClient(t, "https://example.invalid", &http.Client{Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"status":403,"msg":"` + personal + `"}`))}, nil
			})}, RecordWriteModeSynchronous, nil)
			err := client.doJSON(context.Background(), http.MethodGet, client.endpoint(endpoint...), nil, nil, requestOptions{})
			if err == nil || strings.Contains(err.Error(), "Synthetic") || strings.Contains(err.Error(), "@") {
				t.Fatalf("unsafe diagnostic: %v", err)
			}
		}
	}
}

func TestWrappedTransportAndDecodeErrorsDoNotEchoValues(t *testing.T) {
	const privateText = "synthetic-private-response-value"
	for _, decoding := range []bool{false, true} {
		cause := errors.New(privateText)
		client := newTestClient(t, "https://example.invalid", &http.Client{Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			if !decoding {
				return nil, cause
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":200,"data":{"id":"` + privateText + `"}}`))}, nil
		})}, RecordWriteModeSynchronous, nil)
		var output struct {
			Data struct {
				ID flexibleInt64 `json:"id"`
			} `json:"data"`
		}
		err := client.doJSON(context.Background(), http.MethodPut, client.endpoint("test", privateText), nil, &output, requestOptions{})
		if err == nil || strings.Contains(err.Error(), privateText) || !IsAmbiguousWrite(err) {
			t.Fatalf("unsafe or unclassified error: %v", err)
		}
		if !decoding && !errors.Is(err, cause) {
			t.Fatal("transport cause identity was lost")
		}
	}
}
