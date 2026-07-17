package marketplace

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const usersPageSize = 100

type MarketplaceUser struct {
	ID    string
	Email string
}

func (c *Client) Users() ([]MarketplaceUser, error) {
	var users []MarketplaceUser
	for page := 0; ; page++ {
		result, err := c.usersPage(page, usersPageSize)
		if err != nil {
			return nil, err
		}
		users = append(users, result.Users...)
		if result.TotalPages <= 0 || page+1 >= result.TotalPages {
			return users, nil
		}
	}
}

type usersPageResult struct {
	Users      []MarketplaceUser
	TotalPages int
}

func (c *Client) usersPage(page, size int) (usersPageResult, error) {
	uri := fmt.Sprintf("%s/v1/users", c.Endpoint)
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return usersPageResult{}, fmt.Errorf("failed to build request for marketplace users: %w", err)
	}
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("size", strconv.Itoa(size))
	req.URL.RawQuery = q.Encode()
	c.addHeaders(req)

	resp, err := c.httpClient(30 * time.Second).Do(req)
	if err != nil {
		return usersPageResult{}, fmt.Errorf("failed to get marketplace users: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return usersPageResult{}, fmt.Errorf("failed to read marketplace users response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return usersPageResult{}, &StatusError{
			Operation: "failed to get marketplace users", StatusCode: resp.StatusCode,
			Status: resp.Status, Body: string(body),
		}
	}
	result, err := usersResponse(body)
	if err != nil {
		return usersPageResult{}, fmt.Errorf("failed to parse marketplace users response: %w", err)
	}
	return result, nil
}

func (c *Client) UserIDByEmail(email string) (string, error) {
	users, err := c.Users()
	if err != nil {
		return "", err
	}
	var matches []MarketplaceUser
	for _, user := range users {
		if strings.EqualFold(strings.TrimSpace(user.Email), strings.TrimSpace(email)) {
			matches = append(matches, user)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("User not found in CANCOM Marketplace. Contact your Enterprise Administrator")
	case 1:
		if matches[0].ID == "" {
			return "", fmt.Errorf("marketplace user %q has no id", email)
		}
		return matches[0].ID, nil
	default:
		return "", fmt.Errorf("User is ambigous in CANCOM Marketplace. Contact your Enterprise Administrator")
	}
}

func usersResponse(body []byte) (usersPageResult, error) {
	var raw interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return usersPageResult{}, err
	}
	totalPages := 0
	if obj, ok := raw.(map[string]interface{}); ok {
		if page, ok := obj["page"].(map[string]interface{}); ok {
			totalPages = intNumber(page["totalPages"])
		}
		if content, ok := obj["content"]; ok {
			raw = content
		} else if data, ok := obj["data"]; ok {
			raw = data
		}
	}
	items, ok := raw.([]interface{})
	if !ok {
		return usersPageResult{}, fmt.Errorf("expected users response to be an array, content array, or data array")
	}
	users := make([]MarketplaceUser, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		users = append(users, MarketplaceUser{
			ID:    firstString(obj, "id", "uuid", "userUUID"),
			Email: firstString(obj, "email", "emailAddress", "mail"),
		})
	}
	if totalPages == 0 {
		totalPages = 1
	}
	return usersPageResult{Users: users, TotalPages: totalPages}, nil
}

func firstString(obj map[string]interface{}, names ...string) string {
	for _, name := range names {
		if value, ok := obj[name].(string); ok {
			return value
		}
	}
	return ""
}

func intNumber(value interface{}) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		i, _ := strconv.Atoi(string(v))
		return i
	default:
		return 0
	}
}
