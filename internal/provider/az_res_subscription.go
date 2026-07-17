package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"terraform-provider-cancommarketplace/internal/azure"
	"terraform-provider-cancommarketplace/internal/marketplace"
)

const azureSubscriptionPaymentPlanID = 172495

var subscriptionPollInterval = 5 * time.Second

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
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
		},
		Schema: map[string]*schema.Schema{
			"payment_plan_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The fixed payment plan ID of the Azure subscription.",
			},
			"display_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The display name of the subscription.",
			},
			"azure_owner_object_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The Azure principal object ID that should receive the Owner role on the created subscription.",
			},
			"subscription_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The Azure subscription ID returned as externalAccountId by the Marketplace API.",
			},
			"marketplace_subscription_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The CANCOM Marketplace subscription ID used for Marketplace API operations.",
			},
		},
	}
}

func resourceAzSubscriptionCreate(d *schema.ResourceData, m interface{}) error {
	cfg := m.(*Config)
	displayName := d.Get("display_name").(string)

	if err := cfg.Azure.EnsureReady(context.Background(), requiredAzureOperationsForSubscriptionCreate(d)); err != nil {
		return fmt.Errorf("cannot configure display_name/azure_owner_object_id: %w", err)
	}

	sub, _, err := cfg.Marketplace.CreateAzureSubscription(cfg.MarketplaceUserID, azureSubscriptionPaymentPlanID)
	if err != nil {
		return err
	}

	//FIXME: Ensure Subscription is evident in state after successful creation. If further actions with Azure Management API are required, never forget to set the subscription_id in state, otherwise Terraform will try to create a new subscription on next apply.
	d.SetId(sub.Id)
	if err := setSubscriptionState(d, sub); err != nil {
		return err
	}
	activeSub, document, err := waitForSubscriptionActive(sub.Id, cfg, d.Timeout(schema.TimeoutCreate))
	if err != nil {
		return err
	}
	if err := setSubscriptionState(d, activeSub); err != nil {
		return err
	}
	if ownerObjectID := d.Get("azure_owner_object_id").(string); ownerObjectID != "" {
		if activeSub.ExternalAccountId == "" {
			return fmt.Errorf("cannot assign Owner role: Marketplace API returned no externalAccountId (Azure subscription ID)")
		}
		if err := cfg.Azure.AssignOwnerRole(context.Background(), activeSub.ExternalAccountId, ownerObjectID); err != nil {
			return err
		}
		if err := d.Set("azure_owner_object_id", ownerObjectID); err != nil {
			return fmt.Errorf("failed to set azure_owner_object_id: %w", err)
		}
	}

	// PUT requires the complete object and Azure-backed changes must not be sent
	// before the marketplace order has reached ACTIVE.
	if displayName != "" {
		if err := marketplace.SetRawField(document, "label", displayName); err != nil {
			return fmt.Errorf("failed to set subscription label: %w", err)
		}
		//FIXME: This is not meant to set the displayName. displayName changes are not supported by the marketplace API. Use Azure Management API instead.
		if err := cfg.Marketplace.ChangeSubscriptionDocument(document); err != nil {
			return err
		}
		if err := d.Set("display_name", displayName); err != nil {
			return fmt.Errorf("failed to set subscription display_name: %w", err)
		}
	}
	return nil
}

func requiredAzureOperationsForSubscriptionCreate(d *schema.ResourceData) []azure.Operation {
	var operations []azure.Operation
	if d.Get("display_name").(string) != "" {
		operations = append(operations, azure.OperationRenameSubscription)
	}
	if d.Get("azure_owner_object_id").(string) != "" {
		operations = append(operations, azure.OperationAssignOwnerRole)
	}
	return operations
}

