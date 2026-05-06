package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
)

func subscriptionInfo(subscriptionId string, m *Config) (CSPSubscription, error) {
	// Get Subscription info from Marketplace API
	url := fmt.Sprintf("%s/v1/subscriptions/%s", m.Endpoint, subscriptionId)
	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}
	req, err := http.NewRequest("GET", url, nil)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(m.Username+":"+m.Password)))) // #TODO: Auth needs to be changed to OAuth2 token based
	resp, err := httpClient.Do(req)
	if err != nil {
		return CSPSubscription{}, fmt.Errorf("failed to get Azure subscription info: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return CSPSubscription{}, fmt.Errorf("failed to get Azure subscription info: %s", resp.Status)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return CSPSubscription{}, fmt.Errorf("failed to read Azure subscription info response: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return CSPSubscription{}, fmt.Errorf("failed to parse Azure subscription info response: %v", err)
	}

	CSPSubscriptionInfo := result["data"].(CSPSubscription)

	return CSPSubscriptionInfo, nil
}

func changeSubscription(subscriptionObject CSPSubscription, m *Config) error {
	url := fmt.Sprintf("%s/v1/subscriptions/%s", m.Endpoint, subscriptionObject.SubscriptionId)
	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}
	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request to rename Azure subscription: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(m.Username+":"+m.Password)))) // #TODO: Auth needs to be changed to OAuth2 token based
	req.Header.Set("X-Correlation-ID", 106)

	requestBody, err := json.Marshal(subscriptionObject)
	if err != nil {
		return fmt.Errorf("failed to marshal request body to rename Azure subscription: %v", err)
	}
	req.Body = io.NopCloser(bytes.NewReader(requestBody))

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to rename Azure subscription: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to rename Azure subscription: %s", resp.Status)
	}
	return nil
}

func cancelSubscription(subscriptionId string, m *Config) error {
	// Use subscriptionId to cancel the subscription
	if m.AzureAuthCtx == nil {
		return fmt.Errorf("Cannot authenticate with Azure API. To cancel subscription, please run 'az login' or provide Azure Client ID, Client Secret and Tenant ID and try again")
	}

	clientFactory, err := armsubscription.NewClientFactory(m.AzureAuthCtx, nil)
	if err != nil {
		return err
	}
	_, err = clientFactory.NewClient().Cancel(context.Background(), subscriptionId, nil)
	if err != nil {
		return err
	}
	return nil
}
