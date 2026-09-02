package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateRecordSynchronousRequestAndFlexibleResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut || request.URL.Path != "/zones/records/add/example.invalid/A" {
			t.Errorf("request=%s %s", request.Method, request.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, exists := body["type"]; exists {
			t.Errorf("synchronous create unexpectedly sent type: %v", body)
		}
		if body["host"] != "www" || body["rdata"] != "192.0.2.10" || body["ttl"] != float64(300) || body["geozone_id"] != float64(2) {
			t.Errorf("request body=%v", body)
		}
		_, _ = response.Write([]byte(`{"data":{"id":1001,"domain":"example.invalid","host":"www","type":"A","rdata":"192.0.2.10","ttl":"300","prio":null,"geozone_id":"2","last_mod":"now"},"status":201}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	record, err := client.createRecordOnce(context.Background(), CreateRecordRequest{
		Domain: "example.invalid", Host: "www", Type: "A", Rdata: "192.0.2.10", TTL: 300, GeozoneID: 2,
	}, RecordWriteModeSynchronous)
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if record.ID != "1001" || record.TTL != 300 || record.Prio != 0 || record.GeozoneID != 2 {
		t.Fatalf("record=%+v", record)
	}
}

func TestCreateRecordAsynchronousIncludesType(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/zones/async/ux/records/add/example.invalid/TXT" {
			t.Errorf("path=%s", request.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(request.Body).Decode(&body)
		if body["type"] != "TXT" {
			t.Errorf("body=%v", body)
		}
		_, _ = response.Write([]byte(`{"data":{"id":"1002","domain":"example.invalid","host":"_test","type":"TXT","rdata":"value","ttl":600},"status":201}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeAsynchronous, nil)
	record, err := client.createRecordOnce(context.Background(), CreateRecordRequest{
		Domain: "example.invalid", Host: "_test", Type: "TXT", Rdata: "value", TTL: 600,
	}, RecordWriteModeAsynchronous)
	if err != nil || record.ID != "1002" {
		t.Fatalf("record=%+v error=%v", record, err)
	}
}

func TestCreateRecordEmptySuccessResponseIsExplicit(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	_, err := client.createRecordOnce(context.Background(), CreateRecordRequest{Domain: "example.invalid", Host: "www", Type: "A", Rdata: "192.0.2.10"}, RecordWriteModeSynchronous)
	if !errors.Is(err, ErrEmptyResponse) {
		t.Fatalf("error=%v, want ErrEmptyResponse", err)
	}
	if !IsAmbiguousWrite(err) {
		t.Fatalf("error=%T %v, want ambiguous write", err, err)
	}
}

func TestGetRecordsPaginatesAndNormalizesScalarTypes(t *testing.T) {
	t.Parallel()

	requestNumber := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestNumber++
		if request.URL.Path != "/zones/records/all/example.invalid" {
			t.Errorf("path=%s", request.URL.Path)
		}
		switch requestNumber {
		case 1:
			if request.URL.RawQuery != "" {
				t.Errorf("first query=%q", request.URL.RawQuery)
			}
			_, _ = response.Write([]byte(`{"data":[{"id":"1001","domain":"example.invalid","host":"www","type":"A","rdata":"192.0.2.10","ttl":"300","prio":null,"geozone_id":"0"}],"count":"1","total":"2","start":"0","max":"1","status":200}`))
		case 2:
			if request.URL.Query().Get("start") != "1" || request.URL.Query().Get("max") != "1" {
				t.Errorf("second query=%q", request.URL.RawQuery)
			}
			_, _ = response.Write([]byte(`{"data":[{"id":1002,"domain":"example.invalid","host":"@","type":"MX","rdata":"mail.example.invalid.","ttl":600,"prio":10,"geozone_id":0}],"count":1,"total":2,"start":1,"max":1,"status":200}`))
		default:
			t.Errorf("unexpected request %d", requestNumber)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	records, err := client.GetRecords(context.Background(), "example.invalid")
	if err != nil {
		t.Fatalf("GetRecords: %v", err)
	}
	if len(records) != 2 || records[0].ID != "1001" || records[0].TTL != 300 || records[1].ID != "1002" || records[1].Prio != 10 {
		t.Fatalf("records=%+v", records)
	}
}

func TestGetRecordsDetectsPaginationWithoutProgress(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"data":[],"count":0,"total":2,"start":0,"max":1,"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	_, err := client.GetRecords(context.Background(), "example.invalid")
	if err == nil {
		t.Fatal("expected pagination error")
	}
}

func TestGetRecordReturnsTypedNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"data":[],"count":0,"total":0,"start":0,"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	_, err := client.GetRecord(context.Background(), "example.invalid", "missing")
	if !IsNotFound(err) {
		t.Fatalf("error=%T %v", err, err)
	}
}

func TestGetRecordReturnsMatchingRecord(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"data":[{"id":"1001","domain":"example.invalid","host":"www","type":"A","rdata":"192.0.2.10","ttl":300}],"count":1,"total":1,"start":0,"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	record, err := client.GetRecord(context.Background(), "example.invalid", "1001")
	if err != nil || record.ID != "1001" {
		t.Fatalf("record=%+v error=%v", record, err)
	}
}

func TestUpdateRecordIncludesDomainAndZeroPriority(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/zones/records/1001" {
			t.Errorf("request=%s %s", request.Method, request.URL.Path)
		}
		body, _ := io.ReadAll(request.Body)
		var values map[string]any
		_ = json.Unmarshal(body, &values)
		if values["domain"] != "example.invalid" || values["prio"] != float64(0) {
			t.Errorf("body=%v", values)
		}
		_, _ = response.Write([]byte(`{"data":{"id":"1001","domain":"example.invalid","host":"@","type":"MX","rdata":"mail.example.invalid.","ttl":600,"prio":"0"},"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	record, err := client.updateRecordOnce(context.Background(), "1001", CreateRecordRequest{
		Domain: "example.invalid", Host: "@", Type: "MX", Rdata: "mail.example.invalid.", TTL: 600, Prio: 0,
	}, RecordWriteModeSynchronous)
	if err != nil || record.ID != "1001" || record.Prio != 0 {
		t.Fatalf("record=%+v error=%v", record, err)
	}
}

func TestUpdateRecordAsynchronousRouteAndEmptyData(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/zones/async/ux/records/1001" {
			t.Errorf("path=%s", request.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(request.Body).Decode(&body)
		if body["geozone_id"] != float64(4) {
			t.Errorf("body=%v", body)
		}
		_, _ = response.Write([]byte(`{"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeAsynchronous, nil)
	_, err := client.updateRecordOnce(context.Background(), "1001", CreateRecordRequest{
		Domain: "example.invalid", Host: "www", Type: "A", Rdata: "192.0.2.10", TTL: 300, GeozoneID: 4,
	}, RecordWriteModeAsynchronous)
	if !errors.Is(err, ErrEmptyResponse) {
		t.Fatalf("error=%v, want ErrEmptyResponse", err)
	}
	if !IsAmbiguousWrite(err) {
		t.Fatalf("error=%T %v, want ambiguous write", err, err)
	}
}

func TestDeleteRecordRoutesByWriteMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode RecordWriteMode
		path string
	}{
		{name: "synchronous", mode: RecordWriteModeSynchronous, path: "/zones/records/example.invalid/1001"},
		{name: "asynchronous", mode: RecordWriteModeAsynchronous, path: "/zones/async/ux/records/example.invalid/1001"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodDelete || request.URL.Path != test.path {
					t.Errorf("request=%s %s", request.Method, request.URL.Path)
				}
				response.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			client := newTestClient(t, server.URL, server.Client(), test.mode, nil)
			if err := client.deleteRecordOnce(context.Background(), "example.invalid", "1001", test.mode); err != nil {
				t.Fatalf("DeleteRecord: %v", err)
			}
		})
	}
}
