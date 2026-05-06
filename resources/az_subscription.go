package main

import (
	"encoding/base64"
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
				Optional:    false,
				Description: "The object ID of the marketplace principal, which recieves owner permissions after subscription creation.",
				ForceNew:    true,
			},
			"payment_plan_id": {
				Type:        schema.TypeInt,
				Required:    false,
				Optional:    true,
				Description: "The payment plan ID of the Azure subscription.",
				ForceNew:    false,
			},
			"display_name": {
				Type:        schema.TypeString,
				Required:    false,
				Optional:    true,
				Description: "The display name of the subscription.",
				ForceNew:    false,
			},
			"subscription_id": {
				Type:        schema.TypeString,
				Required:    false,
				Optional:    false,
				Computed:    true,
				Description: "The subscription ID of the Azure subscription.",
			},
		},
	}
}

func resourceAzSubscriptionCreate(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	uri := fmt.Sprintf("%s/v1/subscriptions", config.Endpoint)
	parameters := map[string]interface{}{
		"marketplace_user_uuid": d.Get("user_uuid").(string),
		"payment_plan_id":      d.Get("payment_plan_id").(int),
		"display_name":         d.Get("display_name").(string),
	}

	httpClient := &http.Client{
		Timeout: 120 * time.Second,
	}
	req, err := http.NewRequest("POST", uri, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(config.Username+":"+config.Password)))) // #TODO: Auth needs to be changed to OAuth2 token based
	req.Header.Set("X-Correlation-ID", 104)
	q := req.URL.Query()
	q.Add("userUUID", parameters["marketplace_user_uuid"].(string))

	req := map[string]interface{}{
		"order": map[string]interface{}{
			"paymentPlanId": parameters["payment_plan_id"].(int),
		},
	}

	requestBody, err := json.Marshal(parameters)
	if err != nil {
		return err
	}

	req.URL.RawQuery = q.Encode()
	req.Body = io.NopCloser(bytes.NewReader(requestBody))

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create Azure subscription: %s", resp.Status)
	}

	// Parse the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var result map[string]interface{}

	// Get subscription info
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Errorf("failed to parse subscription info from response")
		return err
	}
	
	subscriptionInfo := result["data"].(CSPSubscription)

	// Rename subscription, if display_name is set
	if d.Get("display_name").(string) != "" {
		subscriptionInfo.Label = d.Get("display_name").(string)
		err = changeSubscription(subscriptionInfo, config)
		if err != nil {
			return err
		}
	}

	// Set subscription info to resource data
	if subscriptionInfo.OrderNumber != nil {
		d.Set("user_uuid", subscriptionInfo.User.Id)
	}
	if subscriptionInfo.Order.PaymentPlanId != nil {
		d.Set("payment_plan_id", subscriptionInfo.Order.PaymentPlanId)
	}
	if subscriptionInfo.SubscriptionId != "" {
		d.Set("subscription_id", subscriptionInfo.SubscriptionId)
	}
	if subscriptionInfo.Label != nil {
		d.Set("display_name", subscriptionInfo.Label)
	}

	d.SetId(subscriptionInfo.SubscriptionId)

	return nil
}

func resourceAzSubscriptionRead(d *schema.ResourceData, m interface{}) error {
	subscriptionInfo, err := subscriptionInfo(d.Get("subscription_id").(string), m.(*Config))
	if err != nil {
		return err
	}

	if subscriptionInfo.Order.PaymentPlanId != nil {
		d.Set("payment_plan_id", subscriptionInfo.Order.PaymentPlanId)
	}
	if subscriptionInfo.User.Id != nil {
		d.Set("azure_owner_object_id", subscriptionInfo.User.Id)
	}
	if subscriptionInfo.SubscriptionId != "" {
		d.Set("subscription_id", subscriptionInfo.SubscriptionId)
	}
	if subscriptionInfo.Label != nil {
		d.Set("display_name", subscriptionInfo.Label)
	}
	return nil
}

func resourceAzSubscriptionDelete(d *schema.ResourceData, m interface{}) error {
	return cancelSubscription(d.Get("subscription_id").(string), m.(*Config))
}

func resourceAzSubscriptionUpdate(d *schema.ResourceData, m interface{}) error {
	d.Partial(true)
	subscriptionInfo, err := subscriptionInfo(d.Get("subscription_id").(string), m.(*Config))
	if err != nil {
		return err
	}
	subscriptionInfo.Label = d.Get("display_name").(string)
	subscriptionInfo.Order.PaymentPlanId = d.Get("payment_plan_id").(int)
	subscriptionInfo.User.Id = d.Get("azure_owner_object_id").(string)

	err = changeSubscription(subscriptionInfo, m.(*Config))
	if err != nil {
		return err
	}
	d.Partial(false)

	return nil
}
