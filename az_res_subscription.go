package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAzSubscription() *schema.Resource {
	return &schema.Resource{
		Create:      resourceAzSubscriptionCreate,
		Read:        resourceAzSubscriptionRead,
		Update:      resourceAzSubscriptionUpdate,
		Delete:      resourceAzSubscriptionDelete,
		Description: "Manages an Azure Subscription within the Cancom Marketplace.",
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"user_uuid": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The object ID of the marketplace principal that receives owner permissions after subscription creation.",
			},
			"payment_plan_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "The payment plan ID of the Azure subscription.",
			},
			"display_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The display name of the subscription.",
			},
			"azure_owner_object_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The Azure AD object ID of the subscription owner.",
			},
			"subscription_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The subscription ID of the Azure subscription.",
			},
		},
	}
}

func resourceAzSubscriptionCreate(d *schema.ResourceData, m interface{}) error {
	cfg := m.(*Config)

	uri := fmt.Sprintf("%s/v1/subscriptions", cfg.Endpoint)

	body := map[string]interface{}{
		"order": map[string]interface{}{
			"paymentPlanId": d.Get("payment_plan_id").(int),
		},
	}
	requestBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", uri, bytes.NewReader(requestBody))
	if err != nil {
		return err
	}
	q := req.URL.Query()
	q.Add("userUUID", d.Get("user_uuid").(string))
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.CCMPApiToken)
	req.Header.Set("X-Correlation-ID", "10026")

	httpClient := newMarketplaceClient(120*time.Second, cfg)
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create Azure subscription: %s \n Error: %s", resp.Status, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// The OpenAPI spec doesn't fully document the Create response. Some marketplace
	// endpoints wrap in {"data": ...}, others return the bare object — try both.
	var envelope struct {
		Data CSPSubscription `json:"data"`
	}
	var sub CSPSubscription
	if jsonErr := json.Unmarshal(respBody, &envelope); jsonErr == nil && envelope.Data.Id != "" {
		sub = envelope.Data
	} else if jsonErr := json.Unmarshal(respBody, &sub); jsonErr != nil {
		return fmt.Errorf("failed to parse subscription create response: %w; body=%s", jsonErr, string(respBody))
	}
	if sub.Id == "" {
		return fmt.Errorf("subscription create returned no id; body=%s", string(respBody))
	}

	if displayName := d.Get("display_name").(string); displayName != "" {
		dn := displayName
		sub.Label = &dn
		if err := changeSubscription(sub, cfg); err != nil {
			return err
		}
	}

	d.SetId(sub.Id)
	d.Set("subscription_id", sub.Id)
	d.Set("user_uuid", sub.User.Id)
	d.Set("payment_plan_id", sub.Order.PaymentPlan.Id)
	if sub.Label != nil {
		d.Set("display_name", *sub.Label)
	}
	return nil
}

func resourceAzSubscriptionRead(d *schema.ResourceData, m interface{}) error {
	cfg := m.(*Config)

	id := d.Id()
	if id == "" {
		id = d.Get("subscription_id").(string)
	}

	sub, err := subscriptionInfo(id, cfg)
	if err != nil {
		return err
	}

	d.Set("payment_plan_id", sub.Order.PaymentPlan.Id)
	d.Set("azure_owner_object_id", sub.User.Id)
	d.Set("subscription_id", sub.Id)
	if sub.Label != nil {
		d.Set("display_name", *sub.Label)
	}
	return nil
}

func resourceAzSubscriptionUpdate(d *schema.ResourceData, m interface{}) error {
	cfg := m.(*Config)

	d.Partial(true)
	defer d.Partial(false)

	sub, err := subscriptionInfo(d.Id(), cfg)
	if err != nil {
		return err
	}

	if dn := d.Get("display_name").(string); dn != "" {
		sub.Label = &dn
	}
	sub.Order.PaymentPlan.Id = d.Get("payment_plan_id").(int)
	if owner := d.Get("azure_owner_object_id").(string); owner != "" {
		sub.User.Id = owner
	}

	return changeSubscription(sub, cfg)
}

func resourceAzSubscriptionDelete(d *schema.ResourceData, m interface{}) error {
	cfg := m.(*Config)
	return cancelSubscription(d.Id(), cfg)
}
