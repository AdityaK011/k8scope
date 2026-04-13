package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestGoogleOAuth() *GoogleOAuth {
	return NewGoogleOAuth("test-id", "test-secret", "http://localhost:8080/callback")
}

func TestHandleMetadata(t *testing.T) {
	g := newTestGoogleOAuth()

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()

	g.handleMetadata(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	requiredFields := []string{
		"issuer",
		"authorization_endpoint",
		"token_endpoint",
		"registration_endpoint",
		"response_types_supported",
		"grant_types_supported",
		"code_challenge_methods_supported",
		"token_endpoint_auth_methods_supported",
	}
	for _, field := range requiredFields {
		if _, ok := body[field]; !ok {
			t.Errorf("response missing required field %q", field)
		}
	}

	// Verify endpoints contain the host.
	if authEP, ok := body["authorization_endpoint"].(string); ok {
		if !strings.HasSuffix(authEP, "/authorize") {
			t.Errorf("authorization_endpoint = %q, want it to end with /authorize", authEP)
		}
	}
	if tokenEP, ok := body["token_endpoint"].(string); ok {
		if !strings.HasSuffix(tokenEP, "/token") {
			t.Errorf("token_endpoint = %q, want it to end with /token", tokenEP)
		}
	}
	if regEP, ok := body["registration_endpoint"].(string); ok {
		if !strings.HasSuffix(regEP, "/register") {
			t.Errorf("registration_endpoint = %q, want it to end with /register", regEP)
		}
	}
}

func TestHandleRegister(t *testing.T) {
	g := newTestGoogleOAuth()

	body := `{
		"client_name": "Test Client",
		"redirect_uris": ["http://127.0.0.1:3000/callback"],
		"grant_types": ["authorization_code"],
		"token_endpoint_auth_method": "none"
	}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	g.handleRegister(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	clientID, ok := resp["client_id"].(string)
	if !ok || clientID == "" {
		t.Error("response missing or empty client_id")
	}

	if name, _ := resp["client_name"].(string); name != "Test Client" {
		t.Errorf("client_name = %q, want %q", name, "Test Client")
	}
}

func TestHandleRegisterInvalidRedirectURI(t *testing.T) {
	g := newTestGoogleOAuth()

	body := `{
		"client_name": "Evil Client",
		"redirect_uris": ["https://evil.com/callback"],
		"grant_types": ["authorization_code"],
		"token_endpoint_auth_method": "none"
	}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	g.handleRegister(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "invalid_redirect_uri") {
		t.Errorf("response body = %q, want it to contain 'invalid_redirect_uri'", rec.Body.String())
	}
}

func TestHandleRegisterMissingRedirectURIs(t *testing.T) {
	g := newTestGoogleOAuth()

	body := `{
		"client_name": "No Redirect Client",
		"redirect_uris": [],
		"grant_types": ["authorization_code"]
	}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	g.handleRegister(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "invalid_redirect_uri") {
		t.Errorf("response body = %q, want it to contain 'invalid_redirect_uri'", rec.Body.String())
	}
}

func TestIsLoopbackURI(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want bool
	}{
		{
			name: "IPv4 loopback",
			uri:  "http://127.0.0.1:3000/callback",
			want: true,
		},
		{
			name: "IPv6 loopback",
			uri:  "http://[::1]:3000/callback",
			want: true,
		},
		{
			name: "localhost",
			uri:  "http://localhost:3000/callback",
			want: true,
		},
		{
			name: "external host",
			uri:  "http://evil.com/callback",
			want: false,
		},
		{
			name: "empty string",
			uri:  "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLoopbackURI(tt.uri)
			if got != tt.want {
				t.Errorf("isLoopbackURI(%q) = %v, want %v", tt.uri, got, tt.want)
			}
		})
	}
}

func TestMatchesRegisteredURI(t *testing.T) {
	tests := []struct {
		name           string
		requestURI     string
		registeredURIs []string
		want           bool
	}{
		{
			name:           "same scheme+host+path, different port",
			requestURI:     "http://127.0.0.1:9999/callback",
			registeredURIs: []string{"http://127.0.0.1:3000/callback"},
			want:           true,
		},
		{
			name:           "different host",
			requestURI:     "http://192.168.1.1:3000/callback",
			registeredURIs: []string{"http://127.0.0.1:3000/callback"},
			want:           false,
		},
		{
			name:           "different scheme",
			requestURI:     "https://127.0.0.1:3000/callback",
			registeredURIs: []string{"http://127.0.0.1:3000/callback"},
			want:           false,
		},
		{
			name:           "exact match",
			requestURI:     "http://127.0.0.1:3000/callback",
			registeredURIs: []string{"http://127.0.0.1:3000/callback"},
			want:           true,
		},
		{
			name:           "different path",
			requestURI:     "http://127.0.0.1:3000/other",
			registeredURIs: []string{"http://127.0.0.1:3000/callback"},
			want:           false,
		},
		{
			name:           "no registered URIs",
			requestURI:     "http://127.0.0.1:3000/callback",
			registeredURIs: []string{},
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesRegisteredURI(tt.requestURI, tt.registeredURIs)
			if got != tt.want {
				t.Errorf("matchesRegisteredURI(%q, %v) = %v, want %v",
					tt.requestURI, tt.registeredURIs, got, tt.want)
			}
		})
	}
}
