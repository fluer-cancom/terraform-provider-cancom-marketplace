package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
)

func newMarketplaceClient(timeout time.Duration, cfg *Config) *http.Client {
	if cfg.HTTPClient != nil {
		return cfg.HTTPClient
	}
	return &http.Client{Timeout: timeout}
}

// subscriptionInfo fetches a subscription record from the marketplace API.
func subscriptionInfo(subscriptionId string, m *Config) (CSPSubscription, error) {
	url := fmt.Sprintf("%s/v1/subscriptions/%s", m.Endpoint, subscriptionId)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return CSPSubscription{}, fmt.Errorf("failed to build request for subscription info: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.CCMPApiToken)

	resp, err := newMarketplaceClient(10*time.Second, m).Do(req)
	if err != nil {
		return CSPSubscription{}, fmt.Errorf("failed to get marketplace subscription info: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return CSPSubscription{}, fmt.Errorf("failed to get marketplace subscription info: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CSPSubscription{}, fmt.Errorf("failed to read subscription info response: %w", err)
	}
	var envelope struct {
		Data CSPSubscription `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return CSPSubscription{}, fmt.Errorf("failed to parse subscription info response: %w", err)
	}
	return envelope.Data, nil
}

// changeSubscription PUTs an updated subscription object back to the API.
// Per the API spec the path has no {id} segment — the id lives in the body.
func changeSubscription(subscriptionObject CSPSubscription, m *Config) error {
	url := fmt.Sprintf("%s/v1/subscriptions", m.Endpoint)

	requestBody, err := json.Marshal(subscriptionObject)
	if err != nil {
		return fmt.Errorf("failed to marshal subscription update body: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("failed to build request to update subscription: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.CCMPApiToken)
	req.Header.Set("X-Correlation-ID", "106")

	resp, err := newMarketplaceClient(10*time.Second, m).Do(req)
	if err != nil {
		return fmt.Errorf("failed to send subscription update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update subscription: %s", resp.Status)
	}
	return nil
}

// cancelSubscription cancels the underlying Azure subscription via ARM.
func cancelSubscription(subscriptionId string, m *Config) error {
	if m.AzureAuthCtx == nil {
		return fmt.Errorf("cannot authenticate with Azure API. To cancel subscription, run 'az login' or set azure_client_id/azure_client_secret/azure_tenant_id")
	}
	clientFactory, err := armsubscription.NewClientFactory(m.AzureAuthCtx, nil)
	if err != nil {
		return err
	}
	if _, err := clientFactory.NewClient().Cancel(context.Background(), subscriptionId, nil); err != nil {
		return err
	}
	return nil
}
