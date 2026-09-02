package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGetDomainDecodesEnvelopeAndFlags(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/domain/example.invalid" {
			t.Errorf("path=%s", request.URL.Path)
		}
		_, _ = response.Write([]byte(`{"msg":"OK","tm":1788307200,"data":{"id":"example.invalid","domain":"example.invalid","exists":"Y","onsystem":"Y","expiry":"2030-01-02","next_due":"2029-12-01","cloned_to":false,"service":3857,"sub_block":"1911"},"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	domain, err := client.GetDomain(context.Background(), "Example.Invalid.")
	if err != nil {
		t.Fatalf("GetDomain: %v", err)
	}
	if domain.Domain != "example.invalid" || !domain.Exists || !domain.OnSystem {
		t.Fatalf("domain=%+v", domain)
	}
	if domain.Expiry != "2030-01-02" || domain.Service != 3857 || domain.SubscriptionID != 1911 {
		t.Fatalf("domain=%+v", domain)
	}
	if domain.ClonedTo != "" {
		t.Fatalf("cloned_to false should decode as empty, got %q", domain.ClonedTo)
	}
}

func TestGetDomainTreatsOffSystemAsNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"msg":"OK","data":{"domain":"example.invalid","exists":"N","onsystem":"N","expiry":false,"next_due":"","service":0},"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	_, err := client.GetDomain(context.Background(), "example.invalid")
	if !IsNotFound(err) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

// An empty user must be resolved through /user. The endpoint rejects a
// placeholder path segment: the sandbox answers /domains/list/self with
// HTTP 400 "Username provided does not match provided credentials".
func TestListUserDomainsSortsAndUsesAuthenticatedAccount(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/user":
			_, _ = response.Write([]byte(`{"msg":"OK","data":{"user":"tester"},"status":200}`))
		case "/domains/list/tester":
			_, _ = response.Write([]byte(`{"status":200,"tm":1,"msg":"OK","data":{"user":"tester","index":[{"name":"zeta.invalid","link":"z"},{"name":"alpha.invalid","link":"a"}]}}`))
		default:
			t.Errorf("unexpected path %s; a placeholder username is rejected by EasyDNS", request.URL.Path)
			response.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	user, domains, err := client.ListUserDomains(context.Background(), "")
	if err != nil {
		t.Fatalf("ListUserDomains: %v", err)
	}
	if user != "tester" || len(domains) != 2 {
		t.Fatalf("user=%q domains=%+v", user, domains)
	}
	if domains[0].Domain != "alpha.invalid" || domains[1].Domain != "zeta.invalid" {
		t.Fatalf("domains were not sorted: %+v", domains)
	}
}

func TestListUserDomainsAcceptsSingleObjectIndex(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"status":200,"msg":"OK","data":{"user":"tester","index":{"name":"only.invalid","link":"o"}}}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	_, domains, err := client.ListUserDomains(context.Background(), "tester")
	if err != nil || len(domains) != 1 || domains[0].Domain != "only.invalid" {
		t.Fatalf("domains=%+v error=%v", domains, err)
	}
}

func TestBuildCreateDomainBodyRejectsInconsistentRequests(t *testing.T) {
	t.Parallel()

	base := CreateDomainRequest{Domain: "example.invalid", Service: DomainServiceDNS, Term: 1, Currency: DomainCurrencyUSD}
	owner := &ContactSet{Owner: &Contact{FirstName: "A", LastName: "B", Email: "a@example.invalid"}}

	tests := []struct {
		name    string
		mutate  func(*CreateDomainRequest)
		wantErr string
	}{
		{"unknown service", func(r *CreateDomainRequest) { r.Service = "gold" }, "lite, dns, pro, or enterprise"},
		{"unknown currency", func(r *CreateDomainRequest) { r.Currency = "EUR" }, "CAD or USD"},
		{"term out of range", func(r *CreateDomainRequest) { r.Term = 11 }, "between 1 and 10"},
		{"too many nameservers", func(r *CreateDomainRequest) {
			r.DNSOnly = true
			r.Nameservers = []string{"a.invalid", "b.invalid", "c.invalid", "d.invalid", "e.invalid", "f.invalid", "g.invalid"}
		}, "at most 6 nameservers"},
		{"dns-only with premium", func(r *CreateDomainRequest) { r.DNSOnly = true; r.Premium = true }, "does not apply to a DNS-only domain"},
		{"dns-only with contacts", func(r *CreateDomainRequest) { r.DNSOnly = true; r.Contacts = owner }, "only used when registering"},
		{"registration without owner", func(r *CreateDomainRequest) {}, "requires at least an owner contact"},
		{"premium without price", func(r *CreateDomainRequest) { r.Contacts = owner; r.Premium = true }, "verified premium price"},
		{"price without premium opt-in", func(r *CreateDomainRequest) { r.Contacts = owner; r.PremiumPrice = "45.97" }, "without the premium opt-in"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := base
			test.mutate(&request)
			_, err := buildCreateDomainBody(request)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error=%v, want it to mention %q", err, test.wantErr)
			}
		})
	}
}

func TestBuildCreateDomainBodySelectsDocumentedShapes(t *testing.T) {
	t.Parallel()

	dnsOnly, err := buildCreateDomainBody(CreateDomainRequest{
		Domain: "example.invalid", Service: DomainServiceDNS, Term: 2, Currency: DomainCurrencyCAD,
		DNSOnly: true, Nameservers: []string{"NS2.Example.Invalid.", "ns1.example.invalid"}, DomainGroup: "mygroup",
	})
	if err != nil {
		t.Fatalf("DNS-only body: %v", err)
	}
	if dnsOnly["dns_only"] != 1 || dnsOnly["domain_group"] != "mygroup" {
		t.Fatalf("body=%+v", dnsOnly)
	}
	nameservers, _ := dnsOnly["nameservers"].([]string)
	if len(nameservers) != 2 || nameservers[0] != "ns1.example.invalid" || nameservers[1] != "ns2.example.invalid" {
		t.Fatalf("nameservers were not normalized and sorted: %+v", nameservers)
	}
	if _, hasContacts := dnsOnly["contacts"]; hasContacts {
		t.Fatal("DNS-only body must not carry contacts")
	}

	registration, err := buildCreateDomainBody(CreateDomainRequest{
		Domain: "example.invalid", Service: DomainServicePro, Term: 1, Currency: DomainCurrencyUSD,
		Premium: true, PremiumPrice: "45.97",
		Contacts: &ContactSet{Owner: &Contact{FirstName: "A", LastName: "B", Email: "a@example.invalid"}},
		Extra:    map[string]string{"registrant_type": "individual"},
	})
	if err != nil {
		t.Fatalf("registration body: %v", err)
	}
	if registration["dns_only"] != 0 || registration["premium"] != 1 || registration["premium_price"] != "45.97" {
		t.Fatalf("body=%+v", registration)
	}
	if _, hasExtra := registration["extra"]; !hasExtra {
		t.Fatal("documented TLD extras were dropped")
	}
}

func TestCreateDomainReconcilesEmptyDataWithoutRepeatingTheWrite(t *testing.T) {
	t.Parallel()

	var writes, reads int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPut {
			atomic.AddInt32(&writes, 1)
			_, _ = response.Write([]byte(`{"msg":"OK","status":201}`))
			return
		}
		atomic.AddInt32(&reads, 1)
		_, _ = response.Write([]byte(`{"msg":"OK","data":{"domain":"example.invalid","exists":"N","onsystem":"Y","expiry":false,"next_due":"2030-01-01","service":3}, "status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	created, err := client.CreateDomain(context.Background(), CreateDomainRequest{
		Domain: "example.invalid", Service: DomainServiceDNS, Term: 1, Currency: DomainCurrencyUSD, DNSOnly: true,
	})
	if err != nil {
		t.Fatalf("CreateDomain: %v", err)
	}
	if created.Domain != "example.invalid" {
		t.Fatalf("created=%+v", created)
	}
	if got := atomic.LoadInt32(&writes); got != 1 {
		t.Fatalf("domain creation issued %d writes, want exactly 1", got)
	}
	if atomic.LoadInt32(&reads) == 0 {
		t.Fatal("an empty creation response was not reconciled by a read")
	}
}

