package marketplace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"
)

var correlationSequence uint64 = uint64(time.Now().UnixNano())

type Client struct {
	Endpoint   string
	Token      string
	HTTPClient *http.Client
}

type StatusError struct {
	Operation  string
	StatusCode int
	Status     string
	Body       string
}

func (e *StatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("%s: %s", e.Operation, e.Status)
	}
	return fmt.Sprintf("%s: %s: %s", e.Operation, e.Status, e.Body)
}

func (c *Client) httpClient(timeout time.Duration) *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: timeout}
}

func nextCorrelationID() string {
	return strconv.FormatUint(atomic.AddUint64(&correlationSequence, 1), 10)
}

func (c *Client) addHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
}

func (c *Client) addMutationHeaders(req *http.Request) {
	c.addHeaders(req)
	req.Header.Set("X-Correlation-ID", nextCorrelationID())
}

func SubscriptionResponse(body []byte) (Subscription, map[string]json.RawMessage, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return Subscription{}, nil, err
	}
	raw := body
	if data, ok := top["data"]; ok {
		raw = data
	}

	var sub Subscription
	if err := json.Unmarshal(raw, &sub); err != nil {
		return Subscription{}, nil, err
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(raw, &document); err != nil {
		return Subscription{}, nil, err
	}
	return sub, document, nil
}

func (c *Client) CreateAzureSubscription(userUUID string, paymentPlanID int) (Subscription, map[string]json.RawMessage, error) {
	uri := fmt.Sprintf("%s/v1/subscriptions", c.Endpoint)
	body := map[string]interface{}{
		"order": map[string]interface{}{
			"paymentPlanId": paymentPlanID,
		},
	}
	requestBody, err := json.Marshal(body)
	if err != nil {
		return Subscription{}, nil, err
	}

	req, err := http.NewRequest("POST", uri, bytes.NewReader(requestBody))
	if err != nil {
		return Subscription{}, nil, err
	}
	q := req.URL.Query()
	q.Add("userUUID", userUUID)
	req.URL.RawQuery = q.Encode()
	c.addMutationHeaders(req)

	resp, err := c.httpClient(120 * time.Second).Do(req)
	if err != nil {
		return Subscription{}, nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Subscription{}, nil, err
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return Subscription{}, nil, fmt.Errorf("failed to create Azure subscription: %s \n Error: %s", resp.Status, string(respBody))
	}

	sub, document, err := SubscriptionResponse(respBody)
	if err != nil {
		return Subscription{}, nil, fmt.Errorf("failed to parse subscription create response: %w; body=%s", err, string(respBody))
	}
	if sub.Id == "" {
		return Subscription{}, nil, fmt.Errorf("subscription create returned no id; body=%s", string(respBody))
	}
	return sub, document, nil
}

func (c *Client) SubscriptionInfoDocument(subscriptionID string) (Subscription, map[string]json.RawMessage, error) {
	uri := fmt.Sprintf("%s/v1/subscriptions/%s", c.Endpoint, url.PathEscape(subscriptionID))

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return Subscription{}, nil, fmt.Errorf("failed to build request for subscription info: %w", err)
	}
	c.addHeaders(req)

	resp, err := c.httpClient(10 * time.Second).Do(req)
	if err != nil {
		return Subscription{}, nil, fmt.Errorf("failed to get marketplace subscription info: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Subscription{}, nil, fmt.Errorf("failed to read subscription info response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Subscription{}, nil, &StatusError{
			Operation: "failed to get marketplace subscription info", StatusCode: resp.StatusCode,
			Status: resp.Status, Body: string(body),
		}
	}

	sub, document, err := SubscriptionResponse(body)
	if err != nil {
		return Subscription{}, nil, fmt.Errorf("failed to parse subscription info response: %w", err)
	}
	if sub.Id == "" {
		return Subscription{}, nil, fmt.Errorf("subscription info response missing data.id")
	}
	return sub, document, nil
}

func (c *Client) SubscriptionInfo(subscriptionID string) (Subscription, error) {
	sub, _, err := c.SubscriptionInfoDocument(subscriptionID)
	return sub, err
}

func (c *Client) ChangeSubscriptionDocument(subscriptionObject map[string]json.RawMessage) error {
	uri := fmt.Sprintf("%s/v1/subscriptions", c.Endpoint)

	requestBody, err := json.Marshal(subscriptionObject)
	if err != nil {
		return fmt.Errorf("failed to marshal subscription update body: %w", err)
	}

	req, err := http.NewRequest("PUT", uri, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("failed to build request to update subscription: %w", err)
	}
	c.addMutationHeaders(req)

	resp, err := c.httpClient(10 * time.Second).Do(req)
	if err != nil {
		return fmt.Errorf("failed to send subscription update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &StatusError{Operation: "failed to update subscription", StatusCode: resp.StatusCode, Status: resp.Status, Body: string(body)}
	}
	return nil
}

func (c *Client) ChangeSubscription(subscriptionObject Subscription) error {
	body, err := json.Marshal(subscriptionObject)
	if err != nil {
		return fmt.Errorf("failed to marshal subscription update body: %w", err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(body, &document); err != nil {
		return fmt.Errorf("failed to prepare subscription update body: %w", err)
	}
	return c.ChangeSubscriptionDocument(document)
}
