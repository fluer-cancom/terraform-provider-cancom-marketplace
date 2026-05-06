package main

import (
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// schemaResourceDataFromRaw builds a ResourceData populated from a raw attribute
// map. Used to drive ConfigureFunc / CRUD funcs in tests without standing up a
// full Terraform run.
func schemaResourceDataFromRaw(t *testing.T, s map[string]*schema.Schema, raw map[string]interface{}) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, s, raw)
}

// newTestConfig wires a *Config to the given httptest server.
func newTestConfig(srv *httptest.Server) *Config {
	return &Config{
		Endpoint:     srv.URL,
		CCMPApiToken: "test-token",
		HTTPClient:   srv.Client(),
	}
}
