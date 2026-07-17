package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"terraform-provider-cancommarketplace/internal/azure"
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
	if _, ok := r.Schema["user_uuid"]; ok {
		t.Error("user_uuid should not be part of the subscription resource schema")
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
	var renamedSubscriptionID, renamedDisplayName string

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

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{
		"display_name": "My Sub",
	})

	if err := resourceAzSubscriptionCreate(d, newTestConfigWithAzureHooks(
		srv,
		func(_ context.Context, operations []azure.Operation) error {
			if len(operations) != 1 || operations[0] != azure.OperationRenameSubscription {
				t.Fatalf("preflight operations = %#v, want rename", operations)
			}
			return nil
		},
		func(_ context.Context, subscriptionID, displayName string) error {
			if getCalls < 2 {
				t.Error("subscription rename was sent before order status became ACTIVE")
			}
			renamedSubscriptionID = subscriptionID
			renamedDisplayName = displayName
			return nil
		},
		nil,
		nil,
		nil,
	)); err != nil {
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
	if renamedSubscriptionID != "azure-new" || renamedDisplayName != "My Sub" {
		t.Errorf("rename = subscription %q display name %q", renamedSubscriptionID, renamedDisplayName)
	}
	if fs.calls[routeKey{http.MethodPut, "/v1/subscriptions"}] != 0 {
		t.Error("did not expect Marketplace PUT for display_name")
	}
	if getCalls != 2 {
		t.Errorf("subscription status GET calls = %d, want 2", getCalls)
	}
}

func TestResourceAzSubscriptionCreate_DisplayNameRequiresAzurePreflightBeforePost(t *testing.T) {
	fs, srv := newFakeServer(t)
	defer srv.Close()
	fs.on(http.MethodPost, "/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		t.Error("subscription create POST must not run when Azure preflight fails")
		w.WriteHeader(http.StatusCreated)
	})

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{
		"display_name": "My Sub",
	})
	err := resourceAzSubscriptionCreate(d, newTestConfig(srv))
	if err == nil {
		t.Fatal("expected Azure preflight error, got nil")
	}
	for _, want := range []string{"az login", "azure_client_id", "remove", "display_name"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, expected mention of %q", err.Error(), want)
		}
	}
	if fs.calls[routeKey{http.MethodPost, "/v1/subscriptions"}] != 0 {
		t.Fatal("subscription create POST ran despite missing Azure auth")
	}
	if d.Id() != "" {
		t.Fatalf("resource ID should stay empty on preflight failure, got %q", d.Id())
	}
}

func TestResourceAzSubscriptionCreate_DisplayNamePermissionPreflightBlocksPost(t *testing.T) {
	fs, srv := newFakeServer(t)
	defer srv.Close()
	fs.on(http.MethodPost, "/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		t.Error("subscription create POST must not run when Azure permission preflight fails")
		w.WriteHeader(http.StatusCreated)
	})

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{
		"display_name": "My Sub",
	})
	err := resourceAzSubscriptionCreate(d, newTestConfigWithAzurePreflight(srv, func(_ context.Context, operations []azure.Operation) error {
		if len(operations) != 1 || operations[0] != azure.OperationRenameSubscription {
			t.Fatalf("preflight operations = %#v, want rename", operations)
		}
		return errors.New("missing rename permission")
	}))
	if err == nil || !strings.Contains(err.Error(), "missing rename permission") {
		t.Fatalf("expected permission preflight error, got %v", err)
	}
	if fs.calls[routeKey{http.MethodPost, "/v1/subscriptions"}] != 0 {
		t.Fatal("subscription create POST ran despite failed Azure permission preflight")
	}
	if d.Id() != "" {
		t.Fatalf("resource ID should stay empty on preflight failure, got %q", d.Id())
	}
}

