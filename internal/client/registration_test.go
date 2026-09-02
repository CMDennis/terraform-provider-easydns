package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// The pinned contract types getRegStatus as a single-domain object while the
// operation returns a whole account, so decoding must accept every plausible
// shape rather than guessing one.
func TestRegistrationStatusDecodesEveryDocumentedShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "keyed by domain",
			body: `{"msg":"OK","status":200,"data":{"zeta.invalid":{"reglock":true,"renewal":"renew","auto_renew":false,"let_expire":false,"expiry":"2030-01-01","local_registrar":true,"supports_reglock":true},"alpha.invalid":{"reglock":false,"renewal":"expire","auto_renew":true,"let_expire":true,"expiry":"2031-01-01","local_registrar":true,"supports_reglock":false}}}`,
			want: []string{"alpha.invalid", "zeta.invalid"},
		},
		{
			name: "array of self-naming objects",
			body: `{"msg":"OK","status":200,"data":[{"domain":"zeta.invalid","reglock":1,"renewal":"renew","supports_reglock":1},{"domain":"alpha.invalid","reglock":"N","renewal":"remind","supports_reglock":"Y"}]}`,
			want: []string{"alpha.invalid", "zeta.invalid"},
		},
		{
			name: "bare single-domain object",
			body: `{"msg":"OK","status":200,"data":{"domain":"only.invalid","reglock":true,"renewal":"renew","supports_reglock":true}}`,
			want: []string{"only.invalid"},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				_, _ = response.Write([]byte(test.body))
			}))
			defer server.Close()

			client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
			statuses, err := client.ListRegistrationStatuses(context.Background())
			if err != nil {
				t.Fatalf("ListRegistrationStatuses: %v", err)
			}
			if len(statuses) != len(test.want) {
				t.Fatalf("statuses=%+v", statuses)
			}
			for index, domain := range test.want {
				if statuses[index].Domain != domain {
					t.Fatalf("status %d = %q, want %q (sorted)", index, statuses[index].Domain, domain)
				}
			}
		})
	}
}

func TestGetRegistrationStatusReportsMissingDomainAsNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"msg":"OK","status":200,"data":{"other.invalid":{"reglock":true,"renewal":"renew","supports_reglock":true}}}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	_, err := client.GetRegistrationStatus(context.Background(), "example.invalid")
	if !IsNotFound(err) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestSetRegistrationSettingsSendsDomainKeyedBodyAndWritesOnce(t *testing.T) {
	t.Parallel()

	var writes int32
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			atomic.AddInt32(&writes, 1)
			body, _ = io.ReadAll(request.Body)
			_, _ = response.Write([]byte(`{"msg":"OK","status":200}`))
			return
		}
		_, _ = response.Write([]byte(`{"msg":"OK","status":200,"data":{"example.invalid":{"reglock":true,"renewal":"renew","supports_reglock":true}}}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	status, err := client.SetRegistrationSettings(context.Background(), RegistrationSettingsRequest{
		Domain: "Example.Invalid.", Reglock: true, Renewal: RenewalRenew,
	})
	if err != nil {
		t.Fatalf("SetRegistrationSettings: %v", err)
	}
	if status.Domain != "example.invalid" || !status.Reglock || status.Renewal != "renew" {
		t.Fatalf("status=%+v", status)
	}
	if got := atomic.LoadInt32(&writes); got != 1 {
		t.Fatalf("registration update issued %d writes, want exactly 1", got)
	}

	// The array form the schema describes is refused by EasyDNS with HTTP 406;
	// the domain-keyed object from its example is what works.
	var sent map[string]map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("request body is not the domain-keyed shape: %v (%s)", err, body)
	}
	entry, ok := sent["example.invalid"]
	if !ok || len(sent) != 1 {
		t.Fatalf("sent=%+v", sent)
	}
	if entry["reglock"] != true || entry["renewal"] != "renew" {
		t.Fatalf("entry=%+v", entry)
	}
}

// A TLD that cannot reglock never reports the requested value, so matching on
// it would poll until the deadline instead of settling.
func TestRegistrationSettingsIgnoreReglockOnUnsupportedTLD(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			_, _ = response.Write([]byte(`{"msg":"OK","status":200}`))
			return
		}
		_, _ = response.Write([]byte(`{"msg":"OK","status":200,"data":{"example.invalid":{"reglock":false,"renewal":"renew","supports_reglock":false}}}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	status, err := client.SetRegistrationSettings(context.Background(), RegistrationSettingsRequest{
		Domain: "example.invalid", Reglock: true, Renewal: RenewalRenew,
	})
	if err != nil {
		t.Fatalf("SetRegistrationSettings: %v", err)
	}
	if status.SupportsReglock {
		t.Fatalf("status=%+v", status)
	}
}

func TestParseRenewalActionRejectsUnknownPolicy(t *testing.T) {
	t.Parallel()

	for _, valid := range []string{"remind", "renew", "expire"} {
		if _, err := ParseRenewalAction(valid); err != nil {
			t.Errorf("ParseRenewalAction(%q)=%v", valid, err)
		}
	}
	if _, err := ParseRenewalAction("cancel"); err == nil || !strings.Contains(err.Error(), "remind, renew, or expire") {
		t.Fatalf("error=%v", err)
	}
}

// Observed sandbox shape: the domain-keyed map is nested under data.domains
// alongside scalar siblings such as user, which must not be read as domains.
func TestRegistrationStatusDecodesNestedDomainsEnvelope(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"msg":"OK","tm":1788367325,"data":{"domains":{
			"zeta.invalid":{"reglock":false,"let_expire":false,"renewal":"remind","local_registrar":true,"expiry":"2026-11-12","auto_renew":false,"auto_renew_card_id":false},
			"alpha.invalid":{"reglock":true,"let_expire":false,"renewal":"renew","local_registrar":true,"expiry":"2026-11-05","auto_renew":false,"auto_renew_card_id":false}},
			"user":"tester"},"status":200}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil, RecordWriteModeSynchronous, nil)
	statuses, err := client.ListRegistrationStatuses(context.Background())
	if err != nil {
		t.Fatalf("ListRegistrationStatuses: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 domains, got %d: %+v", len(statuses), statuses)
	}
	if statuses[0].Domain != "alpha.invalid" || statuses[1].Domain != "zeta.invalid" {
		t.Fatalf("not sorted by domain: %+v", statuses)
	}
	if statuses[0].Expiry != "2026-11-05" || !statuses[0].Reglock {
		t.Fatalf("fields lost: %+v", statuses[0])
	}
	for _, status := range statuses {
		if status.Domain == "user" || status.Domain == "domains" {
			t.Fatalf("envelope metadata was decoded as a domain: %+v", status)
		}
	}
}

// An omitted supports_reglock must not be read as "this TLD cannot reglock",
// which would report an unapplied reglock as a successful change.
func TestReglockIsVerifiedWhenSupportIsNotReported(t *testing.T) {
	t.Parallel()

	unreported := RegistrationStatus{Renewal: "renew", Reglock: false, SupportsReglockReported: false}
	desired := RegistrationSettingsRequest{Renewal: RenewalRenew, Reglock: true}
	if registrationSettingsMatch(unreported, desired) {
		t.Error("an unapplied reglock was accepted because support was not reported")
	}

	explicitlyUnsupported := RegistrationStatus{Renewal: "renew", Reglock: false, SupportsReglockReported: true, SupportsReglock: false}
	if !registrationSettingsMatch(explicitlyUnsupported, desired) {
		t.Error("a TLD that explicitly cannot reglock should not be polled forever")
	}
}
