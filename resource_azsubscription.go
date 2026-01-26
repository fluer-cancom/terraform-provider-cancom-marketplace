package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAzSubscription() *schema.Resource {
	return &schema.Resource{
		Create: resourceAzSubscriptionCreate,
		Read:   nil,
		Update: nil,
		Delete: nil,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"orderNumber": {
				Type:        schema.TypeString,
				Required:    false,
				Optional:    true,
				Description: "The PO number of the subscription.",
				ForceNew:    true,
			},
			"azureDiscount": {
				Type:        schema.TypeInt,
				Required:    false,
				Optional:    true,
				Description: "The marketplace discount ID for the Azure Plan.",
				ForceNew:    true,
			},
			"azureOwnerObjectId": {
				Type:        schema.TypeString,
				Required:    true,
				Optional:    false,
				Description: "The object ID of the principal, which recieves owner permissions after subscription creation.",
				ForceNew:    true,
			},
			"country": {
				Type:        schema.TypeString,
				Required:    true,
				Optional:    false,
				Default:     "DE",
				Description: "The country of the customer.",
				ForceNew:    true,
			},
			"subscriptionId": {
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
	uri := fmt.Sprintf("%s/azure-api-gateway/v1/createAzureSubscription", m.(map[string]interface{})["endpoint"].(string))
	parameters := map[string]interface{}{
		"orderNumber":   d.Get("orderNumber").(string),
		"azureDiscount": d.Get("azureDiscount").(int),
		"azureObjectId": d.Get("azureOwnerObjectId").(string),
		"country":       m.(map[string]interface{})["country"].(string),
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest("POST", uri, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", m.(map[string]interface{})["api_username"].(string)+":"+m.(map[string]interface{})["api_password"].(string)))
	q := req.URL.Query()
	q.Add("orderNumber", parameters["orderNumber"].(string))
	q.Add("azureDiscount", fmt.Sprintf("%d", parameters["azureDiscount"].(int)))
	q.Add("azureObjectId", parameters["azureObjectId"].(string))
	q.Add("country", parameters["country"].(string))
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
		success, err := subscriptionStatusSuccess(requestId, m)
		if err != nil {
			return err
		}
		if success {
			break
		}
	}

	// Get subscription info
	subscriptionInfo, err := subscriptionInfo(requestId, m)
	if err != nil {
		return err
	}

	// Set subscription info to resource data
	d.Set("orderNumber", subscriptionInfo["orderNumber"])
	d.Set("azureDiscount", subscriptionInfo["azureDiscount"])
	d.Set("azureOwnerObjectId", subscriptionInfo["objectId"])
	d.Set("subscriptionId", subscriptionInfo["subscriptionId"])

	return nil
}
