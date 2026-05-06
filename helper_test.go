package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

	got, err := subscriptionInfo("sub-1", newTestConfig(srv))
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

	if _, err := subscriptionInfo("x", newTestConfig(srv)); err == nil {
		t.Fatal("expected error for 500 response")
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
	sub := CSPSubscription{Id: "sub-9", Label: &label}
	if err := changeSubscription(sub, newTestConfig(srv)); err != nil {
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
	if sawCorr != "106" {
		t.Errorf("X-Correlation-ID = %q", sawCorr)
	}

	var roundTrip CSPSubscription
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

	err := changeSubscription(CSPSubscription{Id: "x"}, newTestConfig(srv))
	if err == nil || !strings.Contains(err.Error(), "failed to update subscription") {
		t.Fatalf("expected update error, got %v", err)
	}
}

func TestCancelSubscription_NoAzureCredsErrors(t *testing.T) {
	cfg := &Config{} // AzureAuthCtx nil
	err := cancelSubscription("sub-1", cfg)
	if err == nil || !strings.Contains(err.Error(), "cannot authenticate with Azure API") {
		t.Fatalf("expected azure auth error, got %v", err)
	}
}
