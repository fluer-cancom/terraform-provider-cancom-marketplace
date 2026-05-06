package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
	fs, srv := newFakeServer(t)
	defer srv.Close()

	fs.on(http.MethodPost, "/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("userUUID"); got != "uuid-1" {
			t.Errorf("userUUID = %q", got)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"data":{"id":"sub-new","user":{"id":"uuid-1"},"order":{"paymentPlan":{"id":7}}}}`))
	})
	fs.on(http.MethodPut, "/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{
		"user_uuid":       "uuid-1",
		"payment_plan_id": 7,
		"display_name":    "My Sub",
	})

	if err := resourceAzSubscriptionCreate(d, newTestConfig(srv)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if d.Id() != "sub-new" {
		t.Errorf("Id = %q", d.Id())
	}
	if d.Get("subscription_id").(string) != "sub-new" {
		t.Errorf("subscription_id = %q", d.Get("subscription_id"))
	}
	if d.Get("payment_plan_id").(int) != 7 {
		t.Errorf("payment_plan_id = %d", d.Get("payment_plan_id"))
	}
	if fs.calls[routeKey{http.MethodPut, "/v1/subscriptions"}] != 1 {
		t.Error("expected one rename PUT")
	}
}

func TestResourceAzSubscriptionCreate_NoRenameWhenDisplayNameEmpty(t *testing.T) {
	fs, srv := newFakeServer(t)
	defer srv.Close()
	fs.on(http.MethodPost, "/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"data":{"id":"sub-x","user":{"id":"uuid-1"},"order":{"paymentPlan":{"id":1}}}}`))
	})

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{
		"user_uuid":       "uuid-1",
		"payment_plan_id": 1,
	})
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
		w.Write([]byte(`{"data":{"id":"sub-r","label":"Friendly","user":{"id":"owner-1"},"order":{"paymentPlan":{"id":3}}}}`))
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
	if d.Get("subscription_id").(string) != "sub-r" {
		t.Errorf("subscription_id = %q", d.Get("subscription_id"))
	}
}

func TestResourceAzSubscriptionUpdate_RoundTripsThroughChangeSubscription(t *testing.T) {
	fs, srv := newFakeServer(t)
	defer srv.Close()

	fs.on(http.MethodGet, "/v1/subscriptions/sub-u", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"id":"sub-u","label":"Old","user":{"id":"old-owner"},"order":{"paymentPlan":{"id":1}}}}`))
	})
	fs.on(http.MethodPut, "/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{
		"user_uuid":             "u",
		"display_name":          "New",
		"payment_plan_id":       9,
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
	err := resourceAzSubscriptionDelete(d, &Config{})
	if err == nil || !strings.Contains(err.Error(), "cannot authenticate with Azure API") {
		t.Fatalf("expected azure auth error, got %v", err)
	}
}
