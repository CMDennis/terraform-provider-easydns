package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestListGlueRecordsNormalizesAndSorts(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/domains/glue/example.invalid" {
			t.Errorf("path=%s", request.URL.Path)
		}
		_, _ = response.Write([]byte(`{"status":200,"tm":1,"msg":"OK","data":{"domain":"example.invalid","total":2,"glue_records":[
			{"name":"NS2.Example.Invalid.","domain":"example.invalid","ipaddress":"192.0.2.54","ipv6":"2001:0DB8:0000:0000:0000:0000:0000:0054"},
			{"name":"ns1.example.invalid","domain":"example.invalid","ipaddress":"192.0.2.53","ipv6":""}]}}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	records, err := client.ListGlueRecords(context.Background(), "example.invalid")
	if err != nil {
		t.Fatalf("ListGlueRecords: %v", err)
	}
	if len(records) != 2 || records[0].Host != "ns1.example.invalid" || records[1].Host != "ns2.example.invalid" {
		t.Fatalf("records were not normalized and sorted: %+v", records)
	}
	if records[1].IPv6 != "2001:db8::54" {
		t.Fatalf("IPv6 was not canonicalized: %q", records[1].IPv6)
	}
	if records[0].IPv6 != "" {
		t.Fatalf("absent IPv6 should stay empty, got %q", records[0].IPv6)
	}
}

func TestGetGlueRecordReportsMissingHostAsNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"status":200,"msg":"OK","data":{"domain":"example.invalid","total":0,"glue_records":[]}}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	_, err := client.GetGlueRecord(context.Background(), "example.invalid", "ns1.example.invalid")
	if !IsNotFound(err) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestGlueMutationResponseAcceptsNameserverNameKey(t *testing.T) {
	t.Parallel()

	var writes int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPut {
			atomic.AddInt32(&writes, 1)
			_, _ = response.Write([]byte(`{"status":201,"msg":"OK","data":{"domain":"example.invalid","nameserver_name":"ns1.example.invalid","ipaddress":"192.0.2.53"}}`))
			return
		}
		_, _ = response.Write([]byte(`{"status":200,"msg":"OK","data":{"domain":"example.invalid","total":1,"glue_records":[{"name":"ns1.example.invalid","domain":"example.invalid","ipaddress":"192.0.2.53"}]}}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	record, err := client.CreateGlueRecord(context.Background(), GlueRecordRequest{
		Domain: "example.invalid", Host: "NS1.Example.Invalid.", IPv4: "192.0.2.53",
	})
	if err != nil {
		t.Fatalf("CreateGlueRecord: %v", err)
	}
	if record.Host != "ns1.example.invalid" || record.IPv4 != "192.0.2.53" {
		t.Fatalf("record=%+v", record)
	}
	if got := atomic.LoadInt32(&writes); got != 1 {
		t.Fatalf("glue creation issued %d writes, want exactly 1", got)
	}
}

func TestGlueRequestRequiresAnAddress(t *testing.T) {
	t.Parallel()

	_, _, err := GlueRecordRequest{Domain: "example.invalid", Host: "ns1.example.invalid"}.normalize()
	if err == nil || !strings.Contains(err.Error(), "requires an IPv4 address") {
		t.Fatalf("error=%v", err)
	}

	_, _, err = GlueRecordRequest{Domain: "example.invalid", Host: "ns1.example.invalid", IPv4: "2001:db8::1"}.normalize()
	if err == nil || !strings.Contains(err.Error(), "invalid glue IPv4") {
		t.Fatalf("error=%v", err)
	}

	_, _, err = GlueRecordRequest{Domain: "example.invalid", Host: "ns1.example.invalid", IPv6: "192.0.2.1"}.normalize()
	if err == nil || !strings.Contains(err.Error(), "invalid glue IPv6") {
		t.Fatalf("error=%v", err)
	}
}

func TestDeleteGlueRecordSurfacesRegistryRefusal(t *testing.T) {
	t.Parallel()

	var deletes int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodDelete {
			atomic.AddInt32(&deletes, 1)
			response.WriteHeader(http.StatusConflict)
			_, _ = response.Write([]byte(`{"error":{"code":409,"message":"glue still in use"}}`))
			return
		}
		t.Errorf("unexpected %s request", request.Method)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	err := client.DeleteGlueRecord(context.Background(), "example.invalid", "ns1.example.invalid")
	if err == nil || !strings.Contains(err.Error(), "glue still in use") {
		t.Fatalf("error=%v", err)
	}
	if got := atomic.LoadInt32(&deletes); got != 1 {
		t.Fatalf("a refused deletion was retried %d times", got)
	}
}

func TestCheckRegistryGlueReadsExistsFlag(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/domains/glue/example.invalid/ns1.example.invalid/status" {
			t.Errorf("path=%s", request.URL.Path)
		}
		_, _ = response.Write([]byte(`{"status":200,"msg":"OK","data":{"domain":"example.invalid","fqdn":"ns1.example.invalid","exists":"1"}}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	exists, err := client.CheckRegistryGlue(context.Background(), "example.invalid", "ns1.example.invalid")
	if err != nil || !exists {
		t.Fatalf("exists=%v error=%v", exists, err)
	}
}
