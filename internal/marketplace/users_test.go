package marketplace

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUserIDByEmailFindsSingleUser(t *testing.T) {
	var pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/users" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		pages = append(pages, r.URL.Query().Get("page"))
		if r.URL.Query().Get("size") != "100" {
			t.Fatalf("size = %q", r.URL.Query().Get("size"))
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q", got)
		}
		switch r.URL.Query().Get("page") {
		case "0":
			w.Write([]byte(`{"content":[{"uuid":"other","email":"other@example.com"}],"page":{"size":100,"totalElements":2,"totalPages":2,"number":0}}`))
		case "1":
			w.Write([]byte(`{"content":[{"uuid":"user-1","email":"USER@example.com"}],"page":{"size":100,"totalElements":2,"totalPages":2,"number":1}}`))
		default:
			t.Fatalf("unexpected page = %s", r.URL.Query().Get("page"))
		}
	}))
	defer srv.Close()

	client := &Client{Endpoint: srv.URL, Token: "test-token", HTTPClient: srv.Client()}
	id, err := client.UserIDByEmail("user@example.com")
	if err != nil {
		t.Fatalf("UserIDByEmail: %v", err)
	}
	if id != "user-1" {
		t.Fatalf("id = %q", id)
	}
	if strings.Join(pages, ",") != "0,1" {
		t.Fatalf("pages = %v", pages)
	}
}

func TestUserIDByEmailNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"content":[{"uuid":"user-1","email":"other@example.com"}],"page":{"totalPages":1}}`))
	}))
	defer srv.Close()

	client := &Client{Endpoint: srv.URL, Token: "test-token", HTTPClient: srv.Client()}
	_, err := client.UserIDByEmail("user@example.com")
	if err == nil || !strings.Contains(err.Error(), "User not found in CANCOM Marketplace. Contact your Enterprise Administrator") {
		t.Fatalf("expected user not found error, got %v", err)
	}
}

func TestUserIDByEmailAmbiguous(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"content":[{"uuid":"user-1","email":"user@example.com"},{"uuid":"user-2","email":"USER@example.com"}],"page":{"totalPages":1}}`))
	}))
	defer srv.Close()

	client := &Client{Endpoint: srv.URL, Token: "test-token", HTTPClient: srv.Client()}
	_, err := client.UserIDByEmail("user@example.com")
	if err == nil || !strings.Contains(err.Error(), "User is ambigous in CANCOM Marketplace. Contact your Enterprise Administrator") {
		t.Fatalf("expected ambiguous user error, got %v", err)
	}
}
