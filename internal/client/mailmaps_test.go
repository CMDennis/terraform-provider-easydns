package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestMailmapLifecycleUsesContractPathsAndExactlyOneWrite(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	created := false
	deleted := false
	updated := false
	writes := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		response.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/mail/maps/example.invalid":
			items := "[]"
			if created && !deleted {
				alias := "old@example.invalid"
				destination := "z@example.net, a@example.net"
				if updated {
					alias = "new@example.invalid"
					destination = "new@example.net"
				}
				items = fmt.Sprintf(`[{"mailmap_id":"12","domain":"example.invalid","alias":%q,"host":"@","destination":%q,"active":"1","last_modified":"2026-09-02 12:00:00"}]`, alias, destination)
			}
			_, _ = fmt.Fprintf(response, `{"status":200,"tm":1,"data":{"domain":"example.invalid","mailmaps":%s}}`, items)
		case request.Method == http.MethodPut && request.URL.Path == "/mail/maps/example.invalid":
			writes["create"]++
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode create body: %v", err)
			}
			if body["alias"] != "old" || body["host"] != "@" || body["destination"] != "a@example.net, z@example.net" || body["active"] != float64(1) {
				t.Errorf("unexpected create body: %#v", body)
			}
			created = true
			response.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprint(response, `{"status":201,"msg":"OK","data":{"domain":"example.invalid"}}`)
		case request.Method == http.MethodPost && request.URL.Path == "/mail/maps/example.invalid/old@example.invalid":
			writes["update"]++
			updated = true
			response.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprint(response, `{"status":201,"msg":"OK","data":{"domain":"example.invalid"}}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/mail/maps/example.invalid/12":
			writes["delete"]++
			deleted = true
			_, _ = fmt.Fprint(response, `{"status":200,"msg":"OK","data":{"domain":"example.invalid","mailmap_id":12}}`)
		default:
			http.Error(response, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	createdMailmap, err := client.CreateMailmap(context.Background(), MailmapRequest{
		Domain: "Example.Invalid.", Alias: "old", Host: "@", Destinations: []string{"z@example.net", "a@example.net"}, Active: true,
	})
	if err != nil {
		t.Fatalf("create mailmap: %v", err)
	}
	if createdMailmap.ID != "12" || createdMailmap.Email() != "old@example.invalid" || !reflect.DeepEqual(createdMailmap.Destinations, []string{"a@example.net", "z@example.net"}) {
		t.Fatalf("created mailmap=%+v", createdMailmap)
	}

	updatedMailmap, err := client.UpdateMailmap(context.Background(), "0012", createdMailmap.Email(), MailmapRequest{
		Domain: "example.invalid", Alias: "new", Host: "@", Destinations: []string{"new@example.net"}, Active: true,
	})
	if err != nil {
		t.Fatalf("update mailmap: %v", err)
	}
	if updatedMailmap.ID != "12" || updatedMailmap.Email() != "new@example.invalid" {
		t.Fatalf("updated mailmap=%+v", updatedMailmap)
	}
	if err := client.DeleteMailmap(context.Background(), "example.invalid", "12"); err != nil {
		t.Fatalf("delete mailmap: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(writes, map[string]int{"create": 1, "update": 1, "delete": 1}) {
		t.Fatalf("write counts=%v", writes)
	}
}

func TestCreateMailmapRejectsMultipleNewMatches(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPut {
			requests++
			response.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprint(response, `{"status":201,"msg":"OK","data":{"domain":"example.invalid"}}`)
			return
		}
		requests++
		items := "[]"
		if requests > 1 {
			items = `[{"mailmap_id":9,"alias":"test@example.invalid","host":"@","destination":"one@example.net","active":1},{"mailmap_id":10,"alias":"test@example.invalid","host":"@","destination":"one@example.net","active":1}]`
		}
		_, _ = fmt.Fprintf(response, `{"status":200,"data":{"domain":"example.invalid","mailmaps":%s}}`, items)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	_, err := client.CreateMailmap(context.Background(), MailmapRequest{Domain: "example.invalid", Alias: "test", Host: "@", Destinations: []string{"one@example.net"}, Active: true})
	var duplicate *DuplicateMailmapCandidatesError
	if !errors.As(err, &duplicate) || !strings.Contains(err.Error(), "9, 10") {
		t.Fatalf("error=%T %v", err, err)
	}
}

func TestMailmapValidationRejectsInvalidValues(t *testing.T) {
	t.Parallel()
	for _, request := range []MailmapRequest{
		{Domain: "example.invalid", Alias: "bad@example.invalid", Host: "@", Destinations: []string{"ok@example.net"}, Active: true},
		{Domain: "example.invalid", Alias: "ok", Host: "@", Destinations: nil, Active: true},
		{Domain: "example.invalid", Alias: "ok", Host: "@", Destinations: []string{"Display <ok@example.net>"}, Active: true},
	} {
		if _, _, err := request.normalize(); err == nil {
			t.Errorf("invalid request accepted: %+v", request)
		}
	}
}

func TestCreateMailmapReconcilesAmbiguousWriteWithoutReplay(t *testing.T) {
	t.Parallel()
	var writes atomic.Int64
	var created atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPut {
			writes.Add(1)
			created.Store(true)
			response.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprint(response, `{"status":503,"msg":"uncertain"}`)
			return
		}
		items := "[]"
		if created.Load() {
			items = `[{"mailmap_id":12,"alias":"test@example.invalid","host":"@","destination":"one@example.net","active":1}]`
		}
		_, _ = fmt.Fprintf(response, `{"status":200,"data":{"domain":"example.invalid","mailmaps":%s}}`, items)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, func(config *Config) {
		config.RetryPolicy.MaxAttempts = 4
	})

	mailmap, err := client.CreateMailmap(context.Background(), MailmapRequest{Domain: "example.invalid", Alias: "test", Host: "@", Destinations: []string{"one@example.net"}, Active: true})
	if err != nil || mailmap.ID != "12" || writes.Load() != 1 {
		t.Fatalf("mailmap=%+v writes=%d error=%v", mailmap, writes.Load(), err)
	}
}
