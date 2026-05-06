package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAzSubscriptionQuota() *schema.Resource {
	return &schema.Resource{
		Read:        resourceAzSubscriptionQuotaRead,
		Create:	     resourceAzSubscriptionQuotaCreate,
		Update:      resourceAzSubscriptionQuotaUpdate,
		Delete:      resourceAzSubscriptionQuotaDelete,
		Description: "Manages the quota of an Azure Subscription within the Cancom Marketplace.",
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"subscription_id": {
				Type:        schema.TypeString,
				Required:    true,
				Optional:    false,
				Computed:    false,
				Description: "The subscription ID of the Azure subscription.",
			},
			"provider": {
				Type:        schema.TypeString,
				Required:    true,
				Optional:    false,
				Computed:    false,
				Description: "The resource provider of the quota to be managed.",
			},
			"location": {
				Type:        schema.TypeString,
				Required:    true,
				Optional:    false,
				Computed:    false,
				Description: "The location of the quota to be managed.",
			},
			"quota_family" : {
				Type:        schema.TypeString,
				Required:    true,
				Optional:    false,
				Computed:    false,
				Description: "The family of the quota to be managed.",
			},
			"quota_resource": {
				Type:        schema.TypeString,
				Required:    true,
				Optional:    false,
				Computed:    false,
				Description: "The name of the quota resource to be managed.",
			},
			"limit": {
				Type:        schema.TypeInt,
				Required:    true,
				Optional:    false,
				Computed:    false,
				Description: "The limit of the quota to be set. If not provided, the current quota will be read.",
			},
			"current_value": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The current value of the quota.",
			},
			"request_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The ID of the quota resource in Azure.",
			},
		},
		Description: "Reads the quota information for an Azure Subscription within the Cancom Marketplace.",
	}
}

func resourceAzSubscriptionQuotaCreate(d *schema.ResourceData, m interface{}) error {
	url := fmt.Sprintf(
		"%s/v1/microsoft/quota/subscriptions/%s/providers/%s/locations/%s/providers/Microsoft.Quota/quotas/%s",
		m.(*Config).BaseURL,
		d.Get("subscription_id").(string), 
		d.Get("provider").(string), 
		d.Get("location").(string), 
		d.Get("quota_family").(string)
	)

	httpClient := &http.Client{}
	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request to set subscription quota: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", m.(*Config).CCMPApiToken))
	req.Header.Set("X-Correlation-ID", fmt.Sprintf("%d", 5734008))

	body := map[string]interface{}{
		"properties": map[string]interface{}{
			"limit": map[string]interface{}{
				"value": d.Get("limit").(int),
				"limitObjectType": "LimitValue",
			},
			"name": map[string]interface{}{
				"value": d.Get("quota_resource").(string),
			},
			"properties": map[string]interface{}{
				"requestOrigin": "Microsoft_Azure_Capacity/QuotaApproval.ReactView",
			},
		},
	}

	requestBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body to set subscription quota: %v", err)
	}
	req.Body = io.NopCloser(bytes.NewReader(requestBody))

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to set subscription quota: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK	{
		return fmt.Errorf("failed to set subscription quota: %s", resp.Status)
	}

	// Get request name from response body and set it as request_id in Terraform state for future reference
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body after setting subscription quota: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return fmt.Errorf("failed to parse response body after setting subscription quota: %v", err)
	}
	if result["name"] != nil {
		d.Set("request_id", result["name"].(string))
		d.SetId(result["name"].(string))
	}
	return nil
}

func resourceAzSubscriptionQuotaRead(d *schema.ResourceData, m interface{}) error {
	subscriptionId := d.Get("subscription_id").(string)
	provider := d.Get("provider").(string)
	location := d.Get("location").(string)
	quotaResource := d.Get("request_id").(string)

	url := fmt.Sprintf(
		"%s/v1/microsoft/quota/subscriptions/%s/providers/%s/locations/%s/providers/Microsoft.Quota/quotaRequests/%s",
		m.(*Config).BaseURL,
		subscriptionId, 
		provider, 
		location, 
		quotaResourceId,
	)

	httpClient := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request to get subscription quota: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", m.(*Config).CCMPApiToken))

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to get subscription quota: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get subscription quota: %s", resp.Status)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body after getting subscription quota: %v", err)
	}
	var quotaInfo map[string]interface{}
	if err := json.Unmarshal(responseBody, &quotaInfo); err != nil {
		return fmt.Errorf("failed to parse response body after getting subscription quota: %v", err)
	}

	d.Set("current_value", quotaInfo["currentValue"])
	d.Set("limit", quotaInfo["limit"])
	if quotaInfo["name"] != nil {
		d.SetId(quotaInfo["name"].(string))
	}

	return nil
}

func resourceAzSubscriptionQuotaUpdate(d *schema.ResourceData, m interface{}) error {
	return resourceAzSubscriptionQuotaCreate(d, m)
}

func resourceAzSubscriptionQuotaDelete(d *schema.ResourceData, m interface{}) error {
	// Quotas cannot be deleted, so we just set the Terraform state to null
	d.SetId("")
	// Show info message that quota cannot be deleted, only set to null in Terraform state
	fmt.Printf("Quota with request ID %s cannot be deleted, but has been removed from Terraform state.\n", d.Get("request_id").(string))
	return nil
}