package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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
			"user_uuid": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The marketplace user UUID for which the subscription is created.",
			},
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
				Computed:    true,
				Deprecated:  "azure_owner_object_id is a legacy name for the marketplace user UUID; use user_uuid for new configurations",
				Description: "Legacy alias for the marketplace user UUID. This is not an Azure AD object ID.",
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

	uri := fmt.Sprintf("%s/v1/subscriptions", cfg.Endpoint)

	body := map[string]interface{}{
		"order": map[string]interface{}{
			"paymentPlanId": azureSubscriptionPaymentPlanID,
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
	req.Header.Set("X-Correlation-ID", nextCorrelationID())

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
	sub, _, jsonErr := subscriptionResponse(respBody)
	if jsonErr != nil {
		return fmt.Errorf("failed to parse subscription create response: %w; body=%s", jsonErr, string(respBody))
	}
	if sub.Id == "" {
		return fmt.Errorf("subscription create returned no id; body=%s", string(respBody))
	}

	//#FIXME: Ensure Subscription is evident in state after successful creation. If further actions with Azure Management API are required, never forget to set the subscription_id in state, otherwise Terraform will try to create a new subscription on next apply.
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

	// PUT requires the complete object and Azure-backed changes must not be sent
	// before the marketplace order has reached ACTIVE.
	if displayName != "" {
		if err := setRawField(document, "label", displayName); err != nil {
			return fmt.Errorf("failed to set subscription label: %w", err)
		}
		//#FIXME: This is not meant to set the displayName. displayName changes are not supported by the marketplace API. Use Azure Management API instead.
		if err := changeSubscriptionDocument(document, cfg); err != nil {
			return err
		}
		if err := d.Set("display_name", displayName); err != nil {
			return fmt.Errorf("failed to set subscription display_name: %w", err)
		}
	}
	return nil
}

func waitForSubscriptionActive(subscriptionID string, cfg *Config, timeout time.Duration) (CSPSubscription, map[string]json.RawMessage, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(subscriptionPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-deadline.C:
			return CSPSubscription{}, nil, fmt.Errorf("timed out waiting for marketplace subscription %s order to become ACTIVE", subscriptionID)
		case <-ticker.C:
			sub, document, err := subscriptionInfoDocument(subscriptionID, cfg)
			if err != nil {
				var statusErr *marketplaceStatusError
				if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
					continue
				}
				return CSPSubscription{}, nil, fmt.Errorf("failed while waiting for marketplace subscription %s: %w", subscriptionID, err)
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

	sub, err := subscriptionInfo(id, cfg)
	if err != nil {
		var statusErr *marketplaceStatusError
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

	_, document, err := subscriptionInfoDocument(d.Id(), cfg)
	if err != nil {
		return err
	}

	if d.HasChange("display_name") {
		dn := d.Get("display_name").(string)
		if dn == "" {
			if err := setRawField(document, "label", nil); err != nil {
				return err
			}
		} else if err := setRawField(document, "label", dn); err != nil {
			return err
		}
	}
	if d.HasChange("azure_owner_object_id") {
		owner := d.Get("azure_owner_object_id").(string)
		if owner != "" {
			user, err := nestedRawObject(document, "user")
			if err != nil {
				return err
			}
			if err := setRawField(user, "id", owner); err != nil {
				return err
			}
			if err := storeNestedRawObject(document, "user", user); err != nil {
				return err
			}
			order, err := nestedRawObject(document, "order")
			if err != nil {
				return err
			}
			if _, ok := order["user"]; ok {
				orderUser, err := nestedRawObject(order, "user")
				if err != nil {
					return err
				}
				if err := setRawField(orderUser, "id", owner); err != nil {
					return err
				}
				if err := storeNestedRawObject(order, "user", orderUser); err != nil {
					return err
				}
				if err := storeNestedRawObject(document, "order", order); err != nil {
					return err
				}
			}
		}
	}

	return changeSubscriptionDocument(document, cfg)
}

func resourceAzSubscriptionDelete(d *schema.ResourceData, m interface{}) error {
	cfg := m.(*Config)
	azureSubscriptionID := d.Get("subscription_id").(string)
	if azureSubscriptionID == "" {
		return fmt.Errorf("cannot cancel subscription: Marketplace API returned no externalAccountId (Azure subscription ID)")
	}
	return cancelSubscription(azureSubscriptionID, cfg)
}

func setSubscriptionState(d *schema.ResourceData, sub CSPSubscription) error {
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
	if sub.User.Id != "" {
		values["user_uuid"] = sub.User.Id
		values["azure_owner_object_id"] = sub.User.Id
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
