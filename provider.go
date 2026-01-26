package main

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func Provider() *schema.Provider {
	return &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"az_subscription": resourceAzSubscription(),
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
	}
}
