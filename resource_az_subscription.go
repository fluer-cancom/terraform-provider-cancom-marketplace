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
			"order_number": {
				Type:        schema.TypeString,
				Required:    false,
				Optional:    true,
				Description: "The PO number of the subscription.",
				ForceNew:    true,
			},
			"azure_discount": {
				Type:        schema.TypeInt,
				Required:    false,
				Optional:    true,
				Description: "The marketplace discount ID for the Azure Plan.",
				ForceNew:    true,
			},
			"azure_owner_object_id": {
				Type:        schema.TypeString,
				Required:    true,
				Optional:    false,
				Description: "The object ID of the principal, which recieves owner permissions after subscription creation.",
				ForceNew:    true,
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
			"request_id": {
				Type:        schema.TypeString,
				Required:    false,
				Optional:    false,
				Computed:    true,
				Description: "The request ID of the Azure subscription.",
			},
		},
	}
}

func resourceAzSubscriptionCreate(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	uri := fmt.Sprintf("%s/azure-api-gateway/v1/createAzureSubscription", config.Endpoint)
	country := config.Country
	parameters := map[string]interface{}{
		"order_number":          d.Get("order_number").(string),
		"azure_discount":        d.Get("azure_discount").(int),
		"azure_owner_object_id": d.Get("azure_owner_object_id").(string),
		"display_name":          d.Get("display_name").(string),
	}

	if parameters["display_name"].(string) != "" {
		if m.(*Config).AzureAuthCtx == nil {
			return fmt.Errorf("Cannot authenticate with Azure API. To set display name, please run 'az login' or provide Azure Client ID, Client Secret and Tenant ID and try again")
		}
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest("POST", uri, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(config.Username+":"+config.Password))))
	q := req.URL.Query()
	if parameters["order_number"].(string) != "" {
		q.Add("orderNumber", parameters["order_number"].(string))
	}
	if parameters["azure_discount"].(int) != 0 {
		q.Add("azureDiscount", fmt.Sprintf("%d", parameters["azure_discount"].(int)))
	}
	q.Add("azureObjectId", parameters["azure_owner_object_id"].(string))
	q.Add("country", country)
	req.URL.RawQuery = q.Encode()

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
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}

	// Get requestId from response
	requestId := result["requestId"].(string)

	// Wait for subscription to be created
	for {
		time.Sleep(5 * time.Second)
		success, err := subscriptionStatusSuccess(requestId, config)
		if err != nil {
			return err
		}
		if success {
			break
		}
	}

	// Get subscription info
	subscriptionInfo, err := subscriptionInfo(requestId, config)
	if err != nil {
		return err
	}

	// Rename subscription, if display_name is set
	if d.Get("display_name").(string) != "" {
		err = renameSubscription(subscriptionInfo["subscriptionId"].(string), d.Get("display_name").(string), config)
		if err != nil {
			return err
		}
	}

	// Set subscription info to resource data
	if subscriptionInfo["orderNumber"] != nil {
		d.Set("order_number", subscriptionInfo["orderNumber"])
	}
	if subscriptionInfo["azureDiscount"] != nil {
		d.Set("azure_discount", subscriptionInfo["azureDiscount"])
	}
	if subscriptionInfo["objectId"] != nil {
		d.Set("azure_owner_object_id", subscriptionInfo["objectId"])
	}
	if subscriptionInfo["subscriptionId"] != nil {
		d.Set("subscription_id", subscriptionInfo["subscriptionId"])
	}

	d.Set("request_id", requestId)
	d.SetId(subscriptionInfo["subscriptionId"].(string))

	return nil
}

func resourceAzSubscriptionRead(d *schema.ResourceData, m interface{}) error {
	subscriptionInfo, err := subscriptionInfo(d.Get("request_id").(string), m.(*Config))
	subscriptionARMInfo, _ := subscriptionARMInfo(d.Id(), m.(*Config))
	if err != nil {
		return err
	}
	if subscriptionInfo["orderNumber"] != nil {
		d.Set("order_number", subscriptionInfo["orderNumber"])
	}
	if subscriptionInfo["azureDiscount"] != nil {
		d.Set("azure_discount", subscriptionInfo["azureDiscount"])
	}
	if subscriptionInfo["objectId"] != nil {
		d.Set("azure_owner_object_id", subscriptionInfo["objectId"])
	}
	if subscriptionInfo["subscriptionId"] != nil {
		d.Set("subscription_id", subscriptionInfo["subscriptionId"])
	}
	if subscriptionARMInfo.DisplayName != nil {
		d.Set("display_name", *subscriptionARMInfo.DisplayName)
	}
	return nil
}

func resourceAzSubscriptionDelete(d *schema.ResourceData, m interface{}) error {
	return cancelSubscription(d.Get("subscription_id").(string), m.(*Config))
}

func resourceAzSubscriptionUpdate(d *schema.ResourceData, m interface{}) error {
	d.Partial(true)
	if d.HasChange("display_name") {
		if m.(*Config).AzureAuthCtx == nil {
			return fmt.Errorf("Cannot authenticate with Azure API. To set display name, please run 'az login' or provide Azure Client ID, Client Secret and Tenant ID and try again")
		}
		err := renameSubscription(d.Get("subscription_id").(string), d.Get("display_name").(string), m.(*Config))
		if err != nil {
			return err
		}
		if err := d.Set("display_name", d.Get("display_name")); err != nil {
			return err
		}
	}
	d.Partial(false)

	return nil
}
