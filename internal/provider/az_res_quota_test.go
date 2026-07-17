package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestResourceAzSubscriptionQuota_Schema(t *testing.T) {
	r := resourceAzSubscriptionQuota()
	if err := r.InternalValidate(nil, true); err != nil {
		t.Fatalf("schema invalid: %v", err)
	}
	for _, name := range []string{"subscription_id", "provider_namespace", "location", "quota_resource", "limit"} {
		if !r.Schema[name].Required {
			t.Errorf("%q should be Required", name)
		}
	}
	if !r.Schema["quota_family"].Optional || r.Schema["quota_family"].Deprecated == "" {
		t.Error("quota_family should be an optional deprecated compatibility field")
	}
	for _, name := range []string{"provisioning_state", "request_id"} {
		if !r.Schema[name].Computed {
			t.Errorf("%q should be Computed", name)
		}
	}
}

func TestResourceAzSubscriptionQuotaCreate_PutsAndCapturesRequestId(t *testing.T) {
	wantPath := "/v1/microsoft/quota/subscriptions/sub-1/providers/Microsoft.Compute/locations/westeurope/providers/Microsoft.Quota/quotas/standardDv2Family"
	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != wantPath {
			t.Errorf("path = %s\nwant = %s", r.URL.Path, wantPath)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("auth = %q", got)
		}
		sawBody, _ = io.ReadAll(r.Body)
		w.Write([]byte(`{"data":{"id":"/v1/microsoft/quota/subscriptions/sub-1/...","name":"req-42","type":"Microsoft.Quota/Quotas","properties":{"provisioningState":"Succeeded"}}}`))
	}))
	defer srv.Close()

	d := schemaResourceDataFromRaw(t, resourceAzSubscriptionQuota().Schema, map[string]interface{}{
		"subscription_id":    "sub-1",
		"provider_namespace": "Microsoft.Compute",
		"location":           "westeurope",
		"quota_family":       "standardDFamily",
		"quota_resource":     "standardDv2Family",
		"limit":              100,
	})

	if err := resourceAzSubscriptionQuotaCreate(d, newTestConfig(srv)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if d.Id() != "req-42" {
		t.Errorf("Id = %q", d.Id())
	}
	if d.Get("request_id").(string) != "req-42" {
		t.Errorf("request_id = %q", d.Get("request_id"))
	}

	var body map[string]interface{}
	if err := json.Unmarshal(sawBody, &body); err != nil {
		t.Fatalf("body not json: %v\nraw=%s", err, sawBody)
	}
	props, _ := body["properties"].(map[string]interface{})
	limit, _ := props["limit"].(map[string]interface{})
	// API contract: limit.value is a JSON string, not an integer.
	if s, _ := limit["value"].(string); s != "100" {
		t.Errorf(`limit.value = %v, want "100" (string)`, limit["value"])
	}
	if s, _ := limit["limitObjectType"].(string); s != "LimitValue" {
		t.Errorf("limit.limitObjectType = %v", limit["limitObjectType"])
	}
	name, _ := props["name"].(map[string]interface{})
	if s, _ := name["value"].(string); s != "standardDv2Family" {
		t.Errorf("name.value = %v", name["value"])
	}
	innerProps, _ := props["properties"].(map[string]interface{})
	if s, _ := innerProps["requestOrigin"].(string); s != "Microsoft_Azure_Capacity/QuotaApproval.ReactView" {
		t.Errorf("properties.requestOrigin = %v", innerProps["requestOrigin"])
	}
}

func TestResourceAzSubscriptionQuotaCreate_Non2xxReturnsErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadRequest)
	}))
	defer srv.Close()

	d := schemaResourceDataFromRaw(t, resourceAzSubscriptionQuota().Schema, map[string]interface{}{
		"subscription_id":    "s",
		"provider_namespace": "Microsoft.Compute",
		"location":           "we",
		"quota_family":       "f",
		"quota_resource":     "r",
		"limit":              1,
	})
	err := resourceAzSubscriptionQuotaCreate(d, newTestConfig(srv))
	if err == nil || !strings.Contains(err.Error(), "failed to set subscription quota") {
		t.Fatalf("expected set-quota error, got %v", err)
	}
}

