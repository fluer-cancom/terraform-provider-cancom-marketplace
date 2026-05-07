package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAzSubscriptionQuota() *schema.Resource {
	return &schema.Resource{
		Read:        resourceAzSubscriptionQuotaRead,
		Create:      resourceAzSubscriptionQuotaCreate,
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
				Description: "The subscription ID of the Azure subscription.",
			},
			"provider_namespace": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The resource provider namespace of the quota to be managed (e.g. Microsoft.Compute).",
			},
			"location": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The location of the quota to be managed.",
			},
			"quota_family": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The family of the quota to be managed.",
			},
			"quota_resource": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the quota resource to be managed.",
			},
			"limit": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "The desired quota limit.",
			},
			"provisioning_state": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Provisioning state reported by the quota request (e.g. InProgress, Succeeded).",
			},
			"request_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The ID of the quota request in Azure.",
			},
		},
	}
}

func quotaPutURL(cfg *Config, subscriptionId, providerNs, location, quotaFamily string) string {
	return fmt.Sprintf(
		"%s/v1/microsoft/quota/subscriptions/%s/providers/%s/locations/%s/providers/Microsoft.Quota/quotas/%s",
		cfg.Endpoint, subscriptionId, providerNs, location, quotaFamily,
	)
}

func quotaRequestURL(cfg *Config, subscriptionId, providerNs, location, requestId string) string {
	return fmt.Sprintf(
		"%s/v1/microsoft/quota/subscriptions/%s/providers/%s/locations/%s/providers/Microsoft.Quota/quotaRequests/%s",
		cfg.Endpoint, subscriptionId, providerNs, location, requestId,
	)
}

func resourceAzSubscriptionQuotaCreate(d *schema.ResourceData, m interface{}) error {
	cfg := m.(*Config)

	url := quotaPutURL(
		cfg,
		d.Get("subscription_id").(string),
		d.Get("provider_namespace").(string),
		d.Get("location").(string),
		d.Get("quota_family").(string),
	)

	// The marketplace gateway proxies to the Microsoft Quota API which expects
	// limit.value as a string.
	body := map[string]interface{}{
		"properties": map[string]interface{}{
			"limit": map[string]interface{}{
				"value":           strconv.Itoa(d.Get("limit").(int)),
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
		return fmt.Errorf("failed to marshal request body to set subscription quota: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create request to set subscription quota: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.CCMPApiToken)
	req.Header.Set("X-Correlation-ID", "10025")

	resp, err := newMarketplaceClient(60*time.Second, cfg).Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to set subscription quota: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to set subscription quota: %s", resp.Status)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body after setting subscription quota: %w", err)
	}
	var setEnv struct {
		Data struct {
			Name       string `json:"name"`
			Properties struct {
				ProvisioningState string `json:"provisioningState"`
			} `json:"properties"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &setEnv); err != nil {
		return fmt.Errorf("failed to parse response body after setting subscription quota: %w", err)
	}
	if setEnv.Data.Name == "" {
		return fmt.Errorf("quota set response missing data.name")
	}
	d.SetId(setEnv.Data.Name)
	d.Set("request_id", setEnv.Data.Name)
	if setEnv.Data.Properties.ProvisioningState != "" {
		d.Set("provisioning_state", setEnv.Data.Properties.ProvisioningState)
	}
	return nil
}

func resourceAzSubscriptionQuotaRead(d *schema.ResourceData, m interface{}) error {
	cfg := m.(*Config)

	requestId := d.Get("request_id").(string)
	if requestId == "" {
		requestId = d.Id()
	}

	url := quotaRequestURL(
		cfg,
		d.Get("subscription_id").(string),
		d.Get("provider_namespace").(string),
		d.Get("location").(string),
		requestId,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request to get subscription quota: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.CCMPApiToken)

	resp, err := newMarketplaceClient(30*time.Second, cfg).Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to get subscription quota: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get subscription quota: %s", resp.Status)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body after getting subscription quota: %w", err)
	}
	var readEnv struct {
		Data struct {
			Name       string `json:"name"`
			Properties struct {
				ProvisioningState string `json:"provisioningState"`
				Value             []struct {
					Limit struct {
						Value int `json:"value"`
					} `json:"limit"`
					Name struct {
						Value string `json:"value"`
					} `json:"name"`
					ProvisioningState string `json:"provisioningState"`
				} `json:"value"`
			} `json:"properties"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &readEnv); err != nil {
		return fmt.Errorf("failed to parse response body after getting subscription quota: %w", err)
	}

	if readEnv.Data.Name != "" {
		d.SetId(readEnv.Data.Name)
		d.Set("request_id", readEnv.Data.Name)
	}
	if readEnv.Data.Properties.ProvisioningState != "" {
		d.Set("provisioning_state", readEnv.Data.Properties.ProvisioningState)
	}
	if len(readEnv.Data.Properties.Value) > 0 {
		entry := readEnv.Data.Properties.Value[0]
		if entry.Limit.Value != 0 {
			d.Set("limit", entry.Limit.Value)
		}
	}
	return nil
}

func resourceAzSubscriptionQuotaUpdate(d *schema.ResourceData, m interface{}) error {
	return resourceAzSubscriptionQuotaCreate(d, m)
}

func resourceAzSubscriptionQuotaDelete(d *schema.ResourceData, m interface{}) error {
	// Quotas cannot be deleted; we just drop the resource from Terraform state.
	d.SetId("")
	return nil
}
