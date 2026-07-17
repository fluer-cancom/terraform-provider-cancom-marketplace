package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/google/uuid"
)

type Operation string

const (
	OperationReadDefaultManagementGroup Operation = "Microsoft.Management/managementGroups/settings/read"
	OperationRenameSubscription         Operation = "Microsoft.Subscription/subscriptions/rename/action"
	OperationAssignOwnerRole            Operation = "Microsoft.Authorization/roleAssignments/write"
	OperationCancelSubscription         Operation = "Microsoft.Subscription/subscriptions/cancel/action"
)

const (
	defaultManagementEndpoint = "https://management.azure.com"
	ownerRoleDefinitionID     = "8e3af657-a8ff-443c-a75c-2fe8c4bcb635"
)

type Client struct {
	Credential         azcore.TokenCredential
	HTTPClient         *http.Client
	ManagementEndpoint string
	Preflight          func(context.Context, []Operation) error
	Rename             func(context.Context, string, string) error
	DisplayName        func(context.Context, string) (string, error)
	AssignOwner        func(context.Context, string, string) error
	Cancel             func(context.Context, string) error
}

func NewClientSecretCredential(tenantID, clientID, clientSecret string) (azcore.TokenCredential, error) {
	return azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
}

func NewDefaultCredential() (azcore.TokenCredential, error) {
	return azidentity.NewDefaultAzureCredential(nil)
}

func (c *Client) EnsureReady(ctx context.Context, operations []Operation) error {
	if len(operations) == 0 {
		return nil
	}
	if c == nil || c.Credential == nil {
		return missingAuthError(operations)
	}
	if c.Preflight != nil {
		if err := c.Preflight(ctx, operations); err != nil {
			return preflightError(operations, err)
		}
		return nil
	}
	if _, err := c.token(ctx); err != nil {
		return preflightError(operations, err)
	}
	if requiresDefaultManagementGroupPreflight(operations) {
		if _, err := c.DefaultManagementGroup(ctx); err != nil {
			return preflightError(operations, err)
		}
	}
	return nil
}

func (c *Client) DefaultManagementGroup(ctx context.Context) (string, error) {
	var result struct {
		Properties struct {
			DefaultManagementGroup string `json:"defaultManagementGroup"`
		} `json:"properties"`
	}
	if err := c.do(ctx, http.MethodGet, "/providers/Microsoft.Management/managementGroups/root/settings/default?api-version=2020-05-01", nil, &result, http.StatusOK); err != nil {
		return "", err
	}
	if result.Properties.DefaultManagementGroup == "" {
		return "", fmt.Errorf("default management group response missing properties.defaultManagementGroup")
	}
	return result.Properties.DefaultManagementGroup, nil
}

func (c *Client) AssignOwnerRole(ctx context.Context, subscriptionID, principalObjectID string) error {
	if c.AssignOwner != nil {
		return c.AssignOwner(ctx, subscriptionID, principalObjectID)
	}
	if subscriptionID == "" {
		return fmt.Errorf("cannot assign Owner role: Azure subscription ID is empty")
	}
	if principalObjectID == "" {
		return fmt.Errorf("cannot assign Owner role: principal object ID is empty")
	}

	roleAssignmentID := deterministicRoleAssignmentID(subscriptionID, principalObjectID)
	scope := "/subscriptions/" + url.PathEscape(subscriptionID)
	path := fmt.Sprintf(
		"%s/providers/Microsoft.Authorization/roleAssignments/%s?api-version=2022-04-01",
		scope,
		url.PathEscape(roleAssignmentID),
	)
	body := map[string]interface{}{
		"properties": map[string]interface{}{
			"roleDefinitionId": fmt.Sprintf("%s/providers/Microsoft.Authorization/roleDefinitions/%s", scope, ownerRoleDefinitionID),
			"principalId":      principalObjectID,
		},
	}
	if err := c.do(ctx, http.MethodPut, path, body, nil, http.StatusOK, http.StatusCreated); err != nil {
		return fmt.Errorf("failed to assign Owner role on subscription %s to principal %s: %w", subscriptionID, principalObjectID, err)
	}
	return nil
}

func (c *Client) RenameSubscription(ctx context.Context, subscriptionID, displayName string) error {
	if c.Rename != nil {
		return c.Rename(ctx, subscriptionID, displayName)
	}
	if c == nil || c.Credential == nil {
		return fmt.Errorf("cannot authenticate with Azure API. To rename subscription, run 'az login' or set azure_client_id/azure_client_secret/azure_tenant_id")
	}
	if subscriptionID == "" {
		return fmt.Errorf("cannot rename subscription: Azure subscription ID is empty")
	}
	if displayName == "" {
		return nil
	}
	clientFactory, err := armsubscription.NewClientFactory(c.Credential, nil)
	if err != nil {
		return err
	}
	if _, err := clientFactory.NewClient().Rename(ctx, subscriptionID, armsubscription.Name{SubscriptionName: &displayName}, nil); err != nil {
		return err
	}
	return nil
}

