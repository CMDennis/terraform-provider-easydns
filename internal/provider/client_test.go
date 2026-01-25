package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestToInt64(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   interface{}
		want int64
	}{
		{name: "nil", in: nil, want: 0},
		{name: "float64", in: float64(12), want: 12},
		{name: "int64", in: int64(34), want: 34},
		{name: "int", in: int(56), want: 56},
		{name: "string", in: "78", want: 78},
		{name: "json.Number", in: json.Number("90"), want: 90},
		{name: "bad string", in: "abc", want: 0},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := toInt64(tc.in)
			if got != tc.want {
				t.Fatalf("toInt64(%T=%v)=%d, want %d", tc.in, tc.in, got, tc.want)
			}
		})
	}
}

func TestClientDoRequest_SetsHeadersAndAuth(t *testing.T) {
	t.Parallel()

	var gotAuthUser, gotAuthPass string
	var gotContentType, gotAccept string
	var gotBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/test" {
			t.Fatalf("path=%s, want /test", r.URL.Path)
		}

		gotAuthUser, gotAuthPass, _ = r.BasicAuth()
		gotContentType = r.Header.Get("Content-Type")
		gotAccept = r.Header.Get("Accept")

		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		gotBody = string(b)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "key", false)
	c.HTTPClient = srv.Client()

	_, err := c.doRequest(http.MethodPost, "/test", map[string]interface{}{"a": "b"})
	if err != nil {
		t.Fatalf("doRequest returned error: %v", err)
	}

	if gotAuthUser != "tok" || gotAuthPass != "key" {
		t.Fatalf("basic auth=(%q,%q), want (%q,%q)", gotAuthUser, gotAuthPass, "tok", "key")
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type=%q, want application/json", gotContentType)
	}
	if gotAccept != "application/json" {
		t.Fatalf("Accept=%q, want application/json", gotAccept)
	}
	if !strings.Contains(gotBody, `"a":"b"`) {
		t.Fatalf("body=%q, want to contain %q", gotBody, `"a":"b"`)
	}
}

func TestClientDoRequest_HTTPStatusError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("nope"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "key", false)
	c.HTTPClient = srv.Client()

	_, err := c.doRequest(http.MethodGet, "/anything", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("error=%q, want to contain status 500", err.Error())
	}
}

func TestClientDoRequest_BodyErrorEvenOn200(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":{"code":123,"message":"bad"}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "key", false)
	c.HTTPClient = srv.Client()

	_, err := c.doRequest(http.MethodGet, "/anything", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "code 123") || !strings.Contains(err.Error(), "bad") {
		t.Fatalf("error=%q, want to contain code and message", err.Error())
	}
}

func TestClientCreateRecord_PathPayloadAndResponseParsing(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method=%s, want %s", r.Method, http.MethodPut)
		}
		if r.URL.Path != "/zones/records/add/example.com/A" {
			t.Fatalf("path=%s, want %s", r.URL.Path, "/zones/records/add/example.com/A")
		}

		var body map[string]interface{}
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if err := json.Unmarshal(b, &body); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}

		if body["host"] != "www" {
			t.Fatalf("host=%v, want www", body["host"])
		}
		if body["rdata"] != "1.2.3.4" {
			t.Fatalf("rdata=%v, want 1.2.3.4", body["rdata"])
		}
		if body["ttl"] != float64(300) {
			t.Fatalf("ttl=%v, want 300", body["ttl"])
		}
		if _, ok := body["prio"]; ok {
			t.Fatalf("prio should be omitted when 0")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tm":0,"data":{"id":"1","domain":"example.com","host":"www","type":"A","rdata":"1.2.3.4","ttl":300,"prio":null,"geozone_id":"2","last_mod":"x"},"status":200}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "key", false)
	c.HTTPClient = srv.Client()

	rec, err := c.CreateRecord(CreateRecordRequest{
		Domain: "example.com",
		Host:   "www",
		Type:   "A",
		Rdata:  "1.2.3.4",
		TTL:    300,
		Prio:   0,
	})
	if err != nil {
		t.Fatalf("CreateRecord error: %v", err)
	}

	if rec.ID != "1" || rec.Domain != "example.com" || rec.Host != "www" || rec.Type != "A" || rec.Rdata != "1.2.3.4" {
		t.Fatalf("unexpected record: %+v", rec)
	}
	if rec.TTL != 300 {
		t.Fatalf("ttl=%d, want 300", rec.TTL)
	}
	if rec.Prio != 0 {
		t.Fatalf("prio=%d, want 0", rec.Prio)
	}
	if rec.GeozoneID != 2 {
		t.Fatalf("geozone_id=%d, want 2", rec.GeozoneID)
	}
}