func TestResourceAzSubscriptionQuotaRead_PopulatesValues(t *testing.T) {
	wantPath := "/v1/microsoft/quota/subscriptions/sub-1/providers/Microsoft.Compute/locations/westeurope/providers/Microsoft.Quota/quotaRequests/req-42"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath {
			t.Errorf("path = %s\nwant = %s", r.URL.Path, wantPath)
		}
		w.Write([]byte(`{"data":{"id":"/v1/microsoft/quota/subscriptions/sub-1/...","name":"req-42","type":"Microsoft.Quota/QuotaRequests","properties":{"message":"Request completed.","provisioningState":"Succeeded","value":[{"limit":{"limitObjectType":"LimitValue","limitType":"Independent","value":100},"name":{"value":"STANDARDDV2FAMILY"},"provisioningState":"Succeeded","subRequestId":"req-42"}]}}}`))
	}))
	defer srv.Close()

	d := schemaResourceDataFromRaw(t, resourceAzSubscriptionQuota().Schema, map[string]interface{}{
		"subscription_id":    "sub-1",
		"provider_namespace": "Microsoft.Compute",
		"location":           "westeurope",
		"quota_family":       "f",
		"quota_resource":     "standardDv2Family",
		"limit":              0,
	})
	d.Set("request_id", "req-42")

	if err := resourceAzSubscriptionQuotaRead(d, newTestConfig(srv)); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if d.Get("limit").(int) != 100 {
		t.Errorf("limit = %d", d.Get("limit"))
	}
	if d.Get("provisioning_state").(string) != "Succeeded" {
		t.Errorf("provisioning_state = %q", d.Get("provisioning_state"))
	}
	if d.Id() != "req-42" {
		t.Errorf("Id = %q", d.Id())
	}
}

func TestResourceAzSubscriptionQuotaUpdate_DelegatesToCreate(t *testing.T) {
	called := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.Write([]byte(`{"data":{"name":"req-99","properties":{"provisioningState":"Succeeded"}}}`))
	}))
	defer srv.Close()

	d := schemaResourceDataFromRaw(t, resourceAzSubscriptionQuota().Schema, map[string]interface{}{
		"subscription_id":    "sub-1",
		"provider_namespace": "Microsoft.Compute",
		"location":           "westeurope",
		"quota_family":       "f",
		"quota_resource":     "r",
		"limit":              5,
	})
	if err := resourceAzSubscriptionQuotaUpdate(d, newTestConfig(srv)); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if called != 1 {
		t.Errorf("expected 1 PUT, got %d", called)
	}
	if d.Id() != "req-99" {
		t.Errorf("Id = %q", d.Id())
	}
}

func TestResourceAzSubscriptionQuotaDelete_ClearsId(t *testing.T) {
	d := schemaResourceDataFromRaw(t, resourceAzSubscriptionQuota().Schema, map[string]interface{}{
		"subscription_id":    "s",
		"provider_namespace": "p",
		"location":           "l",
		"quota_family":       "f",
		"quota_resource":     "r",
		"limit":              1,
	})
	d.SetId("req-1")
	if err := resourceAzSubscriptionQuotaDelete(d, &Config{}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected Id to be cleared, got %q", d.Id())
	}
}

func TestResourceAzSubscriptionQuotaCreate_PollsUntilSucceeded(t *testing.T) {
	previousInterval := quotaPollInterval
	quotaPollInterval = time.Millisecond
	defer func() { quotaPollInterval = previousInterval }()

	getCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			w.Write([]byte(`{"data":{"name":"req-poll","properties":{"provisioningState":"InProgress"}}}`))
		case http.MethodGet:
			getCalls++
			w.Write([]byte(`{"data":{"name":"req-poll","properties":{"provisioningState":"Succeeded","value":[{"limit":{"value":10},"name":{"value":"STANDARDDFAMILY"}}]}}}`))
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	d := schemaResourceDataFromRaw(t, resourceAzSubscriptionQuota().Schema, map[string]interface{}{
		"subscription_id":    "sub-1",
		"provider_namespace": "Microsoft.Compute",
		"location":           "westeurope",
		"quota_resource":     "standardDFamily",
		"limit":              10,
	})
	if err := resourceAzSubscriptionQuotaCreate(d, newTestConfig(srv)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if getCalls == 0 || d.Get("provisioning_state").(string) != "Succeeded" {
		t.Fatalf("polling calls = %d, state = %q", getCalls, d.Get("provisioning_state"))
	}
}

func TestResourceAzSubscriptionQuotaImport_ParsesCompositeID(t *testing.T) {
	d := schemaResourceDataFromRaw(t, resourceAzSubscriptionQuota().Schema, nil)
	d.SetId("sub-1,Microsoft.Compute,westeurope,req-42")
	resources, err := resourceAzSubscriptionQuotaImport(context.Background(), d, nil)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(resources) != 1 || d.Id() != "req-42" {
		t.Fatalf("resources = %d, ID = %q", len(resources), d.Id())
	}
	if d.Get("provider_namespace").(string) != "Microsoft.Compute" || d.Get("location").(string) != "westeurope" {
		t.Errorf("imported fields: provider=%q location=%q", d.Get("provider_namespace"), d.Get("location"))
	}
}
