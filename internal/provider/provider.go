package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"terraform-provider-cancom-marketplace/internal/azure"
	"terraform-provider-cancom-marketplace/internal/marketplace"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	envAPIClientID          = "CANCOM_MARKETPLACE_API_CLIENT_ID"
	envAPIClientSecret      = "CANCOM_MARKETPLACE_API_CLIENT_SECRET"
	envMarketplaceUserEmail = "CANCOM_MARKETPLACE_USER_EMAIL"
	envAPIScope             = "CANCOM_MARKETPLACE_API_SCOPE"
	envEndpoint             = "CANCOM_MARKETPLACE_ENDPOINT"
	envAzureClientID        = "CANCOM_MARKETPLACE_AZURE_CLIENT_ID"
	envAzureClientSecret    = "CANCOM_MARKETPLACE_AZURE_CLIENT_SECRET"
	envAzureTenantID        = "CANCOM_MARKETPLACE_AZURE_TENANT_ID"
)

func Provider() *schema.Provider {
	return &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"cancom-marketplace_az_subscription":       resourceAzSubscription(),
			"cancom-marketplace_az_subscription_quota": resourceAzSubscriptionQuota(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"cancom-marketplace_az_subscription": dataAzSubscription(),
		},
		Schema: map[string]*schema.Schema{
			"api_client_id": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc(envAPIClientID, nil),
				Description: "The API client ID for the Cancom Marketplace - received by OneTime Link",
			},
			"api_client_secret": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc(envAPIClientSecret, nil),
				Sensitive:   true,
				Description: "The API client secret for the Cancom Marketplace - received by OneTime Link",
			},
			"api_scope": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The API scope for the Cancom Marketplace - default is 'AT-PROD'",
				DefaultFunc: schema.EnvDefaultFunc(envAPIScope, "AT-PROD"),
			},
			"marketplace_user_email": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc(envMarketplaceUserEmail, nil),
				Description: "The marketplace user email address for which subscriptions are created.",
			},
			"azure_client_id": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc(envAzureClientID, nil),
				Description: "The Azure client ID for the customers tenant",
			},
			"azure_client_secret": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc(envAzureClientSecret, nil),
				Sensitive:   true,
				Description: "The Azure client secret for the customers tenant",
			},
			"azure_tenant_id": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc(envAzureTenantID, nil),
				Description: "The Azure tenant ID for the customers tenant",
			},
			"endpoint": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The API endpoint for the Cancom Marketplace",
				DefaultFunc: schema.EnvDefaultFunc(envEndpoint, "https://marketplace-apigateway.cancom.de"),
			},
		},
		ConfigureFunc: providerConfigure,
	}
}

type Config struct {
	Endpoint             string
	ClientId             string
	ClientSecret         string
	Scope                string
	CCMPApiToken         string
	AzureClientId        string
	AzureClientSecret    string
	AzureTenantId        string
	MarketplaceUserEmail string
	MarketplaceUserID    string
	HTTPClient           *http.Client
	Marketplace          *marketplace.Client
	Azure                *azure.Client
}

// fetchToken is exposed as a package var so tests can stub the OAuth call.
var fetchToken = defaultFetchToken

func defaultFetchToken(cfg *Config) (string, error) {
	uri := fmt.Sprintf("%s/v1/oauth2/token", cfg.Endpoint)

	form := url.Values{}
	form.Set("client_id", cfg.ClientId)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("scope", cfg.Scope)
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", uri, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create request for Cancom Marketplace API token: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := cfg.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get Cancom Marketplace API token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get Cancom Marketplace API token: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Cancom Marketplace API token response: %w", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse Cancom Marketplace API token response: %w", err)
	}
	tok, ok := result["access_token"].(string)
	if !ok || tok == "" {
		return "", fmt.Errorf("Cancom Marketplace API token response missing access_token")
	}
	return tok, nil
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	cfg := &Config{
		Endpoint:             d.Get("endpoint").(string),
		ClientId:             d.Get("api_client_id").(string),
		ClientSecret:         d.Get("api_client_secret").(string),
		Scope:                d.Get("api_scope").(string),
		AzureClientId:        d.Get("azure_client_id").(string),
		AzureClientSecret:    d.Get("azure_client_secret").(string),
		AzureTenantId:        d.Get("azure_tenant_id").(string),
		MarketplaceUserEmail: d.Get("marketplace_user_email").(string),
		HTTPClient:           &http.Client{Timeout: 120 * time.Second},
	}

	if cfg.ClientId == "" {
		return nil, fmt.Errorf("api_client_id must be set in provider configuration or %s", envAPIClientID)
	}
	if cfg.ClientSecret == "" {
		return nil, fmt.Errorf("api_client_secret must be set in provider configuration or %s", envAPIClientSecret)
	}
	if cfg.MarketplaceUserEmail == "" {
		return nil, fmt.Errorf("marketplace_user_email must be set in provider configuration or %s", envMarketplaceUserEmail)
	}

	azID := cfg.AzureClientId
	azSecret := cfg.AzureClientSecret
	azTenant := cfg.AzureTenantId
	anyAzure := azID != "" || azSecret != "" || azTenant != ""
	allAzure := azID != "" && azSecret != "" && azTenant != ""

	if anyAzure {
		if !allAzure {
			return nil, fmt.Errorf("if any of azure_client_id, azure_client_secret, azure_tenant_id is set, all three must be set")
		}
		cred, err := azure.NewClientSecretCredential(azTenant, azID, azSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to build Azure client-secret credential: %w", err)
		}
		cfg.Azure = &azure.Client{Credential: cred, HTTPClient: cfg.HTTPClient}
	} else {
		// Best-effort: fall back to default azure CLI / env credentials.
		if cred, err := azure.NewDefaultCredential(); err == nil {
			cfg.Azure = &azure.Client{Credential: cred, HTTPClient: cfg.HTTPClient}
		}
	}

	tok, err := fetchToken(cfg)
	if err != nil {
		return nil, err
	}
	cfg.CCMPApiToken = tok
	cfg.Marketplace = &marketplace.Client{
		Endpoint:   cfg.Endpoint,
		Token:      tok,
		HTTPClient: cfg.HTTPClient,
	}
	userID, err := cfg.Marketplace.UserIDByEmail(cfg.MarketplaceUserEmail)
	if err != nil {
		return nil, err
	}
	cfg.MarketplaceUserID = userID
	return cfg, nil
}