func TestClientUpdateRecord_IncludesPrioWhenZero(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/zones/records/123" {
			t.Fatalf("path=%s, want %s", r.URL.Path, "/zones/records/123")
		}

		var body map[string]interface{}
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if err := json.Unmarshal(b, &body); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}

		if body["prio"] != float64(0) {
			t.Fatalf("prio=%v, want 0", body["prio"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tm":0,"data":{"id":"123","domain":"example.com","host":"@","type":"MX","rdata":"mail.example.com.","ttl":"600","prio":"0","geozone_id":null,"last_mod":"x"},"status":200}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "key", false)
	c.HTTPClient = srv.Client()

	rec, err := c.UpdateRecord("123", CreateRecordRequest{
		Host: "@",
		Type: "MX",
		Rdata: "mail.example.com.",
		TTL:  0,
		Prio: 0,
	})
	if err != nil {
		t.Fatalf("UpdateRecord error: %v", err)
	}
	if rec.Prio != 0 {
		t.Fatalf("prio=%d, want 0", rec.Prio)
	}
	if rec.TTL != 600 {
		t.Fatalf("ttl=%d, want 600", rec.TTL)
	}
}

func TestClientCreateRecord_AsyncAPI(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method=%s, want %s", r.Method, http.MethodPut)
		}
		// Verify async endpoint path
		if r.URL.Path != "/zones/async/ux/records/add/example.com/A" {
			t.Fatalf("path=%s, want %s", r.URL.Path, "/zones/async/ux/records/add/example.com/A")
		}

		var body map[string]interface{}
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if err := json.Unmarshal(b, &body); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}

		// Async API requires type in body
		if body["type"] != "A" {
			t.Fatalf("type=%v, want A (async API requires type in body)", body["type"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tm":0,"data":{"id":"1","domain":"example.com","host":"www","type":"A","rdata":"1.2.3.4","ttl":300,"prio":null,"geozone_id":"0","last_mod":"x"},"status":201}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "key", true) // async enabled
	c.HTTPClient = srv.Client()

	rec, err := c.CreateRecord(CreateRecordRequest{
		Domain: "example.com",
		Host:   "www",
		Type:   "A",
		Rdata:  "1.2.3.4",
		TTL:    300,
	})
	if err != nil {
		t.Fatalf("CreateRecord error: %v", err)
	}
	if rec.ID != "1" {
		t.Fatalf("id=%s, want 1", rec.ID)
	}
}

func TestClientUpdateRecord_AsyncAPI(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s, want %s", r.Method, http.MethodPost)
		}
		// Verify async endpoint path
		if r.URL.Path != "/zones/async/ux/records/123" {
			t.Fatalf("path=%s, want %s", r.URL.Path, "/zones/async/ux/records/123")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tm":0,"data":{"id":"123","domain":"example.com","host":"www","type":"A","rdata":"1.2.3.5","ttl":"600","prio":"0","geozone_id":null,"last_mod":"x"},"status":200}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "key", true) // async enabled
	c.HTTPClient = srv.Client()

	rec, err := c.UpdateRecord("123", CreateRecordRequest{
		Host:  "www",
		Type:  "A",
		Rdata: "1.2.3.5",
		TTL:   600,
	})
	if err != nil {
		t.Fatalf("UpdateRecord error: %v", err)
	}
	if rec.ID != "123" {
		t.Fatalf("id=%s, want 123", rec.ID)
	}
}

func TestClientDeleteRecord_AsyncAPI(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method=%s, want %s", r.Method, http.MethodDelete)
		}
		// Verify async endpoint path
		if r.URL.Path != "/zones/async/ux/records/example.com/123" {
			t.Fatalf("path=%s, want %s", r.URL.Path, "/zones/async/ux/records/example.com/123")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tm":0,"status":200}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "key", true) // async enabled
	c.HTTPClient = srv.Client()

	err := c.DeleteRecord("example.com", "123")
	if err != nil {
		t.Fatalf("DeleteRecord error: %v", err)
	}
}

func TestClientGetZone_ExpiryConversion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		expiryJSON string
		wantExpiry string
	}{
		{name: "expiry false", expiryJSON: "false", wantExpiry: ""},
		{name: "expiry string", expiryJSON: `"2026-01-01"`, wantExpiry: "2026-01-01"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Fatalf("method=%s, want %s", r.Method, http.MethodGet)
				}
				if r.URL.Path != "/domain/example.com" {
					t.Fatalf("path=%s, want %s", r.URL.Path, "/domain/example.com")
				}

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"msg":"ok","tm":0,"data":{"id":"z1","domain":"example.com","exists":"Y","onsystem":"N","expiry":` + tc.expiryJSON + `,"next_due":"x","service":"y"},"status":200}`))
			}))
			defer srv.Close()

			c := NewClient(srv.URL, "tok", "key", false)
			c.HTTPClient = srv.Client()

			zone, err := c.GetZone("example.com")
			if err != nil {
				t.Fatalf("GetZone error: %v", err)
			}
			if zone.Expiry != tc.wantExpiry {
				t.Fatalf("expiry=%q, want %q", zone.Expiry, tc.wantExpiry)
			}
			if !zone.Exists {
				t.Fatalf("Exists=false, want true")
			}
			if zone.OnSystem {
				t.Fatalf("OnSystem=true, want false")
			}
		})
	}
}
