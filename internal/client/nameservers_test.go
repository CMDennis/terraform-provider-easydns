package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGetDomainNameserversNormalizesToASortedSet(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/domains/ns/example.invalid" {
			t.Errorf("path=%s", request.URL.Path)
		}
		_, _ = response.Write([]byte(`{"status":200,"tm":1,"msg":"OK","data":{"domain":"example.invalid","nameservers":["NS2.Example.Invalid.","ns1.example.invalid"]}}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	nameservers, err := client.GetDomainNameservers(context.Background(), "Example.Invalid")
	if err != nil {
		t.Fatalf("GetDomainNameservers: %v", err)
	}
	if len(nameservers) != 2 || nameservers[0] != "ns1.example.invalid" || nameservers[1] != "ns2.example.invalid" {
		t.Fatalf("nameservers=%+v", nameservers)
	}
}

func TestSetDomainNameserversEnforcesDocumentedBounds(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, "https://example.invalid", nil, RecordWriteModeSynchronous, nil)

	_, err := client.SetDomainNameservers(context.Background(), "example.invalid", []string{"ns1.example.invalid"})
	if err == nil || !strings.Contains(err.Error(), "between 2 and 10") {
		t.Fatalf("error=%v", err)
	}

	tooMany := make([]string, 0, 11)
	for index := 0; index < 11; index++ {
		tooMany = append(tooMany, string(rune('a'+index))+"ns.example.invalid")
	}
	_, err = client.SetDomainNameservers(context.Background(), "example.invalid", tooMany)
	if err == nil || !strings.Contains(err.Error(), "between 2 and 10") {
		t.Fatalf("error=%v", err)
	}

	_, err = client.SetDomainNameservers(context.Background(), "example.invalid", []string{"ns1.example.invalid", "NS1.Example.Invalid."})
	if err == nil || !strings.Contains(err.Error(), "listed more than once") {
		t.Fatalf("duplicate nameservers error=%v", err)
	}
}

func TestSetDomainNameserversPollsUntilDelegationMatches(t *testing.T) {
	t.Parallel()

	var writes, reads int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			atomic.AddInt32(&writes, 1)
			_, _ = response.Write([]byte(`{"status":200,"msg":"OK","data":{"domain":"example.invalid","nameservers":[]}}`))
			return
		}
		if atomic.AddInt32(&reads, 1) == 1 {
			_, _ = response.Write([]byte(`{"status":200,"msg":"OK","data":{"domain":"example.invalid","nameservers":["old1.example.invalid","old2.example.invalid"]}}`))
			return
		}
		_, _ = response.Write([]byte(`{"status":200,"msg":"OK","data":{"domain":"example.invalid","nameservers":["ns1.example.invalid","ns2.example.invalid"]}}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	nameservers, err := client.SetDomainNameservers(context.Background(), "example.invalid", []string{"ns2.example.invalid", "ns1.example.invalid"})
	if err != nil {
		t.Fatalf("SetDomainNameservers: %v", err)
	}
	if len(nameservers) != 2 || nameservers[0] != "ns1.example.invalid" {
		t.Fatalf("nameservers=%+v", nameservers)
	}
	if got := atomic.LoadInt32(&writes); got != 1 {
		t.Fatalf("nameserver update issued %d writes, want exactly 1", got)
	}
	if atomic.LoadInt32(&reads) < 2 {
		t.Fatal("the update did not poll until the delegation matched")
	}
}
