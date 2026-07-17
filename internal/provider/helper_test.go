package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"terraform-provider-cancom-marketplace/internal/azure"
	"terraform-provider-cancom-marketplace/internal/marketplace"
)

func TestSubscriptionInfo_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v1/subscriptions/sub-1" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"id":"sub-1","label":"Hello","user":{"id":"user-1"},"order":{"paymentPlan":{"id":42}}}}`))
	}))
	defer srv.Close()

	got, err := newTestConfig(srv).Marketplace.SubscriptionInfo("sub-1")
	if err != nil {
		t.Fatalf("subscriptionInfo: %v", err)
	}
	if got.Id != "sub-1" {
		t.Errorf("Id = %q", got.Id)
	}
	if got.Label == nil || *got.Label != "Hello" {
		t.Errorf("Label = %v", got.Label)
	}
	if got.User.Id != "user-1" {
		t.Errorf("User.Id = %q", got.User.Id)
	}
	if got.Order.PaymentPlan.Id != 42 {
		t.Errorf("Order.PaymentPlan.Id = %d", got.Order.PaymentPlan.Id)
	}
}

func TestSubscriptionInfo_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := newTestConfig(srv).Marketplace.SubscriptionInfo("x"); err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestSubscriptionResponse_AcceptsStringPreviousOrderIDAndPreservesUnknownFields(t *testing.T) {
	body := []byte(`{"data":{"id":"sub-1","order":{"previousOrder":{"id":"7922581"}},"futureApiField":{"keep":true}}}`)
	sub, document, err := marketplace.SubscriptionResponse(body)
	if err != nil {
		t.Fatalf("subscriptionResponse: %v", err)
	}
	if sub.Order.PreviousOrder == nil || sub.Order.PreviousOrder.Id != "7922581" {
		t.Errorf("previous order ID = %#v", sub.Order.PreviousOrder)
	}
	if _, ok := document["futureApiField"]; !ok {
		t.Error("raw document did not preserve unknown field")
	}
}

func TestSubscriptionResponse_AcceptsRealAzureSubscriptionShape(t *testing.T) {
	body := []byte(`{
		"data": {
			"id": "7bbbfb19-c61f-461f-9cd7-cc8411b865eb",
			"externalAccountId": "1ff979cb-acbf-461d-a471-57f6684ec086",
			"maxUsers": null,
			"order": {
				"endDate": null,
				"billingEndDate": null,
				"paymentPlan": {"id": 172495, "contract": null},
				"contract": null,
				"paymentPlanId": 172495,
				"discountId": null,
				"oneTimeOrders": [{"paymentPlan": {"id": "172495"}, "id": 9591217}],
				"customAttributes": [{"name": "dnbDisabled", "attributeType": "MULTISELECT", "valueKeys": ["yes"]}]
			}
		}
	}`)
	sub, _, err := marketplace.SubscriptionResponse(body)
	if err != nil {
		t.Fatalf("subscriptionResponse: %v", err)
	}
	if sub.Id != "7bbbfb19-c61f-461f-9cd7-cc8411b865eb" {
		t.Errorf("marketplace ID = %q", sub.Id)
	}
	if sub.ExternalAccountId != "1ff979cb-acbf-461d-a471-57f6684ec086" {
		t.Errorf("Azure subscription ID = %q", sub.ExternalAccountId)
	}
	if sub.MaxUsers != nil || sub.Order.EndDate != nil || sub.Order.BillingEndDate != nil || sub.Order.Contract != nil || sub.Order.PaymentPlan.Contract != nil {
		t.Error("nullable API fields were not decoded as nil")
	}
	if sub.Order.CustomAttributes == nil || len(*sub.Order.CustomAttributes) != 1 || len((*sub.Order.CustomAttributes)[0].ValueKeys) != 1 {
		t.Errorf("custom attributes = %#v", sub.Order.CustomAttributes)
	}
}

func TestChangeSubscription_PutsToCorrectURLWithBody(t *testing.T) {
	var sawMethod, sawPath, sawAuth, sawCorr string
	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawMethod = r.Method
		sawPath = r.URL.Path
		sawAuth = r.Header.Get("Authorization")
		sawCorr = r.Header.Get("X-Correlation-ID")
		sawBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	label := "renamed"
	sub := marketplace.Subscription{Id: "sub-9", Label: &label}
	if err := newTestConfig(srv).Marketplace.ChangeSubscription(sub); err != nil {
		t.Fatalf("changeSubscription: %v", err)
	}

	if sawMethod != http.MethodPut {
		t.Errorf("method = %s", sawMethod)
	}
	if sawPath != "/v1/subscriptions" {
		t.Errorf("path = %s, want /v1/subscriptions (id is in body, not path)", sawPath)
	}
	if sawAuth != "Bearer test-token" {
		t.Errorf("auth = %q", sawAuth)
	}
	if sawCorr == "" {
		t.Error("X-Correlation-ID should be set")
	} else if _, err := strconv.ParseUint(sawCorr, 10, 64); err != nil {
		t.Errorf("X-Correlation-ID = %q, want a numeric value", sawCorr)
	}

	var roundTrip marketplace.Subscription
	if err := json.Unmarshal(sawBody, &roundTrip); err != nil {
		t.Fatalf("body not json: %v\nraw=%s", err, sawBody)
	}
	if roundTrip.Id != "sub-9" || roundTrip.Label == nil || *roundTrip.Label != "renamed" {
		t.Errorf("round-trip body = %+v label=%v", roundTrip, roundTrip.Label)
	}
}

func TestChangeSubscription_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	err := newTestConfig(srv).Marketplace.ChangeSubscription(marketplace.Subscription{Id: "x"})
	if err == nil || !strings.Contains(err.Error(), "failed to update subscription") {
		t.Fatalf("expected update error, got %v", err)
	}
}

func TestCancelSubscription_NoAzureCredsErrors(t *testing.T) {
	err := (&azure.Client{}).CancelSubscription(context.Background(), "sub-1")
	if err == nil || !strings.Contains(err.Error(), "cannot authenticate with Azure API") {
		t.Fatalf("expected azure auth error, got %v", err)
	}
}
