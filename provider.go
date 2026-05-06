package main

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func Provider() *schema.Provider {
	return &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"az_subscription": resourceAzSubscription(),
			"az_subscription_quota": resourceAzSubscriptionQuota(),
		},
		Schema: map[string]*schema.Schema{
			"api_client_id": {
				Type:        schema.TypeString,
				Required:    true,
				Optional:    false,
				Description: "The API client ID for the Cancom Marketplace - received by OneTime Link",
			},
			"api_client_secret": {
				Type:        schema.TypeString,
				Required:    true,
				Optional:    false,
				Description: "The API client secret for the Cancom Marketplace - received by OneTime Link",
			},
			"api_scope": {
				Type:        schema.TypeString,
				Required:    false,
				Optional:    true,
				Description: "The API scope for the Cancom Marketplace - default is 'AT-PROD'",
				Default:     "AT-PROD",
			},
			"azure_client_id": {
				Type:        schema.TypeString,
				Required:    false,
				Optional:    true,
				Description: "The Azure client ID for the customers tenant",
			},
			"azure_client_secret": {
				Type:        schema.TypeString,
				Required:    false,
				Optional:    true,
				Description: "The Azure client secret for the customers tenant",
			},
			"azure_tenant_id": {
				Type:        schema.TypeString,
				Required:    false,
				Optional:    true,
				Description: "The Azure tenant ID for the customers tenant",
			},
			"endpoint": {
				Type:        schema.TypeString,
				Required:    false,
				Optional:    true,
				Description: "The API endpoint for the Cancom Marketplace",
				Default:     "https://ccmpin-marketplace-apigateway-dev-eagtetaqaxc8ewd6.northeurope-01.azurewebsites.net",
			}
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
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	providerConfig := &Config{
		Endpoint:          d.Get("endpoint").(string),
		ClientId:          d.Get("api_client_id").(string),
		ClientSecret:      d.Get("api_client_secret").(string),
		Scope: 			   d.Get("api_scope").(string),
		CCMPApiToken:      nil,
		AzureClientId:     d.Get("azure_client_id").(string),
		AzureClientSecret: d.Get("azure_client_secret").(string),
		AzureTenantId:     d.Get("azure_tenant_id").(string),
		AzureAuthCtx:      nil,
	}
	// Get Azure API token if Azure credentials are provided, otherwise rely on Azure CLI context. This is needed for subscription renaming, which requires Azure API calls.
	if d.Get("azure_client_id").(string) != "" || d.Get("azure_client_secret").(string) != "" || d.Get("azure_tenant_id").(string) != "" {
		var err error
		if d.Get("azure_client_id").(string) != "" && d.Get("azure_client_secret").(string) != "" && d.Get("azure_tenant_id").(string) != "" {
			providerConfig.AzureAuthCtx, err = azidentity.NewClientSecretCredential(d.Get("azure_tenant_id").(string), d.Get("azure_client_id").(string), d.Get("azure_client_secret").(string), nil)
			if err != nil {
				return nil, fmt.Errorf("Failed to connect with Azure API, using Azure CLI context: %s", err)
			}
			return providerConfig, nil
		}
		return nil, fmt.Errorf("If Azure Client ID, Client Secret and Tenant ID are provided, all three must be provided")
	}
	providerConfig.AzureAuthCtx, _ = azidentity.NewDefaultAzureCredential(nil) // Disregard error, not needed for Cancom Marketplace Subscription Deployment

	// Get API token for Cancom Marketplace API
	uri := fmt.Sprintf("%s/v1/auth/token", providerConfig.Endpoint)
	parameters := map[string]interface{}{
		"clientId":     providerConfig.ClientId,
		"clientSecret": providerConfig.ClientSecret,
		"scope":        providerConfig.Scope,
		"grant_type":   "client_credentials",
	}

	httpClient := &http.Client{
		Timeout: 120 * time.Second,
	}
	req, err := http.NewRequest("POST", uri, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create request for Cancom Marketplace API token: %s", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	req.URL.Query().Add("clientId", parameters["clientId"].(string))
	req.URL.Query().Add("clientSecret", parameters["clientSecret"].(string))
	req.URL.Query().Add("scope", parameters["scope"].(string))
	req.URL.Query().Add("grant_type", parameters["grant_type"].(string))

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to get Cancom Marketplace API token: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Failed to get Cancom Marketplace API token: %s", resp.Status)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read Cancom Marketplace API token response: %s", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, fmt.Errorf("Failed to parse Cancom Marketplace API token response: %s", err)
	}
	providerConfig.CCMPApiToken = result["access_token"].(string)
	
	return providerConfig, nil
}
