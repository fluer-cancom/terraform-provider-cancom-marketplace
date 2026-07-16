package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func useFastSubscriptionPolling(t *testing.T) {
	t.Helper()
	previous := subscriptionPollInterval
	subscriptionPollInterval = time.Millisecond
	t.Cleanup(func() { subscriptionPollInterval = previous })
}

func TestResourceAzSubscription_Schema(t *testing.T) {
	r := resourceAzSubscription()
	if err := r.InternalValidate(nil, true); err != nil {
		t.Fatalf("schema invalid: %v", err)
	}
	if !r.Schema["user_uuid"].Required {
		t.Error("user_uuid should be Required")
	}
	if !r.Schema["user_uuid"].ForceNew {
		t.Error("user_uuid should be ForceNew")
	}
	if !r.Schema["subscription_id"].Computed {
		t.Error("subscription_id should be Computed")
	}
	if !r.Schema["marketplace_subscription_id"].Computed {
		t.Error("marketplace_subscription_id should be Computed")
	}
	if !r.Schema["payment_plan_id"].Computed || r.Schema["payment_plan_id"].Optional || r.Schema["payment_plan_id"].Required {
		t.Error("payment_plan_id should be read-only")
	}
}

// fakeServer routes by (method, path-prefix) to allow chaining a POST then PUT.
type routeKey struct{ method, path string }
type fakeServer struct {
	t        *testing.T
	handlers map[routeKey]http.HandlerFunc
	calls    map[routeKey]int
}

func newFakeServer(t *testing.T) (*fakeServer, *httptest.Server) {
	t.Helper()
	fs := &fakeServer{t: t, handlers: map[routeKey]http.HandlerFunc{}, calls: map[routeKey]int{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := routeKey{r.Method, r.URL.Path}
		h, ok := fs.handlers[key]
		if !ok {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "no handler", http.StatusNotFound)
			return
		}
		fs.calls[key]++
		h(w, r)
	}))
	return fs, srv
}

func (fs *fakeServer) on(method, path string, h http.HandlerFunc) {
	fs.handlers[routeKey{method, path}] = h
}

func TestResourceAzSubscriptionCreate_HappyPathWithRename(t *testing.T) {
	useFastSubscriptionPolling(t)
	fs, srv := newFakeServer(t)
	defer srv.Close()
	getCalls := 0

	fs.on(http.MethodPost, "/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("userUUID"); got != "uuid-1" {
			t.Errorf("userUUID = %q", got)
		}
		var body struct {
			Order struct {
				PaymentPlanID int `json:"paymentPlanId"`
			} `json:"order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode create body: %v", err)
		}
		if body.Order.PaymentPlanID != azureSubscriptionPaymentPlanID {
			t.Errorf("paymentPlanId = %d, want %d", body.Order.PaymentPlanID, azureSubscriptionPaymentPlanID)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"data":{"id":"sub-new","externalAccountId":"azure-new","user":{"id":"uuid-1"},"order":{"paymentPlan":{"id":172495}}}}`))
	})
	fs.on(http.MethodGet, "/v1/subscriptions/sub-new", func(w http.ResponseWriter, r *http.Request) {
		getCalls++
		if getCalls == 1 {
			w.Write([]byte(`{"data":{"id":"sub-new","externalAccountId":"azure-new","user":{"id":"uuid-1"},"order":{"status":"PENDING","paymentPlan":{"id":172495},"paymentPlanId":172495}}}`))
			return
		}
		w.Write([]byte(`{"data":{"id":"sub-new","externalAccountId":"azure-new","user":{"id":"uuid-1"},"order":{"status":"ACTIVE","paymentPlan":{"id":172495},"paymentPlanId":172495},"futureApiField":{"keep":true}}}`))
	})
	fs.on(http.MethodPut, "/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		if getCalls < 2 {
			t.Error("subscription update was sent before order status became ACTIVE")
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode update body: %v", err)
		}
		if body["label"] != "My Sub" {
			t.Errorf("label = %v", body["label"])
		}
		if _, ok := body["futureApiField"]; !ok {
			t.Error("update dropped an unknown API field")
		}
		w.WriteHeader(http.StatusOK)
	})

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{
		"user_uuid":    "uuid-1",
		"display_name": "My Sub",
	})

	if err := resourceAzSubscriptionCreate(d, newTestConfig(srv)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if d.Id() != "sub-new" {
		t.Errorf("Id = %q", d.Id())
	}
	if d.Get("subscription_id").(string) != "azure-new" {
		t.Errorf("subscription_id = %q", d.Get("subscription_id"))
	}
	if d.Get("marketplace_subscription_id").(string) != "sub-new" {
		t.Errorf("marketplace_subscription_id = %q", d.Get("marketplace_subscription_id"))
	}
	if d.Get("payment_plan_id").(int) != azureSubscriptionPaymentPlanID {
		t.Errorf("payment_plan_id = %d", d.Get("payment_plan_id"))
	}
	if fs.calls[routeKey{http.MethodPut, "/v1/subscriptions"}] != 1 {
		t.Error("expected one rename PUT")
	}
	if getCalls != 2 {
		t.Errorf("subscription status GET calls = %d, want 2", getCalls)
	}
}

