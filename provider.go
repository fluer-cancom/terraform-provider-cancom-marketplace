package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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
				Required:    true,
				Description: "The API client ID for the Cancom Marketplace - received by OneTime Link",
			},
			"api_client_secret": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				Description: "The API client secret for the Cancom Marketplace - received by OneTime Link",
			},
			"api_scope": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The API scope for the Cancom Marketplace - default is 'AT-PROD'",
				Default:     "AT-PROD",
			},
			"azure_client_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The Azure client ID for the customers tenant",
			},
			"azure_client_secret": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "The Azure client secret for the customers tenant",
			},
			"azure_tenant_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The Azure tenant ID for the customers tenant",
			},
			"endpoint": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The API endpoint for the Cancom Marketplace",
				Default:     "https://marketplace-apigateway.cancom.de",
			},
		},
		ConfigureFunc: providerConfigure,
	}
}

type Config struct {
	Endpoint          string
	ClientId          string
	ClientSecret      string
	Scope             string
	CCMPApiToken      string
	AzureClientId     string
	AzureClientSecret string
	AzureTenantId     string
	AzureAuthCtx      azcore.TokenCredential
	HTTPClient        *http.Client
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
		Endpoint:          d.Get("endpoint").(string),
		ClientId:          d.Get("api_client_id").(string),
		ClientSecret:      d.Get("api_client_secret").(string),
		Scope:             d.Get("api_scope").(string),
		AzureClientId:     d.Get("azure_client_id").(string),
		AzureClientSecret: d.Get("azure_client_secret").(string),
		AzureTenantId:     d.Get("azure_tenant_id").(string),
		HTTPClient:        &http.Client{Timeout: 120 * time.Second},
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
		cred, err := azidentity.NewClientSecretCredential(azTenant, azID, azSecret, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to build Azure client-secret credential: %w", err)
		}
		cfg.AzureAuthCtx = cred
	} else {
		// Best-effort: fall back to default azure CLI / env credentials.
		if cred, err := azidentity.NewDefaultAzureCredential(nil); err == nil {
			cfg.AzureAuthCtx = cred
		}
	}

	tok, err := fetchToken(cfg)
	if err != nil {
		return nil, err
	}
	cfg.CCMPApiToken = tok
	return cfg, nil
}
