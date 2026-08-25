package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthenticatorUsesLegacyTenantResultByDefault(t *testing.T) {
	statusCode, tenants := evaluatePolicy(t, legacyPolicy, nil)

	if statusCode != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", statusCode)
	}

	if len(tenants) != 1 || tenants[0] != "tenant-a" {
		t.Fatalf("expected tenant-a, got %+v", tenants)
	}
}

func TestAuthenticatorUsesAccessObjectWhenEnabled(t *testing.T) {
	statusCode, tenants := evaluatePolicy(t, accessObjectPolicy, []Option{WithAccessObjectAuthorization(true)})

	if statusCode != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", statusCode)
	}

	if len(tenants) != 1 || tenants[0] != "tenant-a" {
		t.Fatalf("expected only tenant-a, got %+v", tenants)
	}
}

func evaluatePolicy(t *testing.T, policy string, opts []Option) (int, []string) {
	t.Helper()

	const readDevices Scope = "devices.read"

	authz, err := NewAuthenticator(t.Context(), strings.NewReader(policy), opts...)
	if err != nil {
		t.Fatalf("failed to create authenticator: %v", err)
	}

	var tenants []string
	handler := authz.RequireAccess(readDevices)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenants = GetTenantsWithAllowedScopes(r.Context(), readDevices)
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v0/devices", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec.Code, tenants
}

const legacyPolicy = `
package example.authz

default allow := false

allow = response if {
	input.method == "GET"
	input.path == ["api", "v0", "devices"]

	response := {
		"tenants": ["tenant-a"]
	}
}`

const accessObjectPolicy = `
package example.authz

default allow := false

allow = response if {
	response := {
		"access": {
			"tenant-a": ["devices.read"],
			"tenant-b": ["sensors.read"]
		}
	}
}`
