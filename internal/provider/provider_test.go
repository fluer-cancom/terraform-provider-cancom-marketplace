package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderInternalValidate(t *testing.T) {
	if err := Provider().InternalValidate(); err != nil {
		t.Fatalf("Provider schema is invalid: %v", err)
	}
}

func TestProviderSchemaFields(t *testing.T) {
	p := Provider()

	required := []string{"api_client_id", "api_client_secret", "marketplace_user_email"}
	for _, name := range required {
		s, ok := p.Schema[name]
		if !ok {
			t.Errorf("missing schema field %q", name)
			continue
		}
		if !s.Required {
			t.Errorf("field %q should be Required", name)
		}
	}

	defaults := map[string]interface{}{
		"api_scope": "AT-PROD",
		"endpoint":  "https://marketplace-apigateway.cancom.de",
	}
	for name, want := range defaults {
		s, ok := p.Schema[name]
		if !ok {
			t.Errorf("missing schema field %q", name)
			continue
		}
		if s.Default != want {
			t.Errorf("field %q default = %v, want %v", name, s.Default, want)
		}
	}

	if _, ok := p.ResourcesMap["cancom-marketplace_az_subscription"]; !ok {
		t.Error("resource cancom-marketplace_az_subscription not registered")
	}
	if _, ok := p.ResourcesMap["cancom-marketplace_az_subscription_quota"]; !ok {
		t.Error("resource cancom-marketplace_az_subscription_quota not registered")
	}
	if _, ok := p.DataSourcesMap["cancom-marketplace_az_subscription"]; !ok {
		t.Error("data source cancom-marketplace_az_subscription not registered")
	}
}

func TestProviderConfigure_PartialAzureCredsRejected(t *testing.T) {
	p := Provider()
	raw := map[string]interface{}{
		"api_client_id":          "id",
		"api_client_secret":      "secret",
		"marketplace_user_email": "user@example.com",
		"azure_client_id":        "az-id",
		// missing tenant + secret
	}
	d := schemaResourceDataFromRaw(t, p.Schema, raw)

	prev := fetchToken
	fetchToken = func(*Config) (string, error) { return "ignored", nil }
	defer func() { fetchToken = prev }()

	_, err := providerConfigure(d)
	if err == nil {
		t.Fatal("expected error for partial azure credentials, got nil")
	}
	if !strings.Contains(err.Error(), "all three must be set") {
		t.Errorf("error = %v, expected mention of 'all three must be set'", err)
	}
}

func TestProviderConfigure_FetchesTokenAndPopulatesConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/users" {
			t.Fatalf("unexpected path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
			t.Fatalf("Authorization = %q", got)
		}
		w.Write([]byte(`{"data":[{"id":"user-1","email":"user@example.com"}]}`))
	}))
	defer srv.Close()

	p := Provider()
	raw := map[string]interface{}{
		"api_client_id":          "id",
		"api_client_secret":      "secret",
		"marketplace_user_email": "user@example.com",
		"endpoint":               srv.URL,
	}
	d := schemaResourceDataFromRaw(t, p.Schema, raw)

	prev := fetchToken
	fetchToken = func(cfg *Config) (string, error) {
		if cfg.ClientId != "id" || cfg.ClientSecret != "secret" {
			t.Errorf("fetchToken got wrong creds: %+v", cfg)
		}
		if cfg.Endpoint != srv.URL {
			t.Errorf("fetchToken got wrong endpoint: %s", cfg.Endpoint)
		}
		return "token-123", nil
	}
	defer func() { fetchToken = prev }()

	out, err := providerConfigure(d)
	if err != nil {
		t.Fatalf("providerConfigure returned error: %v", err)
	}
	cfg, ok := out.(*Config)
	if !ok {
		t.Fatalf("expected *Config, got %T", out)
	}
	if cfg.CCMPApiToken != "token-123" {
		t.Errorf("CCMPApiToken = %q, want %q", cfg.CCMPApiToken, "token-123")
	}
	if cfg.Scope != "AT-PROD" {
		t.Errorf("default Scope = %q, want %q", cfg.Scope, "AT-PROD")
	}
	if cfg.HTTPClient == nil {
		t.Error("HTTPClient should be non-nil")
	}
	if cfg.MarketplaceUserID != "user-1" {
		t.Errorf("MarketplaceUserID = %q, want user-1", cfg.MarketplaceUserID)
	}
}

func TestProviderConfigure_UserEmailNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"user-1","email":"someone@example.com"}]}`))
	}))
	defer srv.Close()

	p := Provider()
	d := schemaResourceDataFromRaw(t, p.Schema, map[string]interface{}{
		"api_client_id":          "id",
		"api_client_secret":      "secret",
		"marketplace_user_email": "user@example.com",
		"endpoint":               srv.URL,
	})
	prev := fetchToken
	fetchToken = func(*Config) (string, error) { return "token-123", nil }
	defer func() { fetchToken = prev }()

	_, err := providerConfigure(d)
	if err == nil || !strings.Contains(err.Error(), "User not found in CANCOM Marketplace. Contact your Enterprise Administrator") {
		t.Fatalf("expected user not found error, got %v", err)
	}
}

func TestProviderConfigure_UserEmailAmbiguous(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"user-1","email":"user@example.com"},{"id":"user-2","email":"USER@example.com"}]}`))
	}))
	defer srv.Close()

	p := Provider()
	d := schemaResourceDataFromRaw(t, p.Schema, map[string]interface{}{
		"api_client_id":          "id",
		"api_client_secret":      "secret",
		"marketplace_user_email": "user@example.com",
		"endpoint":               srv.URL,
	})
	prev := fetchToken
	fetchToken = func(*Config) (string, error) { return "token-123", nil }
	defer func() { fetchToken = prev }()

	_, err := providerConfigure(d)
	if err == nil || !strings.Contains(err.Error(), "User is ambigous in CANCOM Marketplace. Contact your Enterprise Administrator") {
		t.Fatalf("expected ambiguous user error, got %v", err)
	}
}

func TestDefaultFetchToken_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("token request method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/oauth2/token" {
			t.Errorf("token request path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q", got)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if r.PostForm.Get("client_id") != "id" ||
			r.PostForm.Get("client_secret") != "sec" ||
			r.PostForm.Get("scope") != "AT-PROD" ||
			r.PostForm.Get("grant_type") != "client_credentials" {
			t.Errorf("token request form = %v", r.PostForm)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"abc","token_type":"Bearer","expires_in":3600}`))
	}))
	defer srv.Close()

	cfg := &Config{
		Endpoint:     srv.URL,
		ClientId:     "id",
		ClientSecret: "sec",
		Scope:        "AT-PROD",
		HTTPClient:   srv.Client(),
	}
	tok, err := defaultFetchToken(cfg)
	if err != nil {
		t.Fatalf("defaultFetchToken: %v", err)
	}
	if tok != "abc" {
		t.Errorf("token = %q, want %q", tok, "abc")
	}
}

func TestDefaultFetchToken_Non200Returns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer srv.Close()

	cfg := &Config{Endpoint: srv.URL, HTTPClient: srv.Client()}
	if _, err := defaultFetchToken(cfg); err == nil {
		t.Fatal("expected error on non-200 token response")
	}
}

func TestDefaultFetchToken_MissingAccessToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cfg := &Config{Endpoint: srv.URL, HTTPClient: srv.Client()}
	_, err := defaultFetchToken(cfg)
	if err == nil || !strings.Contains(err.Error(), "missing access_token") {
		t.Fatalf("expected missing access_token error, got %v", err)
	}
}