func (c *Client) SubscriptionDisplayName(ctx context.Context, subscriptionID string) (string, error) {
	if c.DisplayName != nil {
		return c.DisplayName(ctx, subscriptionID)
	}
	if c == nil || c.Credential == nil {
		return "", fmt.Errorf("cannot authenticate with Azure API. To read subscription display name, run 'az login' or set azure_client_id/azure_client_secret/azure_tenant_id")
	}
	if subscriptionID == "" {
		return "", fmt.Errorf("cannot read subscription display name: Azure subscription ID is empty")
	}
	client, err := armsubscription.NewSubscriptionsClient(c.Credential, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Get(ctx, subscriptionID, nil)
	if err != nil {
		return "", err
	}
	if resp.DisplayName == nil {
		return "", nil
	}
	return *resp.DisplayName, nil
}

func (c *Client) CancelSubscription(ctx context.Context, subscriptionID string) error {
	if c.Cancel != nil {
		return c.Cancel(ctx, subscriptionID)
	}
	if c == nil || c.Credential == nil {
		return fmt.Errorf("cannot authenticate with Azure API. To cancel subscription, run 'az login' or set azure_client_id/azure_client_secret/azure_tenant_id")
	}
	clientFactory, err := armsubscription.NewClientFactory(c.Credential, nil)
	if err != nil {
		return err
	}
	if _, err := clientFactory.NewClient().Cancel(ctx, subscriptionID, nil); err != nil {
		return err
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, body interface{}, out interface{}, okStatuses ...int) error {
	if c == nil || c.Credential == nil {
		return missingAuthError(nil)
	}
	var reader io.Reader
	if body != nil {
		requestBody, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(requestBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.managementEndpoint(), "/")+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	tok, err := c.token(ctx)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok.Token)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	for _, status := range okStatuses {
		if resp.StatusCode == status {
			if out != nil && len(respBody) > 0 {
				if err := json.Unmarshal(respBody, out); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return fmt.Errorf("%s %s returned %s: %s", method, req.URL.Path, resp.Status, string(respBody))
}

func (c *Client) token(ctx context.Context) (azcore.AccessToken, error) {
	return c.Credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{strings.TrimRight(c.managementEndpoint(), "/") + "/.default"},
	})
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *Client) managementEndpoint() string {
	if c.ManagementEndpoint != "" {
		return c.ManagementEndpoint
	}
	return defaultManagementEndpoint
}

func missingAuthError(operations []Operation) error {
	if len(operations) == 0 {
		return fmt.Errorf("cannot authenticate with Azure API. Run 'az login' or set azure_client_id/azure_client_secret/azure_tenant_id in the provider")
	}
	return fmt.Errorf("Azure authentication is required for %s. Run 'az login', set azure_client_id/azure_client_secret/azure_tenant_id in the provider, or remove the Azure-backed property from the resource configuration", operationList(operations))
}

func preflightError(operations []Operation, err error) error {
	return fmt.Errorf("Azure preflight failed for %s: %w. Run 'az login', set azure_client_id/azure_client_secret/azure_tenant_id in the provider, ensure the identity has the required Azure permissions, or remove the Azure-backed property from the resource configuration", operationList(operations), err)
}

func requiresDefaultManagementGroupPreflight(operations []Operation) bool {
	for _, operation := range operations {
		switch operation {
		case OperationReadDefaultManagementGroup, OperationRenameSubscription, OperationAssignOwnerRole, OperationCancelSubscription:
			return true
		}
	}
	return false
}

func deterministicRoleAssignmentID(subscriptionID, principalObjectID string) string {
	// UUID v5 with a fixed namespace keeps retries idempotent for the same
	// subscription/principal/role tuple without storing a separate assignment ID.
	return strings.ToLower(uuid.NewSHA1(uuid.NameSpaceOID, []byte(subscriptionID+"/"+principalObjectID+"/"+ownerRoleDefinitionID)).String())
}

func operationList(operations []Operation) string {
	if len(operations) == 1 {
		return string(operations[0])
	}
	out := ""
	for i, operation := range operations {
		if i > 0 {
			out += ", "
		}
		out += string(operation)
	}
	return out
}
