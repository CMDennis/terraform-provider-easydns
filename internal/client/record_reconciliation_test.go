package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const matchingARecord = `{"id":"20","domain":"example.invalid","host":"www","type":"A","rdata":"192.0.2.10","ttl":300,"prio":0,"geozone_id":0}`

func TestCreateRecordReconcilesNewIDInBothWriteModes(t *testing.T) {
	for _, test := range []struct {
		name      string
		mode      RecordWriteMode
		writePath string
	}{
		{name: "synchronous", mode: RecordWriteModeSynchronous, writePath: "/zones/records/add/example.invalid/A"},
		{name: "asynchronous", mode: RecordWriteModeAsynchronous, writePath: "/zones/async/ux/records/add/example.invalid/A"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var reads atomic.Int32
			var writes atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				switch request.Method {
				case http.MethodGet:
					read := reads.Add(1)
					data := `[{"id":"10","domain":"example.invalid","host":"www","type":"A","rdata":"192.0.2.10","ttl":300,"prio":0,"geozone_id":0}]`
					if read >= 3 {
						data = `[` + data[1:len(data)-1] + `,` + matchingARecord + `]`
					}
					_, _ = fmt.Fprintf(response, `{"data":%s,"count":%d,"total":%d,"status":200}`, data, 1, 1)
				case http.MethodPut:
					writes.Add(1)
					if request.URL.Path != test.writePath {
						t.Errorf("write path=%s", request.URL.Path)
					}
					response.WriteHeader(http.StatusCreated)
				default:
					t.Errorf("method=%s", request.Method)
				}
			}))
			defer server.Close()

			fake := newFakeTime()
			client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, func(config *Config) {
				config.Clock = fake
				config.Waiter = fake
				config.RecordPollInterval = time.Second
				config.RecordReconcileTimeout = 5 * time.Second
			})
			record, err := client.CreateRecordWithMode(context.Background(), CreateRecordRequest{
				Domain: "EXAMPLE.INVALID.", Host: "WWW", Type: "a", Rdata: "192.0.2.10", TTL: 300,
			}, test.mode)
			if err != nil || record.ID != "20" {
				t.Fatalf("record=%+v error=%v", record, err)
			}
			if writes.Load() != 1 {
				t.Fatalf("writes=%d, want exactly one", writes.Load())
			}
		})
	}
}

func TestCreateRecordRejectsMultipleNewMatchingIDs(t *testing.T) {
	var wrote atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPut {
			wrote.Store(true)
			response.WriteHeader(http.StatusCreated)
			return
		}
		data := `[]`
		count := 0
		if wrote.Load() {
			data = `[` + matchingARecord + `,{"id":"19","domain":"example.invalid","host":"www","type":"A","rdata":"192.0.2.10","ttl":300,"prio":0,"geozone_id":0}]`
			count = 2
		}
		_, _ = fmt.Fprintf(response, `{"data":%s,"count":%d,"total":%d,"status":200}`, data, count, count)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	_, err := client.CreateRecord(context.Background(), CreateRecordRequest{
		Domain: "example.invalid", Host: "www", Type: "A", Rdata: "192.0.2.10", TTL: 300,
	})
	var duplicate *DuplicateRecordCandidatesError
	if !errors.As(err, &duplicate) || len(duplicate.IDs) != 2 || duplicate.IDs[0] != "19" {
		t.Fatalf("error=%T %v", err, err)
	}
}

func TestUpdateRecordReconcilesAmbiguousWrite(t *testing.T) {
	var writes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			writes.Add(1)
			response.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = response.Write([]byte(`{"data":[` + matchingARecord + `],"count":1,"total":1,"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	record, err := client.UpdateRecord(context.Background(), "20", CreateRecordRequest{
		Domain: "example.invalid", Host: "www", Type: "A", Rdata: "192.0.2.10", TTL: 300,
	})
	if err != nil || record.ID != "20" || writes.Load() != 1 {
		t.Fatalf("record=%+v error=%v writes=%d", record, err, writes.Load())
	}
}

func TestDeleteRecordWaitsUntilAbsent(t *testing.T) {
	var reads atomic.Int32
	var writes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodDelete {
			writes.Add(1)
			response.WriteHeader(http.StatusNoContent)
			return
		}
		if reads.Add(1) == 1 {
			_, _ = response.Write([]byte(`{"data":[` + matchingARecord + `],"count":1,"total":1,"status":200}`))
			return
		}
		_, _ = response.Write([]byte(`{"data":[],"count":0,"total":0,"status":200}`))
	}))
	defer server.Close()

	fake := newFakeTime()
	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeAsynchronous, func(config *Config) {
		config.Clock = fake
		config.Waiter = fake
	})
	if err := client.DeleteRecord(context.Background(), "example.invalid", "20"); err != nil {
		t.Fatalf("DeleteRecord: %v", err)
	}
	if writes.Load() != 1 || reads.Load() != 2 {
		t.Fatalf("writes=%d reads=%d", writes.Load(), reads.Load())
	}
}

func TestCreateRecordReconciliationTimeoutPreservesAmbiguousCause(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPut {
			response.WriteHeader(http.StatusCreated)
			return
		}
		_, _ = response.Write([]byte(`{"data":[],"count":0,"total":0,"status":200}`))
	}))
	defer server.Close()

	fake := newFakeTime()
	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, func(config *Config) {
		config.Clock = fake
		config.Waiter = fake
		config.RecordPollInterval = time.Second
		config.RecordReconcileTimeout = 2 * time.Second
	})
	_, err := client.CreateRecord(context.Background(), CreateRecordRequest{
		Domain: "example.invalid", Host: "www", Type: "A", Rdata: "192.0.2.10", TTL: 300,
	})
	var timeout *ReconciliationTimeoutError
	if !errors.As(err, &timeout) || !errors.Is(err, ErrEmptyResponse) || !IsAmbiguousWrite(err) {
		t.Fatalf("error=%T %v", err, err)
	}
}
