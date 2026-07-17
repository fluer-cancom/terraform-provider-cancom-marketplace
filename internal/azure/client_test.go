package azure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

type fakeCredential struct{}

func (fakeCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{
		Token:     "fake-token",
		ExpiresOn: time.Now().Add(time.Hour),
	}, nil
}

func TestEnsureReadyReadsDefaultManagementGroup(t *testing.T) {
	var sawPath, sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.String()
		sawAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"properties":{"defaultManagementGroup":"/providers/Microsoft.Management/managementGroups/default-mg"}}`))
	}))
	defer srv.Close()

	client := &Client{
		Credential:         fakeCredential{},
		HTTPClient:         srv.Client(),
		ManagementEndpoint: srv.URL,
	}
	if err := client.EnsureReady(context.Background(), []Operation{OperationAssignOwnerRole}); err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
	if sawPath != "/providers/Microsoft.Management/managementGroups/root/settings/default?api-version=2020-05-01" {
		t.Fatalf("path = %q", sawPath)
	}
	if sawAuth != "Bearer fake-token" {
		t.Fatalf("Authorization = %q", sawAuth)
	}
}

func TestAssignOwnerRolePutsSubscriptionScopedAssignment(t *testing.T) {
	var sawPath, sawAuth string
	var body struct {
		Properties struct {
			RoleDefinitionID string `json:"roleDefinitionId"`
			PrincipalID      string `json:"principalId"`
		} `json:"properties"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		sawAuth = r.Header.Get("Authorization")
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := &Client{
		Credential:         fakeCredential{},
		HTTPClient:         srv.Client(),
		ManagementEndpoint: srv.URL,
	}
	if err := client.AssignOwnerRole(context.Background(), "sub-1", "principal-1"); err != nil {
		t.Fatalf("AssignOwnerRole: %v", err)
	}
	if !strings.HasPrefix(sawPath, "/subscriptions/sub-1/providers/Microsoft.Authorization/roleAssignments/") {
		t.Fatalf("path = %q", sawPath)
	}
	if sawAuth != "Bearer fake-token" {
		t.Fatalf("Authorization = %q", sawAuth)
	}
	if body.Properties.PrincipalID != "principal-1" {
		t.Fatalf("principalId = %q", body.Properties.PrincipalID)
	}
	if body.Properties.RoleDefinitionID != "/subscriptions/sub-1/providers/Microsoft.Authorization/roleDefinitions/"+ownerRoleDefinitionID {
		t.Fatalf("roleDefinitionId = %q", body.Properties.RoleDefinitionID)
	}
}
