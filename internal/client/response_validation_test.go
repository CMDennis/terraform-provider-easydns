package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func responseClient(t *testing.T, body string) *Client {
	t.Helper()
	return newTestClient(t, "https://example.invalid", &http.Client{Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}, RecordWriteModeSynchronous, nil)
}

func TestMalformedDomainReadsCannotEstablishAbsence(t *testing.T) {
	for _, body := range []string{
		`{}`, `null`, `{"status":200}`, `{"status":200,"data":null}`, `{"status":200,"data":{}}`,
		`{"data":{"domain":"example.invalid","exists":"N","onsystem":"N"}}`,
		`{"status":200,"data":{"domain":"example.invalid","exists":"N"}}`,
		`{"status":200,"data":{"domain":"example.invalid","exists":"N","onsystem":null}}`,
		`{"status":200,"data":{"domain":"example.invalid","exists":"N","onsystem":""}}`,
		`{"status":200,"data":{"domain":"example.invalid","exists":"N","onsystem":"unexpected"}}`,
		`{"status":200,"data":{"domain":"example.invalid","exists":"N","onsystem":2}}`,
		`{"status":200,"data":{"domain":"other.invalid","exists":"N","onsystem":"N"}}`,
		`{"status":200,"data":{"exists":"N","onsystem":"N"}}`,
		`{"status":200,"data":{"domain":"example.invalid","exists":"N","onsystem":"N"}} {}`,
	} {
		_, err := responseClient(t, body).GetDomain(context.Background(), "example.invalid")
		if err == nil || IsNotFound(err) {
			t.Fatalf("malformed response treated as authoritative: %s; %v", body, err)
		}
	}
	for _, negative := range []string{`"N"`, `"no"`, `"false"`, `"0"`, `false`, `0`} {
		body := fmt.Sprintf(`{"status":200,"data":{"domain":"Example.Invalid.","exists":false,"onsystem":%s}}`, negative)
		_, err := responseClient(t, body).GetDomain(context.Background(), "example.invalid")
		if !IsNotFound(err) {
			t.Fatalf("explicit negative was not recognized: %v", err)
		}
	}
}

func TestMalformedCollectionsCannotEstablishAbsence(t *testing.T) {
	for _, test := range []struct {
		name    string
		read    func(*Client) error
		invalid []string
		empty   string
	}{
		{
			name:    "records",
			read:    func(c *Client) error { _, err := c.GetRecord(context.Background(), "example.invalid", "1"); return err },
			invalid: []string{`{"status":200,"data":{}}`, `{"status":200,"data":[{}]}`, `{"status":200,"data":[{"id":"2","domain":"other.invalid","host":"@","type":"A"}]}`, `{"status":200,"data":[],"count":1,"total":0}`},
			empty:   `{"status":200,"data":[]}`,
		},
		{
			name: "mailmaps",
			read: func(c *Client) error {
				_, err := c.GetMailmap(context.Background(), "example.invalid", "1")
				return err
			},
			invalid: []string{`{"status":200,"data":{}}`, `{"status":200,"data":{"domain":"example.invalid","mailmaps":null}}`, `{"status":200,"data":{"domain":"other.invalid","mailmaps":[]}}`, `{"status":200,"data":{"domain":"example.invalid","mailmaps":[{}]}}`},
			empty:   `{"status":200,"data":{"domain":"example.invalid","mailmaps":[]}}`,
		},
		{
			name: "glue",
			read: func(c *Client) error {
				_, err := c.GetGlueRecord(context.Background(), "example.invalid", "ns1.example.invalid")
				return err
			},
			invalid: []string{`{"status":200,"data":{}}`, `{"status":200,"data":{"domain":"example.invalid","glue_records":null}}`, `{"status":200,"data":{"domain":"other.invalid","glue_records":[]}}`, `{"status":200,"data":{"domain":"example.invalid","glue_records":[],"total":1}}`},
			empty:   `{"status":200,"data":{"domain":"example.invalid","glue_records":[]}}`,
		},
		{
			name: "registration",
			read: func(c *Client) error {
				_, err := c.GetRegistrationStatus(context.Background(), "example.invalid")
				return err
			},
			invalid: []string{`{"status":200,"data":{"domains":null}}`, `{"status":200,"data":{"user":"synthetic"}}`, `{"status":200,"data":[{}]}`, `{"status":200,"data":{"example.invalid":{}}}`, `{"status":200,"data":{"example.invalid":{"domain":"other.invalid","reglock":false,"renewal":"renew"}}}`},
			empty:   `{"status":200,"data":{"domains":{}}}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			invalid := append([]string{`{}`, `{"status":200}`, `{"status":200,"data":null}`, `{"data":[]}`}, test.invalid...)
			for _, body := range invalid {
				if err := test.read(responseClient(t, body)); err == nil || IsNotFound(err) {
					t.Fatalf("incomplete collection accepted: %s; %v", body, err)
				}
			}
			if err := test.read(responseClient(t, test.empty)); !IsNotFound(err) {
				t.Fatalf("explicit empty collection rejected: %v", err)
			}
		})
	}
}

func TestMalformedZoneAndNameserverReadsFail(t *testing.T) {
	for _, body := range []string{`{}`, `{"status":200,"data":{}}`, `{"status":200,"data":null}`} {
		c := responseClient(t, body)
		if _, err := c.GetZone(context.Background(), "example.invalid"); err == nil {
			t.Fatal("incomplete zone accepted")
		}
		if _, err := c.GetDomainNameservers(context.Background(), "example.invalid"); err == nil {
			t.Fatal("incomplete delegation accepted")
		}
	}
}

func TestErrorCodesCannotTurnRedirectsOrDeniedReadsIntoAbsence(t *testing.T) {
	for _, status := range []int{301, 302, 307, 403, 500} {
		if IsNotFound(&APIError{HTTPStatus: status, Code: 404}) {
			t.Fatalf("HTTP %d was interpreted as authoritative absence", status)
		}
	}
	for _, status := range []int{200, 404} {
		if !IsNotFound(&APIError{HTTPStatus: status, Code: 404}) {
			t.Fatalf("HTTP %d absence was not recognized", status)
		}
	}
}

func TestConfirmedDomainVisibilityIsPolledWithoutAnotherWrite(t *testing.T) {
	reads, writes := 0, 0
	fake := newFakeTime()
	c := newTestClient(t, "https://example.invalid", &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		status, body := 200, `{"status":200,"data":{"domain":"example.invalid","exists":"N","onsystem":"Y"}}`
		if request.Method == http.MethodPut {
			writes++
			status, body = 201, `{"status":201,"data":{"domain":"example.invalid","inv_id":123}}`
		} else {
			reads++
			if reads < 3 {
				status, body = 404, `{"status":404}`
			}
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}, RecordWriteModeSynchronous, func(config *Config) {
		config.Clock, config.Waiter = fake, fake
		config.RecordPollInterval, config.RecordReconcileTimeout = time.Second, 5*time.Second
	})
	created, err := c.CreateDomain(context.Background(), CreateDomainRequest{Domain: "example.invalid", Service: DomainServiceDNS, Term: 1, Currency: DomainCurrencyUSD, DNSOnly: true})
	if err != nil || created == nil || created.InvoiceID != 123 {
		t.Fatalf("creation not confirmed: %v", err)
	}
	domain, err := c.WaitForDomain(context.Background(), created.Domain)
	if err != nil || domain == nil || !domain.OnSystem || reads != 3 || writes != 1 || len(fake.Waits()) != 2 {
		t.Fatalf("visibility polling failed: reads=%d writes=%d error=%v", reads, writes, err)
	}
}
