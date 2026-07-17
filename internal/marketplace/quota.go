package marketplace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type QuotaSetResponse struct {
	Name              string
	ProvisioningState string
}

type QuotaReadResponse struct {
	Name              string
	ProvisioningState string
	Limit             int
	ResourceName      string
}

func quotaPutURL(endpoint, subscriptionID, providerNs, location, quotaResource string) string {
	return fmt.Sprintf(
		"%s/v1/microsoft/quota/subscriptions/%s/providers/%s/locations/%s/providers/Microsoft.Quota/quotas/%s",
		endpoint, url.PathEscape(subscriptionID), url.PathEscape(providerNs), url.PathEscape(location), url.PathEscape(quotaResource),
	)
}

func quotaRequestURL(endpoint, subscriptionID, providerNs, location, requestID string) string {
	return fmt.Sprintf(
		"%s/v1/microsoft/quota/subscriptions/%s/providers/%s/locations/%s/providers/Microsoft.Quota/quotaRequests/%s",
		endpoint, url.PathEscape(subscriptionID), url.PathEscape(providerNs), url.PathEscape(location), url.PathEscape(requestID),
	)
}

func (c *Client) SetQuota(subscriptionID, providerNs, location, quotaResource string, limit int) (QuotaSetResponse, error) {
	uri := quotaPutURL(c.Endpoint, subscriptionID, providerNs, location, quotaResource)
	body := map[string]interface{}{
		"properties": map[string]interface{}{
			"limit": map[string]interface{}{
				"value":           strconv.Itoa(limit),
				"limitObjectType": "LimitValue",
			},
			"name": map[string]interface{}{
				"value": quotaResource,
			},
			"properties": map[string]interface{}{
				"requestOrigin": "Microsoft_Azure_Capacity/QuotaApproval.ReactView",
			},
		},
	}
	requestBody, err := json.Marshal(body)
	if err != nil {
		return QuotaSetResponse{}, fmt.Errorf("failed to marshal request body to set subscription quota: %w", err)
	}

	req, err := http.NewRequest("PUT", uri, bytes.NewReader(requestBody))
	if err != nil {
		return QuotaSetResponse{}, fmt.Errorf("failed to create request to set subscription quota: %w", err)
	}
	c.addMutationHeaders(req)

	resp, err := c.httpClient(60 * time.Second).Do(req)
	if err != nil {
		return QuotaSetResponse{}, fmt.Errorf("failed to send request to set subscription quota: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return QuotaSetResponse{}, fmt.Errorf("failed to read response body after setting subscription quota: %w", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return QuotaSetResponse{}, &StatusError{Operation: "failed to set subscription quota", StatusCode: resp.StatusCode, Status: resp.Status, Body: string(respBody)}
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
		return QuotaSetResponse{}, fmt.Errorf("failed to parse response body after setting subscription quota: %w", err)
	}
	if setEnv.Data.Name == "" {
		return QuotaSetResponse{}, fmt.Errorf("quota set response missing data.name")
	}
	return QuotaSetResponse{
		Name:              setEnv.Data.Name,
		ProvisioningState: setEnv.Data.Properties.ProvisioningState,
	}, nil
}

func (c *Client) QuotaRequest(subscriptionID, providerNs, location, requestID string) (QuotaReadResponse, error) {
	uri := quotaRequestURL(c.Endpoint, subscriptionID, providerNs, location, requestID)

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return QuotaReadResponse{}, fmt.Errorf("failed to create request to get subscription quota: %w", err)
	}
	c.addHeaders(req)

	resp, err := c.httpClient(30 * time.Second).Do(req)
	if err != nil {
		return QuotaReadResponse{}, fmt.Errorf("failed to send request to get subscription quota: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return QuotaReadResponse{}, fmt.Errorf("failed to read response body after getting subscription quota: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return QuotaReadResponse{}, &StatusError{Operation: "failed to get subscription quota", StatusCode: resp.StatusCode, Status: resp.Status, Body: string(respBody)}
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
		return QuotaReadResponse{}, fmt.Errorf("failed to parse response body after getting subscription quota: %w", err)
	}

	out := QuotaReadResponse{
		Name:              readEnv.Data.Name,
		ProvisioningState: readEnv.Data.Properties.ProvisioningState,
	}
	if len(readEnv.Data.Properties.Value) > 0 {
		entry := readEnv.Data.Properties.Value[0]
		out.Limit = entry.Limit.Value
		out.ResourceName = entry.Name.Value
	}
	return out, nil
}