func TestResourceAzSubscriptionCreate_NoRenameWhenDisplayNameEmpty(t *testing.T) {
	useFastSubscriptionPolling(t)
	fs, srv := newFakeServer(t)
	defer srv.Close()
	fs.on(http.MethodPost, "/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"data":{"id":"sub-x","externalAccountId":"azure-x","user":{"id":"uuid-1"},"order":{"paymentPlan":{"id":172495}}}}`))
	})
	fs.on(http.MethodGet, "/v1/subscriptions/sub-x", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"id":"sub-x","externalAccountId":"azure-x","user":{"id":"uuid-1"},"order":{"status":"ACTIVE","paymentPlan":{"id":172495}}}}`))
	})

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{"user_uuid": "uuid-1"})
	if err := resourceAzSubscriptionCreate(d, newTestConfig(srv)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if d.Id() != "sub-x" {
		t.Errorf("Id = %q", d.Id())
	}
	if fs.calls[routeKey{http.MethodPut, "/v1/subscriptions"}] != 0 {
		t.Error("did not expect rename PUT when display_name empty")
	}
}

func TestResourceAzSubscriptionCreate_AcceptsBareObjectResponse(t *testing.T) {
	useFastSubscriptionPolling(t)
	// Some marketplace responses are not wrapped in {"data": ...}; accept either.
	fs, srv := newFakeServer(t)
	defer srv.Close()
	fs.on(http.MethodPost, "/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"sub-bare","externalAccountId":"azure-bare","user":{"id":"uuid-1"},"order":{"paymentPlan":{"id":172495}}}`))
	})
	fs.on(http.MethodGet, "/v1/subscriptions/sub-bare", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"id":"sub-bare","externalAccountId":"azure-bare","user":{"id":"uuid-1"},"order":{"status":"ACTIVE","paymentPlan":{"id":172495}}}}`))
	})

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{"user_uuid": "uuid-1"})
	if err := resourceAzSubscriptionCreate(d, newTestConfig(srv)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if d.Id() != "sub-bare" {
		t.Errorf("Id = %q, want sub-bare", d.Id())
	}
}

func TestResourceAzSubscriptionCreate_FailsLoudlyOnEmptyId(t *testing.T) {
	fs, srv := newFakeServer(t)
	defer srv.Close()
	fs.on(http.MethodPost, "/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"data":{}}`))
	})

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{"user_uuid": "u"})
	err := resourceAzSubscriptionCreate(d, newTestConfig(srv))
	if err == nil || !strings.Contains(err.Error(), "no id") {
		t.Fatalf("expected 'no id' error, got %v", err)
	}
	if d.Id() != "" {
		t.Errorf("Id should remain empty on failure, got %q", d.Id())
	}
}

