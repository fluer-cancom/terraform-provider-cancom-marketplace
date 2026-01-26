package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func subscriptionStatusSuccess(requestId string, m interface{}) (bool, error) {
	// Use requestId to check status of the subscription creation
	uri_status := fmt.Sprintf("%s/azure-api-gateway/v1/subscriptionStatus?requestId=%s", m.(map[string]interface{})["endpoint"].(string), requestId)
	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}
	req_status, err := http.NewRequest("GET", uri_status, nil)
	if err != nil {
		return false, err
	}
	req_status.Header.Set("Content-Type", "application/json")
	req_status.Header.Set("Authorization", fmt.Sprintf("Basic %s", m.(map[string]interface{})["api_username"].(string)+":"+m.(map[string]interface{})["api_password"].(string)))
	resp_status, err := httpClient.Do(req_status)
	if err != nil {
		return false, err
	}
	defer resp_status.Body.Close()
	if resp_status.StatusCode != http.StatusAccepted {
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
		return false, nil
	}

	return true, nil
}

func subscriptionInfo(requestId string, m interface{}) (map[string]interface{}, error) {
	// Use requestId to get info of the subscription creation
	uri_info := fmt.Sprintf("%s/azure-api-gateway/v1/subscriptionStatus?requestId=%s", m.(map[string]interface{})["endpoint"].(string), requestId)
	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}
	req_info, err := http.NewRequest("GET", uri_info, nil)
	if err != nil {
		return nil, err
	}
	req_info.Header.Set("Content-Type", "application/json")
	req_info.Header.Set("Authorization", fmt.Sprintf("Basic %s", m.(map[string]interface{})["api_username"].(string)+":"+m.(map[string]interface{})["api_password"].(string)))
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