func TestResourceAzSubscriptionCreate_AssignsOwnerRoleAfterActivation(t *testing.T) {
	useFastSubscriptionPolling(t)
	fs, srv := newFakeServer(t)
	defer srv.Close()
	var assignedSubscriptionID, assignedPrincipalID string

	fs.on(http.MethodPost, "/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"data":{"id":"sub-owner","externalAccountId":"azure-owner","user":{"id":"uuid-1"},"order":{"paymentPlan":{"id":172495}}}}`))
	})
	fs.on(http.MethodGet, "/v1/subscriptions/sub-owner", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"id":"sub-owner","externalAccountId":"azure-owner","user":{"id":"uuid-1"},"order":{"status":"ACTIVE","paymentPlan":{"id":172495}}}}`))
	})

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{
		"azure_owner_object_id": "principal-1",
	})

	err := resourceAzSubscriptionCreate(d, newTestConfigWithAzureHooks(
		srv,
		func(_ context.Context, operations []azure.Operation) error {
			if len(operations) != 1 || operations[0] != azure.OperationAssignOwnerRole {
				t.Fatalf("preflight operations = %#v, want owner role assignment", operations)
			}
			return nil
		},
		nil,
		nil,
		func(_ context.Context, subscriptionID, principalID string) error {
			assignedSubscriptionID = subscriptionID
			assignedPrincipalID = principalID
			return nil
		},
		nil,
	))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if assignedSubscriptionID != "azure-owner" || assignedPrincipalID != "principal-1" {
		t.Fatalf("owner assignment = subscription %q principal %q", assignedSubscriptionID, assignedPrincipalID)
	}
	if d.Get("azure_owner_object_id").(string) != "principal-1" {
		t.Fatalf("azure_owner_object_id = %q", d.Get("azure_owner_object_id"))
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

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{})
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

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{})
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

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{})
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

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{})
	err := resourceAzSubscriptionCreate(d, newTestConfig(srv))
	if err == nil || !strings.Contains(err.Error(), "failed to create Azure subscription") {
		t.Fatalf("expected create error, got %v", err)
	}
}

func TestResourceAzSubscriptionRead_PopulatesAttributes(t *testing.T) {
	fs, srv := newFakeServer(t)
	defer srv.Close()
	fs.on(http.MethodGet, "/v1/subscriptions/sub-r", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"id":"sub-r","externalAccountId":"azure-r","label":"Marketplace Label","user":{"id":"owner-1"},"order":{"paymentPlan":{"id":3}}}}`))
	})

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{"display_name": "Tracked"})
	d.SetId("sub-r")

	if err := resourceAzSubscriptionRead(d, newTestConfigWithAzureHooks(
		srv,
		nil,
		nil,
		func(_ context.Context, subscriptionID string) (string, error) {
			if subscriptionID != "azure-r" {
				t.Fatalf("display name read subscription = %q", subscriptionID)
			}
			return "Azure Friendly", nil
		},
		nil,
		nil,
	)); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if d.Get("display_name").(string) != "Azure Friendly" {
		t.Errorf("display_name = %q", d.Get("display_name"))
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

func TestResourceAzSubscriptionUpdate_RenamesThroughAzureWithoutMarketplaceMutation(t *testing.T) {
	fs, srv := newFakeServer(t)
	defer srv.Close()
	var renamedSubscriptionID, renamedDisplayName string

	fs.on(http.MethodGet, "/v1/subscriptions/sub-u", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"id":"sub-u","externalAccountId":"azure-u","label":"Old","user":{"id":"old-owner"},"order":{"paymentPlan":{"id":172495},"paymentPlanId":172495}}}`))
	})

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{
		"display_name": "New",
	})
	d.SetId("sub-u")

	if err := resourceAzSubscriptionUpdate(d, newTestConfigWithAzureHooks(
		srv,
		func(_ context.Context, operations []azure.Operation) error {
			if len(operations) != 1 || operations[0] != azure.OperationRenameSubscription {
				t.Fatalf("preflight operations = %#v, want rename", operations)
			}
			return nil
		},
		func(_ context.Context, subscriptionID, displayName string) error {
			renamedSubscriptionID = subscriptionID
			renamedDisplayName = displayName
			return nil
		},
		nil,
		nil,
		nil,
	)); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if renamedSubscriptionID != "azure-u" || renamedDisplayName != "New" {
		t.Fatalf("rename = subscription %q display name %q", renamedSubscriptionID, renamedDisplayName)
	}
	if fs.calls[routeKey{http.MethodPut, "/v1/subscriptions"}] != 0 {
		t.Errorf("did not expect Marketplace PUT for display_name, got %d", fs.calls[routeKey{http.MethodPut, "/v1/subscriptions"}])
	}
}

func TestResourceAzSubscriptionUpdate_AssignsOwnerRoleWithoutMarketplaceMutation(t *testing.T) {
	fs, srv := newFakeServer(t)
	defer srv.Close()
	var assignedSubscriptionID, assignedPrincipalID string

	fs.on(http.MethodGet, "/v1/subscriptions/sub-u", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"id":"sub-u","externalAccountId":"azure-u","label":"Old","user":{"id":"old-owner"},"order":{"paymentPlan":{"id":172495},"paymentPlanId":172495}}}`))
	})

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{
		"azure_owner_object_id": "new-owner",
	})
	d.SetId("sub-u")

	err := resourceAzSubscriptionUpdate(d, newTestConfigWithAzureHooks(
		srv,
		func(_ context.Context, operations []azure.Operation) error {
			if len(operations) != 1 || operations[0] != azure.OperationAssignOwnerRole {
				t.Fatalf("preflight operations = %#v, want owner role assignment", operations)
			}
			return nil
		},
		nil,
		nil,
		func(_ context.Context, subscriptionID, principalID string) error {
			assignedSubscriptionID = subscriptionID
			assignedPrincipalID = principalID
			return nil
		},
		nil,
	))
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if assignedSubscriptionID != "azure-u" || assignedPrincipalID != "new-owner" {
		t.Fatalf("owner assignment = subscription %q principal %q", assignedSubscriptionID, assignedPrincipalID)
	}
	if fs.calls[routeKey{http.MethodPut, "/v1/subscriptions"}] != 0 {
		t.Errorf("did not expect Marketplace PUT for owner assignment, got %d", fs.calls[routeKey{http.MethodPut, "/v1/subscriptions"}])
	}
}

