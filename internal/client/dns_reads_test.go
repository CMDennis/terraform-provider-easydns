package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchRecordsEscapesKeywordAndSortsIDs(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.EscapedPath() != "/zones/records/all/example.invalid/search/www%2Fprod" {
			t.Errorf("escaped path=%s", request.URL.EscapedPath())
		}
		_, _ = response.Write([]byte(`{"data":[{"id":"10","domain":"example.invalid","host":"b","type":"TXT","rdata":"b"},{"id":"2","domain":"example.invalid","host":"a","type":"TXT","rdata":"a"}],"count":2,"total":2,"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	records, err := client.SearchRecords(context.Background(), "example.invalid", "www/prod")
	if err != nil || len(records) != 2 || records[0].ID != "2" {
		t.Fatalf("records=%+v error=%v", records, err)
	}
}

func TestGetParsedRecordsSupportsArrayAndParsedFields(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"data":[{"id":7,"domain":"example.invalid","host":"go","type":"URL","rdata":"192.0.2.1","ttl":"600","url":"https://example.invalid/","orig_rdata":"LOCAL."}],"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	records, err := client.GetParsedRecords(context.Background(), "example.invalid")
	if err != nil || len(records) != 1 || records[0].ID != "7" || records[0].URL == "" || records[0].OrigRdata != "LOCAL." {
		t.Fatalf("records=%+v error=%v", records, err)
	}
}

func TestGetZoneSOASupportsRootAndEnvelopeShapes(t *testing.T) {
	for _, body := range []string{
		`{"domain":"example.invalid","soa":2026090101}`,
		`{"data":{"domain":"example.invalid","soa":"2026090102"},"status":200}`,
	} {
		body := body
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				_, _ = response.Write([]byte(body))
			}))
			defer server.Close()
			client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
			soa, err := client.GetZoneSOA(context.Background(), "example.invalid")
			if err != nil || soa.Serial == 0 || soa.Domain != "example.invalid" {
				t.Fatalf("soa=%+v error=%v", soa, err)
			}
		})
	}
}

func TestGetGeoRegionsPaginatesAndSorts(t *testing.T) {
	t.Parallel()

	requestNumber := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestNumber++
		if requestNumber == 1 {
			_, _ = response.Write([]byte(`{"data":[{"id":"2","geo_code":"EU","location":"Europe"}],"count":"1","total":"2","start":"0","max":"1","status":200}`))
			return
		}
		if request.URL.Query().Get("start") != "1" || request.URL.Query().Get("max") != "1" {
			t.Errorf("query=%s", request.URL.RawQuery)
		}
		_, _ = response.Write([]byte(`{"data":{"id":1,"geo_code":"NA","location":"North America"},"count":1,"total":2,"start":1,"max":1,"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)
	result, err := client.GetGeoRegions(context.Background(), nil)
	if err != nil || len(result.Regions) != 2 || result.Regions[0].ID != 1 || result.Total != 2 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}
