package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestMetadataEndpointsDecodeContractShapes(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/user":
			_, _ = fmt.Fprint(response, `{"status":"200","data":{"user":"tester","first_name":"Ada","last_name":"Lovelace","org_name":"Example","address1":"1 Main","address2":"Suite 2","address3":"Desk 3","city":"Toronto","state":"ON","country":"CA","postal_code":"A1A 1A1","currency":"CAD","phone":"+14165550100","cellphone":"+14165550101","fax":"+14165550102","email":"ada@example.invalid","email2":"alt@example.invalid","notices_email":"notice@example.invalid","public_email":"public@example.invalid","alerts_email":"alert@example.invalid","url":"https://example.invalid","opt_out":"1","beta":2}}`)
		case "/services/61/description":
			_, _ = fmt.Fprint(response, `{"status":200,"data":{"service_id":"61","name":"DNS Standard","period":"12","enterprise":"0","description":"Standard DNS"}}`)
		case "/services/subscription/9001/description":
			_, _ = fmt.Fprint(response, `{"status":200,"data":{"subscription_id":"9001","service_id":61,"name":"DNS Block","period":12,"enterprise":1,"description":"Block","size":"10"}}`)
		default:
			http.Error(response, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, nil)

	user, err := client.GetCurrentUser(context.Background())
	if err != nil || user.Username != "tester" || user.Address3 != "Desk 3" || user.OptOut != 1 || user.Beta != 2 {
		t.Fatalf("user=%+v error=%v", user, err)
	}
	service, err := client.GetServiceDescription(context.Background(), 61)
	if err != nil || service.ServiceID != 61 || service.Period != 12 || service.Enterprise {
		t.Fatalf("service=%+v error=%v", service, err)
	}
	subscription, err := client.GetSubscriptionServiceDescription(context.Background(), 9001)
	if err != nil || subscription.SubscriptionID != 9001 || subscription.ServiceID != 61 || !subscription.Enterprise || subscription.Size != 10 {
		t.Fatalf("subscription=%+v error=%v", subscription, err)
	}
}

func TestDomainPricingPOSTIsRetriedAsReadAndPreservesDecimals(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/domains/service/check/example.invalid" {
			http.Error(response, "unexpected request", http.StatusNotFound)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode pricing body: %v", err)
		}
		if !reflect.DeepEqual(body, map[string]any{"service": "dns", "min_term": float64(1), "max_term": float64(2)}) {
			t.Errorf("pricing body=%#v", body)
		}
		if attempts.Add(1) == 1 {
			response.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprint(response, `{"status":503,"msg":"temporary"}`)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(response, `{"status":200,"data":{"domain":"example.invalid","avail":"1","tld":"invalid","services":[{"id":"62","name":"Pro","code":"pro","currency":"CAD","price":"47.4300","isPremium":"0","pricePeriod":"2","pricePeriodName":"year","tax1":2.37,"tax2":"0.00","tax3":0}]}}`)
	}))
	defer server.Close()
	fake := newFakeTime()
	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, func(config *Config) {
		config.RetryPolicy.MaxAttempts = 2
		config.RetryPolicy.InitialDelay = time.Millisecond
		config.RetryPolicy.MaxDelay = time.Millisecond
		config.Clock = fake
		config.Waiter = fake
	})

	pricing, err := client.GetDomainPricing(context.Background(), DomainPricingRequest{Domain: "example.invalid", Service: "dns", MinTerm: 1, MaxTerm: 2})
	if err != nil {
		t.Fatalf("get pricing: %v", err)
	}
	if attempts.Load() != 2 || !pricing.Available || len(pricing.Services) != 1 || pricing.Services[0].Price != "47.4300" || pricing.Services[0].Tax1 != "2.37" {
		t.Fatalf("attempts=%d pricing=%+v", attempts.Load(), pricing)
	}
}

func TestDomainPricingFailureIsNotAnAmbiguousWrite(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		response.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprint(response, `{"status":503,"msg":"temporary"}`)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, server.Client(), RecordWriteModeSynchronous, func(config *Config) {
		config.RetryPolicy.MaxAttempts = 2
	})

	_, err := client.GetDomainPricing(context.Background(), DomainPricingRequest{Domain: "example.invalid"})
	if err == nil || IsAmbiguousWrite(err) || attempts.Load() != 2 {
		t.Fatalf("attempts=%d error=%T %v", attempts.Load(), err, err)
	}
}