func waitForSubscriptionActive(subscriptionID string, cfg *Config, timeout time.Duration) (marketplace.Subscription, map[string]json.RawMessage, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(subscriptionPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-deadline.C:
			return marketplace.Subscription{}, nil, fmt.Errorf("timed out waiting for marketplace subscription %s order to become ACTIVE", subscriptionID)
		case <-ticker.C:
			sub, document, err := cfg.Marketplace.SubscriptionInfoDocument(subscriptionID)
			if err != nil {
				var statusErr *marketplace.StatusError
				if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
					continue
				}
				return marketplace.Subscription{}, nil, fmt.Errorf("failed while waiting for marketplace subscription %s: %w", subscriptionID, err)
			}
			if sub.Order.Status == "ACTIVE" {
				return sub, document, nil
			}
		}
	}
}

func resourceAzSubscriptionRead(d *schema.ResourceData, m interface{}) error {
	cfg := m.(*Config)

	id := d.Id()
	if id == "" {
		id = d.Get("marketplace_subscription_id").(string)
	}

	sub, err := cfg.Marketplace.SubscriptionInfo(id)
	if err != nil {
		var statusErr *marketplace.StatusError
		if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
			d.SetId("")
			return nil
		}
		return err
	}

	return setSubscriptionState(d, sub)
}

func resourceAzSubscriptionUpdate(d *schema.ResourceData, m interface{}) error {
	cfg := m.(*Config)

	_, document, err := cfg.Marketplace.SubscriptionInfoDocument(d.Id())
	if err != nil {
		return err
	}

	marketplaceDocumentChanged := false
	if d.HasChange("display_name") {
		dn := d.Get("display_name").(string)
		if dn == "" {
			if err := marketplace.SetRawField(document, "label", nil); err != nil {
				return err
			}
		} else if err := marketplace.SetRawField(document, "label", dn); err != nil {
			return err
		}
		marketplaceDocumentChanged = true
	}
	if d.HasChange("azure_owner_object_id") {
		owner := d.Get("azure_owner_object_id").(string)
		if owner != "" {
			if err := cfg.Azure.EnsureReady(context.Background(), []azure.Operation{azure.OperationAssignOwnerRole}); err != nil {
				return fmt.Errorf("cannot configure azure_owner_object_id: %w", err)
			}
			azureSubscriptionID := d.Get("subscription_id").(string)
			if azureSubscriptionID == "" {
				sub, err := cfg.Marketplace.SubscriptionInfo(d.Id())
				if err != nil {
					return err
				}
				azureSubscriptionID = sub.ExternalAccountId
			}
			if azureSubscriptionID == "" {
				return fmt.Errorf("cannot assign Owner role: Marketplace API returned no externalAccountId (Azure subscription ID)")
			}
			if err := cfg.Azure.AssignOwnerRole(context.Background(), azureSubscriptionID, owner); err != nil {
				return err
			}
		}
	}

	if marketplaceDocumentChanged {
		return cfg.Marketplace.ChangeSubscriptionDocument(document)
	}
	return nil
}

func resourceAzSubscriptionDelete(d *schema.ResourceData, m interface{}) error {
	cfg := m.(*Config)
	azureSubscriptionID := d.Get("subscription_id").(string)
	if azureSubscriptionID == "" {
		return fmt.Errorf("cannot cancel subscription: Marketplace API returned no externalAccountId (Azure subscription ID)")
	}
	return cfg.Azure.CancelSubscription(azureSubscriptionID)
}

func setSubscriptionState(d *schema.ResourceData, sub marketplace.Subscription) error {
	paymentPlanID := sub.Order.PaymentPlan.Id
	if paymentPlanID == 0 {
		paymentPlanID = sub.Order.PaymentPlanId
	}
	values := map[string]interface{}{
		"marketplace_subscription_id": sub.Id,
	}
	if sub.ExternalAccountId != "" {
		values["subscription_id"] = sub.ExternalAccountId
	}
	if paymentPlanID != 0 {
		values["payment_plan_id"] = paymentPlanID
	}
	if sub.Label != nil {
		values["display_name"] = *sub.Label
	} else {
		values["display_name"] = ""
	}
	for name, value := range values {
		if err := d.Set(name, value); err != nil {
			return fmt.Errorf("failed to set subscription state field %s: %w", name, err)
		}
	}
	return nil
}
