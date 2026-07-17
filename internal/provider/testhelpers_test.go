package provider

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"terraform-provider-cancom-marketplace/internal/azure"
	"terraform-provider-cancom-marketplace/internal/marketplace"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// schemaResourceDataFromRaw builds a ResourceData populated from a raw attribute
// map. Used to drive ConfigureFunc / CRUD funcs in tests without standing up a
// full Terraform run.
func schemaResourceDataFromRaw(t *testing.T, s map[string]*schema.Schema, raw map[string]interface{}) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, s, raw)
}

// newTestConfig wires a *Config to the given httptest server.
func newTestConfig(srv *httptest.Server) *Config {
	return &Config{
		Endpoint:             srv.URL,
		CCMPApiToken:         "test-token",
		HTTPClient:           srv.Client(),
		MarketplaceUserEmail: "user@example.com",
		MarketplaceUserID:    "uuid-1",
		Marketplace: &marketplace.Client{
			Endpoint:   srv.URL,
			Token:      "test-token",
			HTTPClient: srv.Client(),
		},
	}
}

func newTestConfigWithAzurePreflight(srv *httptest.Server, preflight func(context.Context, []azure.Operation) error) *Config {
	cfg := newTestConfig(srv)
	cfg.Azure = &azure.Client{
		Credential: fakeCredential{},
		Preflight:  preflight,
		Rename:     func(context.Context, string, string) error { return nil },
		DisplayName: func(context.Context, string) (string, error) {
			return "", nil
		},
	}
	return cfg
}

func newTestConfigWithAzureHooks(
	srv *httptest.Server,
	preflight func(context.Context, []azure.Operation) error,
	rename func(context.Context, string, string) error,
	displayName func(context.Context, string) (string, error),
	assignOwner func(context.Context, string, string) error,
	cancel func(context.Context, string) error,
) *Config {
	cfg := newTestConfig(srv)
	cfg.Azure = &azure.Client{
		Credential:  fakeCredential{},
		Preflight:   preflight,
		Rename:      rename,
		DisplayName: displayName,
		AssignOwner: assignOwner,
		Cancel:      cancel,
	}
	return cfg
}

type fakeCredential struct{}

func (fakeCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{
		Token:     "fake-token",
		ExpiresOn: time.Now().Add(time.Hour),
	}, nil
}
