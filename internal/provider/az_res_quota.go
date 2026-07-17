package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"terraform-provider-cancom-marketplace/internal/marketplace"

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

func resourceAzSubscriptionQuotaCreate(d *schema.ResourceData, m interface{}) error {
	return setAzSubscriptionQuota(d, m, d.Timeout(schema.TimeoutCreate))
}

func setAzSubscriptionQuota(d *schema.ResourceData, m interface{}, timeout time.Duration) error {
	cfg := m.(*Config)

	result, err := cfg.Marketplace.SetQuota(
		d.Get("subscription_id").(string),
		d.Get("provider_namespace").(string),
		d.Get("location").(string),
		d.Get("quota_resource").(string),
		d.Get("limit").(int),
	)
	if err != nil {
		return err
	}
	d.SetId(result.Name)
	if err := d.Set("request_id", result.Name); err != nil {
		return fmt.Errorf("failed to set quota request ID in state: %w", err)
	}
	if result.ProvisioningState != "" {
		if err := d.Set("provisioning_state", result.ProvisioningState); err != nil {
			return fmt.Errorf("failed to set quota provisioning state: %w", err)
		}
	}
	if result.ProvisioningState == "InProgress" {
		return waitForQuotaRequest(d, m, timeout)
	}
	if result.ProvisioningState == "Failed" || result.ProvisioningState == "Canceled" {
		return fmt.Errorf("quota request %s finished with state %s", result.Name, result.ProvisioningState)
	}
	return nil
}

func resourceAzSubscriptionQuotaRead(d *schema.ResourceData, m interface{}) error {
	cfg := m.(*Config)

	requestId := d.Get("request_id").(string)
	if requestId == "" {
		requestId = d.Id()
	}

	result, err := cfg.Marketplace.QuotaRequest(
		d.Get("subscription_id").(string),
		d.Get("provider_namespace").(string),
		d.Get("location").(string),
		requestId,
	)
	if err != nil {
		var statusErr *marketplace.StatusError
		if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
			d.SetId("")
			return nil
		}
		return err
	}
	if result.Name != "" {
		d.SetId(result.Name)
		if err := d.Set("request_id", result.Name); err != nil {
			return fmt.Errorf("failed to set quota request ID in state: %w", err)
		}
	}
	if result.ProvisioningState != "" {
		if err := d.Set("provisioning_state", result.ProvisioningState); err != nil {
			return fmt.Errorf("failed to set quota provisioning state: %w", err)
		}
	}
	if result.ResourceName != "" {
		if err := d.Set("limit", result.Limit); err != nil {
			return fmt.Errorf("failed to set quota limit in state: %w", err)
		}
		configuredResource := d.Get("quota_resource").(string)
		if configuredResource == "" {
			if err := d.Set("quota_resource", result.ResourceName); err != nil {
				return fmt.Errorf("failed to set quota resource in state: %w", err)
			}
		} else if !strings.EqualFold(configuredResource, result.ResourceName) {
			return fmt.Errorf("quota request returned resource %q, expected %q", result.ResourceName, configuredResource)
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
