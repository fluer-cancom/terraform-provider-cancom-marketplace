package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var quotaPollInterval = 10 * time.Second

func resourceAzSubscriptionQuota() *schema.Resource {
	return &schema.Resource{
		Read:        resourceAzSubscriptionQuotaRead,
		Create:      resourceAzSubscriptionQuotaCreate,
		Update:      resourceAzSubscriptionQuotaUpdate,
		Delete:      resourceAzSubscriptionQuotaDelete,
		Description: "Manages the quota of an Azure Subscription within the Cancom Marketplace.",
		Importer: &schema.ResourceImporter{
			StateContext: resourceAzSubscriptionQuotaImport,
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
			Update: schema.DefaultTimeout(30 * time.Minute),
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
				Optional:    true,
				Deprecated:  "quota_family is no longer used; quota_resource identifies the API quota resource",
				Description: "Deprecated legacy field. Use quota_resource.",
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
		cfg.Endpoint, url.PathEscape(subscriptionId), url.PathEscape(providerNs), url.PathEscape(location), url.PathEscape(quotaFamily),
	)
}

func quotaRequestURL(cfg *Config, subscriptionId, providerNs, location, requestId string) string {
	return fmt.Sprintf(
		"%s/v1/microsoft/quota/subscriptions/%s/providers/%s/locations/%s/providers/Microsoft.Quota/quotaRequests/%s",
		cfg.Endpoint, url.PathEscape(subscriptionId), url.PathEscape(providerNs), url.PathEscape(location), url.PathEscape(requestId),
	)
}

func resourceAzSubscriptionQuotaCreate(d *schema.ResourceData, m interface{}) error {
	return setAzSubscriptionQuota(d, m, d.Timeout(schema.TimeoutCreate))
}

func setAzSubscriptionQuota(d *schema.ResourceData, m interface{}, timeout time.Duration) error {
	cfg := m.(*Config)

	url := quotaPutURL(
		cfg,
		d.Get("subscription_id").(string),
		d.Get("provider_namespace").(string),
		d.Get("location").(string),
		d.Get("quota_resource").(string),
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
	req.Header.Set("X-Correlation-ID", nextCorrelationID())

	resp, err := newMarketplaceClient(60*time.Second, cfg).Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to set subscription quota: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return &marketplaceStatusError{Operation: "failed to set subscription quota", StatusCode: resp.StatusCode, Status: resp.Status, Body: string(respBody)}
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
	if err := d.Set("request_id", setEnv.Data.Name); err != nil {
		return fmt.Errorf("failed to set quota request ID in state: %w", err)
	}
	if setEnv.Data.Properties.ProvisioningState != "" {
		if err := d.Set("provisioning_state", setEnv.Data.Properties.ProvisioningState); err != nil {
			return fmt.Errorf("failed to set quota provisioning state: %w", err)
		}
	}
	if setEnv.Data.Properties.ProvisioningState == "InProgress" {
		return waitForQuotaRequest(d, m, timeout)
	}
	if setEnv.Data.Properties.ProvisioningState == "Failed" || setEnv.Data.Properties.ProvisioningState == "Canceled" {
		return fmt.Errorf("quota request %s finished with state %s", setEnv.Data.Name, setEnv.Data.Properties.ProvisioningState)
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
		respBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusNotFound {
			d.SetId("")
			return nil
		}
		return &marketplaceStatusError{Operation: "failed to get subscription quota", StatusCode: resp.StatusCode, Status: resp.Status, Body: string(respBody)}
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
		if err := d.Set("request_id", readEnv.Data.Name); err != nil {
			return fmt.Errorf("failed to set quota request ID in state: %w", err)
		}
	}
	if readEnv.Data.Properties.ProvisioningState != "" {
		if err := d.Set("provisioning_state", readEnv.Data.Properties.ProvisioningState); err != nil {
			return fmt.Errorf("failed to set quota provisioning state: %w", err)
		}
	}
	if len(readEnv.Data.Properties.Value) > 0 {
		entry := readEnv.Data.Properties.Value[0]
		if err := d.Set("limit", entry.Limit.Value); err != nil {
			return fmt.Errorf("failed to set quota limit in state: %w", err)
		}
		if entry.Name.Value != "" {
			configuredResource := d.Get("quota_resource").(string)
			if configuredResource == "" {
				if err := d.Set("quota_resource", entry.Name.Value); err != nil {
					return fmt.Errorf("failed to set quota resource in state: %w", err)
				}
			} else if !strings.EqualFold(configuredResource, entry.Name.Value) {
				return fmt.Errorf("quota request returned resource %q, expected %q", entry.Name.Value, configuredResource)
			}
		}
	}
	return nil
}

func resourceAzSubscriptionQuotaImport(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
	parts := strings.Split(d.Id(), ",")
	if len(parts) != 4 {
		return nil, fmt.Errorf("quota import ID must be subscription_id,provider_namespace,location,quota_request_id")
	}
	fields := []string{"subscription_id", "provider_namespace", "location", "request_id"}
	for i, field := range fields {
		value := strings.TrimSpace(parts[i])
		if value == "" {
			return nil, fmt.Errorf("quota import field %s cannot be empty", field)
		}
		if err := d.Set(field, value); err != nil {
			return nil, fmt.Errorf("failed to set imported quota field %s: %w", field, err)
		}
	}
	d.SetId(strings.TrimSpace(parts[3]))
	return []*schema.ResourceData{d}, nil
}

func resourceAzSubscriptionQuotaUpdate(d *schema.ResourceData, m interface{}) error {
	return setAzSubscriptionQuota(d, m, d.Timeout(schema.TimeoutUpdate))
}

func resourceAzSubscriptionQuotaDelete(d *schema.ResourceData, m interface{}) error {
	// Quotas cannot be deleted; we just drop the resource from Terraform state.
	d.SetId("")
	return nil
}

func waitForQuotaRequest(d *schema.ResourceData, m interface{}, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(quotaPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-deadline.C:
			return fmt.Errorf("timed out waiting for quota request %s to complete", d.Id())
		case <-ticker.C:
			if err := resourceAzSubscriptionQuotaRead(d, m); err != nil {
				return err
			}
			if d.Id() == "" {
				return fmt.Errorf("quota request disappeared while waiting for completion")
			}
			state := d.Get("provisioning_state").(string)
			switch state {
			case "Succeeded":
				return nil
			case "Failed", "Canceled":
				return fmt.Errorf("quota request %s finished with state %s", d.Id(), state)
			}
		}
	}
}
