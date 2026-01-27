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
			"cancom-marketplace_az_subscription": resourceAzSubscription(),
		},
		Schema: map[string]*schema.Schema{
			"api_username": {
				Type:        schema.TypeString,
				Required:    true,
				Optional:    false,
				Description: "The API username for the Cancom Marketplace - recieved by OneTime Link",
			},
			"api_password": {
				Type:        schema.TypeString,
				Required:    true,
				Optional:    false,
				Description: "The API password for the Cancom Marketplace - recieved by OneTime Link",
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
				Default:     "https://cc-marketplace-ip.azure-api.net",
			},
			"country": {
				Type:        schema.TypeString,
				Required:    true,
				Optional:    false,
				Description: "The country of the customer",
			},
		},
		ConfigureFunc: providerConfigure,
	}
}

type Config struct {
	Endpoint          string
	Username          string
	Password          string
	Country           string
	AzureClientId     string
	AzureClientSecret string
	AzureTenantId     string
	AzureAuthCtx      azcore.TokenCredential
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	providerConfig := &Config{
		Endpoint:          d.Get("endpoint").(string),
		Username:          d.Get("api_username").(string),
		Password:          d.Get("api_password").(string),
		Country:           d.Get("country").(string),
		AzureClientId:     d.Get("azure_client_id").(string),
		AzureClientSecret: d.Get("azure_client_secret").(string),
		AzureTenantId:     d.Get("azure_tenant_id").(string),
		AzureAuthCtx:      nil,
	}

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
	return providerConfig, nil
}
