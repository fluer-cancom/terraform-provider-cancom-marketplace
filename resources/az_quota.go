package main

import (
	"context"
	"fmt"
	
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAzSubscriptionQuota() *schema.Resource {
	return &schema.Resource{
		Read:        resourceAzSubscriptionQuotaRead,
		Description: "Reads the quota information for an Azure Subscription within the Cancom Marketplace.",
		Schema: map[string]*schema.Schema{
			"subscription_id": {
				Type:        schema.TypeString,
				Required:    true,
				Optional:    false,
				Computed:    false,
				Description: "The subscription ID of the Azure subscription.",
			},
			"quota": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The quota of the Azure subscription.",
			},
		},
	}
}