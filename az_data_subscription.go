package main

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataAzSubscription() *schema.Resource {
	return &schema.Resource{
		Read:        dataAzSubscriptionRead,
		Description: "Reads information about an Azure Subscription within the Cancom Marketplace.",
		Schema: map[string]*schema.Schema{
			"marketplace_subscription_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The CANCOM Marketplace subscription ID used to query the Marketplace API.",
			},
			"subscription_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The Azure subscription ID returned as externalAccountId by the Marketplace API.",
			},
			"display_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The display name of the subscription.",
			},
			"payment_plan_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The payment plan ID associated with the subscription's order.",
			},
			"owner_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The marketplace user ID that owns the subscription.",
			},
		},
	}
}

func dataAzSubscriptionRead(d *schema.ResourceData, m interface{}) error {
	cfg := m.(*Config)
	marketplaceSubscriptionID := d.Get("marketplace_subscription_id").(string)

	sub, err := subscriptionInfo(marketplaceSubscriptionID, cfg)
	if err != nil {
		return fmt.Errorf("failed to read Azure subscription info: %w", err)
	}

	d.SetId(sub.Id)
	if err := d.Set("subscription_id", sub.ExternalAccountId); err != nil {
		return fmt.Errorf("failed to set Azure subscription ID: %w", err)
	}
	if sub.Label != nil {
		d.Set("display_name", *sub.Label)
	}
	d.Set("payment_plan_id", sub.Order.PaymentPlan.Id)
	d.Set("owner_id", sub.User.Id)
	return nil
}