func TestDeleteDomainPollsUntilAbsentAndWritesOnce(t *testing.T) {
	t.Parallel()

	var deletes int32
	var reads int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodDelete {
			atomic.AddInt32(&deletes, 1)
			_, _ = response.Write([]byte(`{"msg":"OK","status":200}`))
			return
		}
		if atomic.AddInt32(&reads, 1) == 1 {
			_, _ = response.Write([]byte(`{"msg":"OK","data":{"domain":"example.invalid","exists":"Y","onsystem":"Y","next_due":"","service":1},"status":200}`))
			return
		}
		_, _ = response.Write([]byte(`{"msg":"OK","data":{"domain":"example.invalid","exists":"N","onsystem":"N","next_due":"","service":0},"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	if err := client.DeleteDomain(context.Background(), "example.invalid"); err != nil {
		t.Fatalf("DeleteDomain: %v", err)
	}
	if got := atomic.LoadInt32(&deletes); got != 1 {
		t.Fatalf("domain deletion issued %d writes, want exactly 1", got)
	}
	if atomic.LoadInt32(&reads) < 2 {
		t.Fatal("deletion did not poll until the domain was absent")
	}
}

// Regression guard for the live-contract gap Phase 0 flagged and sandbox
// observation settled: no placeholder user segment is ever sent.
func TestListUserDomainsNeverSendsAPlaceholderUsername(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/domains/list/self" {
			t.Error("client sent the placeholder username EasyDNS rejects with HTTP 400")
		}
		switch request.URL.Path {
		case "/user":
			_, _ = response.Write([]byte(`{"msg":"OK","data":{"user":"example-user"},"status":200}`))
		default:
			_, _ = response.Write([]byte(`{"status":200,"msg":"OK","data":{"user":"example-user","index":[]}}`))
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	user, domains, err := client.ListUserDomains(context.Background(), "")
	if err != nil || user != "example-user" || len(domains) != 0 {
		t.Fatalf("user=%q domains=%+v error=%v", user, domains, err)
	}
}

// An account whose credentials resolve to no username must fail loudly rather
// than fall back to a path segment the API rejects.
func TestListUserDomainsFailsWhenTheAccountHasNoUsername(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"msg":"OK","data":{},"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	if _, _, err := client.ListUserDomains(context.Background(), ""); err == nil {
		t.Fatal("a missing username was not reported")
	}
}

// Sandbox observation: a domain with no subscription block returns
// sub_block "NONE" and an uncloned domain returns cloned_to "NONE". Decoding
// those as integers and strings used to fail the whole read.
func TestGetDomainAcceptsNoneSentinels(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"msg":"OK","tm":1788366281,"data":{"id":"example.invalid","domain":"example.invalid","exists":"Y","onsystem":"Y","expiry":false,"next_due":"2026-11-05","cloned_to":"NONE","service":"2423","sub_block":"NONE"},"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	domain, err := client.GetDomain(context.Background(), "example.invalid")
	if err != nil {
		t.Fatalf("GetDomain: %v", err)
	}
	if domain.SubscriptionID != 0 {
		t.Errorf("sub_block %q should decode as absent, got %d", "NONE", domain.SubscriptionID)
	}
	if domain.ClonedTo != "" {
		t.Errorf("cloned_to %q should decode as absent, got %q", "NONE", domain.ClonedTo)
	}
	if domain.Expiry != "" {
		t.Errorf("expiry false should decode as absent, got %q", domain.Expiry)
	}
	if domain.Service != 2423 || domain.NextDue != "2026-11-05" {
		t.Errorf("real values were lost: %+v", domain)
	}
}

// A sentinel must not swallow a genuinely malformed integer.
func TestFlexibleIntStillRejectsMalformedValues(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"msg":"OK","data":{"domain":"example.invalid","exists":"Y","onsystem":"Y","next_due":"","service":"not-a-number"},"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	if _, err := client.GetDomain(context.Background(), "example.invalid"); err == nil {
		t.Fatal("a malformed integer was silently accepted")
	}
}

// Observed shape: each domain is a numerically keyed sibling of user rather
// than a member of the documented index array. Reading the documented form
// only, the client reported an account with domains as empty.
func TestListUserDomainsDecodesNumericallyKeyedEntries(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/user":
			_, _ = response.Write([]byte(`{"msg":"OK","data":{"user":"example-user"},"status":200}`))
		default:
			_, _ = response.Write([]byte(`{"msg":"OK","data":{"user":"example-user",
				"0":{"name":"zeta.invalid","link":"https://example.invalid/domain/zeta.invalid"},
				"1":{"name":"alpha.invalid","link":"https://example.invalid/domain/alpha.invalid"},
				"2":{"name":"middle.invalid","link":"https://example.invalid/domain/middle.invalid"}},
				"total":3,"count":3,"start":0,"max":1000,"status":200}`))
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	user, domains, err := client.ListUserDomains(context.Background(), "")
	if err != nil {
		t.Fatalf("ListUserDomains: %v", err)
	}
	if user != "example-user" {
		t.Errorf("user=%q", user)
	}
	if len(domains) != 3 {
		t.Fatalf("expected 3 domains, got %d: %+v", len(domains), domains)
	}
	if domains[0].Domain != "alpha.invalid" || domains[2].Domain != "zeta.invalid" {
		t.Fatalf("not sorted by domain: %+v", domains)
	}
	for _, domain := range domains {
		if domain.Domain == "example-user" {
			t.Fatal("the user field was decoded as a domain")
		}
	}
}

// EasyDNS can answer a domain delete with 200 and keep reporting the domain.
// That must not be reported as a successful deletion.
func TestDeleteDomainReportsAnAcceptedButUnobservedDeletion(t *testing.T) {
	t.Parallel()

	var deletes int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodDelete {
			atomic.AddInt32(&deletes, 1)
			_, _ = response.Write([]byte(`{"msg":"OK","data":{"domain":"example.invalid"},"status":200}`))
			return
		}
		_, _ = response.Write([]byte(`{"msg":"OK","data":{"domain":"example.invalid","exists":"N","onsystem":"Y","next_due":"2027-09-02","service":"2424"},"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	err := client.DeleteDomain(context.Background(), "example.invalid")
	if err == nil {
		t.Fatal("a domain that survived deletion was reported as deleted")
	}
	var notObservable *DomainDeletionNotObservableError
	if !errors.As(err, &notObservable) {
		t.Fatalf("error=%T %v", err, err)
	}
	if !strings.Contains(err.Error(), "dashboard") {
		t.Errorf("diagnostic does not tell the operator where to look: %v", err)
	}
	if got := atomic.LoadInt32(&deletes); got != 1 {
		t.Fatalf("deletion issued %d writes, want exactly 1", got)
	}
}