func TestResourceAzSubscriptionCreate_PostFailureBubblesUp(t *testing.T) {
	fs, srv := newFakeServer(t)
	defer srv.Close()
	fs.on(http.MethodPost, "/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{"user_uuid": "u"})
	err := resourceAzSubscriptionCreate(d, newTestConfig(srv))
	if err == nil || !strings.Contains(err.Error(), "failed to create Azure subscription") {
		t.Fatalf("expected create error, got %v", err)
	}
}

func TestResourceAzSubscriptionRead_PopulatesAttributes(t *testing.T) {
	fs, srv := newFakeServer(t)
	defer srv.Close()
	fs.on(http.MethodGet, "/v1/subscriptions/sub-r", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"id":"sub-r","externalAccountId":"azure-r","label":"Friendly","user":{"id":"owner-1"},"order":{"paymentPlan":{"id":3}}}}`))
	})

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{"user_uuid": "ignored"})
	d.SetId("sub-r")

	if err := resourceAzSubscriptionRead(d, newTestConfig(srv)); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if d.Get("display_name").(string) != "Friendly" {
		t.Errorf("display_name = %q", d.Get("display_name"))
	}
	if d.Get("azure_owner_object_id").(string) != "owner-1" {
		t.Errorf("azure_owner_object_id = %q", d.Get("azure_owner_object_id"))
	}
	if d.Get("payment_plan_id").(int) != 3 {
		t.Errorf("payment_plan_id = %d", d.Get("payment_plan_id"))
	}
	if d.Get("subscription_id").(string) != "azure-r" {
		t.Errorf("subscription_id = %q", d.Get("subscription_id"))
	}
	if d.Get("marketplace_subscription_id").(string) != "sub-r" {
		t.Errorf("marketplace_subscription_id = %q", d.Get("marketplace_subscription_id"))
	}
}

func TestResourceAzSubscriptionUpdate_RoundTripsThroughChangeSubscription(t *testing.T) {
	fs, srv := newFakeServer(t)
	defer srv.Close()

	fs.on(http.MethodGet, "/v1/subscriptions/sub-u", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"id":"sub-u","label":"Old","user":{"id":"old-owner","href":"/users/old-owner"},"order":{"user":{"id":"old-owner","href":"/users/old-owner"},"paymentPlan":{"id":172495,"futurePlanField":"preserve"},"paymentPlanId":172495},"futureApiField":{"keep":true}}}`))
	})
	fs.on(http.MethodPut, "/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode update body: %v", err)
		}
		if body["label"] != "New" {
			t.Errorf("label = %v", body["label"])
		}
		if _, ok := body["futureApiField"]; !ok {
			t.Error("update dropped an unknown top-level field")
		}
		order := body["order"].(map[string]interface{})
		if order["paymentPlanId"] != float64(azureSubscriptionPaymentPlanID) {
			t.Errorf("paymentPlanId = %v", order["paymentPlanId"])
		}
		paymentPlan := order["paymentPlan"].(map[string]interface{})
		if paymentPlan["id"] != float64(azureSubscriptionPaymentPlanID) || paymentPlan["futurePlanField"] != "preserve" {
			t.Errorf("paymentPlan = %#v", paymentPlan)
		}
		orderUser := order["user"].(map[string]interface{})
		if orderUser["id"] != "new-owner" || orderUser["href"] != "/users/old-owner" {
			t.Errorf("order user = %#v", orderUser)
		}
		user := body["user"].(map[string]interface{})
		if user["id"] != "new-owner" || user["href"] != "/users/old-owner" {
			t.Errorf("user = %#v", user)
		}
		w.WriteHeader(http.StatusOK)
	})

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{
		"user_uuid":             "u",
		"display_name":          "New",
		"azure_owner_object_id": "new-owner",
	})
	d.SetId("sub-u")

	if err := resourceAzSubscriptionUpdate(d, newTestConfig(srv)); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if fs.calls[routeKey{http.MethodPut, "/v1/subscriptions"}] != 1 {
		t.Errorf("expected one PUT, got %d", fs.calls[routeKey{http.MethodPut, "/v1/subscriptions"}])
	}
}

func TestResourceAzSubscriptionDelete_NoAzureCredsErrors(t *testing.T) {
	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{"user_uuid": "u"})
	d.SetId("sub-d")
	if err := d.Set("subscription_id", "azure-sub-d"); err != nil {
		t.Fatalf("set subscription_id: %v", err)
	}
	err := resourceAzSubscriptionDelete(d, &Config{})
	if err == nil || !strings.Contains(err.Error(), "cannot authenticate with Azure API") {
		t.Fatalf("expected azure auth error, got %v", err)
	}
}
