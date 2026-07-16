package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDataAzSubscription_Schema(t *testing.T) {
	r := dataAzSubscription()
	if err := r.InternalValidate(nil, false); err != nil {
		t.Fatalf("schema invalid: %v", err)
	}
	if !r.Schema["marketplace_subscription_id"].Required {
		t.Error("marketplace_subscription_id should be Required")
	}
	for _, name := range []string{"subscription_id", "display_name", "payment_plan_id", "owner_id"} {
		if !r.Schema[name].Computed {
			t.Errorf("%q should be Computed", name)
		}
	}
}

func TestDataAzSubscriptionRead_PopulatesAttributes(t *testing.T) {
	// Response shape lifted from OpenAPI spec example for
	// GET /v1/subscriptions/{subscriptionId}.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/v1/subscriptions/7c9a1342-3091-4317-b4fe-0d8a68c18acd" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("auth = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"data":{
				"id":"7c9a1342-3091-4317-b4fe-0d8a68c18acd",
				"externalAccountId":"1ff979cb-acbf-461d-a471-57f6684ec086",
				"label":"Production Tenant",
				"status":"ACTIVE",
				"user":{"id":"98671c7c-a75c-406a-b536-51c072c05bdd"},
				"order":{
					"uuid":"94763dc6-a252-4682-8ac1-0bb6f2f484e6",
					"paymentPlan":{"id":221442,"uuid":"15592bec-d0b1-4d14-89d3-a628e1a50bfe"}
				}
			}
		}`))
	}))
	defer srv.Close()

	d := schemaResourceDataFromRaw(t, dataAzSubscription().Schema, map[string]interface{}{
		"marketplace_subscription_id": "7c9a1342-3091-4317-b4fe-0d8a68c18acd",
	})
	if err := dataAzSubscriptionRead(d, newTestConfig(srv)); err != nil {
		t.Fatalf("Read: %v", err)
	}

	if d.Id() != "7c9a1342-3091-4317-b4fe-0d8a68c18acd" {
		t.Errorf("Id = %q", d.Id())
	}
	if d.Get("subscription_id").(string) != "1ff979cb-acbf-461d-a471-57f6684ec086" {
		t.Errorf("subscription_id = %q", d.Get("subscription_id"))
	}
	if d.Get("display_name").(string) != "Production Tenant" {
		t.Errorf("display_name = %q", d.Get("display_name"))
	}
	if d.Get("owner_id").(string) != "98671c7c-a75c-406a-b536-51c072c05bdd" {
		t.Errorf("owner_id = %q", d.Get("owner_id"))
	}
	if d.Get("payment_plan_id").(int) != 221442 {
		t.Errorf("payment_plan_id = %d", d.Get("payment_plan_id"))
	}
}

func TestDataAzSubscriptionRead_Non200Errors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusNotFound)
	}))
	defer srv.Close()

	d := schemaResourceDataFromRaw(t, dataAzSubscription().Schema, map[string]interface{}{
		"marketplace_subscription_id": "missing",
	})
	if err := dataAzSubscriptionRead(d, newTestConfig(srv)); err == nil {
		t.Fatal("expected error for 404 response")
	}
}
