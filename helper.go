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
	"sync/atomic"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
)

var correlationSequence uint64 = uint64(time.Now().UnixNano())

func nextCorrelationID() string {
	return strconv.FormatUint(atomic.AddUint64(&correlationSequence, 1), 10)
}

type marketplaceStatusError struct {
	Operation  string
	StatusCode int
	Status     string
	Body       string
}

func (e *marketplaceStatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("%s: %s", e.Operation, e.Status)
	}
	return fmt.Sprintf("%s: %s: %s", e.Operation, e.Status, e.Body)
}

// subscriptionResponse decodes either the API's {"data": ...} envelope or a
// bare subscription object. Raw fields are retained so a subsequent PUT can
// preserve fields the provider does not understand.
func subscriptionResponse(body []byte) (CSPSubscription, map[string]json.RawMessage, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return CSPSubscription{}, nil, err
	}
	raw := body
	if data, ok := top["data"]; ok {
		raw = data
	}

	var sub CSPSubscription
	if err := json.Unmarshal(raw, &sub); err != nil {
		return CSPSubscription{}, nil, err
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(raw, &document); err != nil {
		return CSPSubscription{}, nil, err
	}
	return sub, document, nil
}

func subscriptionInfoDocument(subscriptionId string, m *Config) (CSPSubscription, map[string]json.RawMessage, error) {
	url := fmt.Sprintf("%s/v1/subscriptions/%s", m.Endpoint, url.PathEscape(subscriptionId))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return CSPSubscription{}, nil, fmt.Errorf("failed to build request for subscription info: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.CCMPApiToken)

	resp, err := newMarketplaceClient(10*time.Second, m).Do(req)
	if err != nil {
		return CSPSubscription{}, nil, fmt.Errorf("failed to get marketplace subscription info: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CSPSubscription{}, nil, fmt.Errorf("failed to read subscription info response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return CSPSubscription{}, nil, &marketplaceStatusError{
			Operation: "failed to get marketplace subscription info", StatusCode: resp.StatusCode,
			Status: resp.Status, Body: string(body),
		}
	}

	sub, document, err := subscriptionResponse(body)
	if err != nil {
		return CSPSubscription{}, nil, fmt.Errorf("failed to parse subscription info response: %w", err)
	}
	if sub.Id == "" {
		return CSPSubscription{}, nil, fmt.Errorf("subscription info response missing data.id")
	}
	return sub, document, nil
}

func newMarketplaceClient(timeout time.Duration, cfg *Config) *http.Client {
	if cfg.HTTPClient != nil {
		return cfg.HTTPClient
	}
	return &http.Client{Timeout: timeout}
}

// subscriptionInfo fetches a subscription record from the marketplace API.
func subscriptionInfo(subscriptionId string, m *Config) (CSPSubscription, error) {
	sub, _, err := subscriptionInfoDocument(subscriptionId, m)
	return sub, err
}

// changeSubscription PUTs an updated subscription object back to the API.
// Per the API spec the path has no {id} segment — the id lives in the body.
func changeSubscriptionDocument(subscriptionObject map[string]json.RawMessage, m *Config) error {
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
	req.Header.Set("X-Correlation-ID", nextCorrelationID())

	resp, err := newMarketplaceClient(10*time.Second, m).Do(req)
	if err != nil {
		return fmt.Errorf("failed to send subscription update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &marketplaceStatusError{Operation: "failed to update subscription", StatusCode: resp.StatusCode, Status: resp.Status, Body: string(body)}
	}
	return nil
}

// changeSubscription remains useful for callers that already have a complete
// object. Resource updates use changeSubscriptionDocument to avoid losing
// fields that are new or undocumented in the API.
func changeSubscription(subscriptionObject CSPSubscription, m *Config) error {
	body, err := json.Marshal(subscriptionObject)
	if err != nil {
		return fmt.Errorf("failed to marshal subscription update body: %w", err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(body, &document); err != nil {
		return fmt.Errorf("failed to prepare subscription update body: %w", err)
	}
	return changeSubscriptionDocument(document, m)
}

func setRawField(document map[string]json.RawMessage, name string, value interface{}) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	document[name] = raw
	return nil
}

func nestedRawObject(document map[string]json.RawMessage, name string) (map[string]json.RawMessage, error) {
	var nested map[string]json.RawMessage
	if raw, ok := document[name]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &nested); err != nil {
			return nil, fmt.Errorf("subscription field %q is not an object: %w", name, err)
		}
	}
	if nested == nil {
		nested = make(map[string]json.RawMessage)
	}
	return nested, nil
}

func storeNestedRawObject(document map[string]json.RawMessage, name string, nested map[string]json.RawMessage) error {
	raw, err := json.Marshal(nested)
	if err != nil {
		return err
	}
	document[name] = raw
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