func TestResourceAzSubscriptionDelete_NoAzureCredsErrors(t *testing.T) {
	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{})
	d.SetId("sub-d")
	if err := d.Set("subscription_id", "azure-sub-d"); err != nil {
		t.Fatalf("set subscription_id: %v", err)
	}
	err := resourceAzSubscriptionDelete(d, &Config{})
	if err == nil || !strings.Contains(err.Error(), "Azure authentication is required") || !strings.Contains(err.Error(), string(azure.OperationCancelSubscription)) {
		t.Fatalf("expected azure auth error, got %v", err)
	}
}

func TestResourceAzSubscriptionDelete_PreflightsAndCancelsWithAzure(t *testing.T) {
	_, srv := newFakeServer(t)
	defer srv.Close()
	var canceledSubscriptionID string

	d := schemaResourceDataFromRaw(t, resourceAzSubscription().Schema, map[string]interface{}{})
	d.SetId("sub-d")
	if err := d.Set("subscription_id", "azure-sub-d"); err != nil {
		t.Fatalf("set subscription_id: %v", err)
	}

	err := resourceAzSubscriptionDelete(d, newTestConfigWithAzureHooks(
		srv,
		func(_ context.Context, operations []azure.Operation) error {
			if len(operations) != 1 || operations[0] != azure.OperationCancelSubscription {
				t.Fatalf("preflight operations = %#v, want cancel", operations)
			}
			return nil
		},
		nil,
		nil,
		nil,
		func(_ context.Context, subscriptionID string) error {
			canceledSubscriptionID = subscriptionID
			return nil
		},
	))
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if canceledSubscriptionID != "azure-sub-d" {
		t.Fatalf("canceled subscription = %q", canceledSubscriptionID)
	}
}
