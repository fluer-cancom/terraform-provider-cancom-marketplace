package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func subscriptionStatusSuccess(requestId string, m *Config) (bool, error) {
	// Use requestId to check status of the subscription creation
	uri_status := fmt.Sprintf("%s/azure-api-gateway/v1/subscriptionStatus?requestId=%s&country=%s", m.Endpoint, requestId, m.Country)
	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}
	req_status, err := http.NewRequest("GET", uri_status, nil)
	if err != nil {
		return false, err
	}
	req_status.Header.Set("Content-Type", "application/json")
	req_status.Header.Set("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(m.Username+":"+m.Password))))
	resp_status, err := httpClient.Do(req_status)
	if err != nil {
		return false, err
	}
	defer resp_status.Body.Close()
	if resp_status.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to get Azure subscription status: %s", resp_status.Status)
	}
	body, err := io.ReadAll(resp_status.Body)
	if err != nil {
		return false, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, err
	}

	if result["status"].(string) != "successfull" {
		if result["status"].(string) == "pending" {
			return false, nil
		}
		return false, fmt.Errorf("failed to create Azure subscription: %s", result["message"].(string))
	}

	return true, nil
}

func subscriptionInfo(requestId string, m *Config) (map[string]interface{}, error) {
	// Use requestId to get info of the subscription creation
	uri_info := fmt.Sprintf("%s/azure-api-gateway/v1/subscriptionStatus?requestId=%s&country=%s", m.Endpoint, requestId, m.Country)
	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}
	req_info, err := http.NewRequest("GET", uri_info, nil)
	if err != nil {
		return nil, err
	}
	req_info.Header.Set("Content-Type", "application/json")
	req_info.Header.Set("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(m.Username+":"+m.Password))))
	resp_info, err := httpClient.Do(req_info)
	if err != nil {
		return nil, err
	}
	defer resp_info.Body.Close()
	if resp_info.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("failed to get Azure subscription info: %s", resp_info.Status)
	}
	body, err := io.ReadAll(resp_info.Body)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}
