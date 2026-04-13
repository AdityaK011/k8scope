package auth

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestCreateSession(t *testing.T) {
	store := NewStore()

	id := store.CreateSession(Session{
		Email:        "user@example.com",
		AccessToken:  "access-123",
		RefreshToken: "refresh-456",
		ExpiresAt:    time.Now().Add(time.Hour),
	})

	// Session ID must be 64-char hex (32 random bytes).
	if len(id) != 64 {
		t.Fatalf("expected session ID of length 64, got %d", len(id))
	}
	if _, err := hex.DecodeString(id); err != nil {
		t.Fatalf("session ID is not valid hex: %v", err)
	}

	// Retrieve and verify fields.
	sess, err := store.GetSession(id)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if sess.Email != "user@example.com" {
		t.Errorf("Email = %q, want %q", sess.Email, "user@example.com")
	}
	if sess.AccessToken != "access-123" {
		t.Errorf("AccessToken = %q, want %q", sess.AccessToken, "access-123")
	}
	if sess.RefreshToken != "refresh-456" {
		t.Errorf("RefreshToken = %q, want %q", sess.RefreshToken, "refresh-456")
	}
	if sess.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set by CreateSession")
	}
}

func TestGetSessionNotFound(t *testing.T) {
	store := NewStore()

	_, err := store.GetSession("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent session, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to contain 'not found'", err.Error())
	}
}

func TestGetSessionExpiry(t *testing.T) {
	store := NewStore()

	id := store.CreateSession(Session{
		Email:       "expired@example.com",
		AccessToken: "token",
	})

	// Manually set CreatedAt to 25 hours ago to force expiry.
	store.mu.Lock()
	store.sessions[id].CreatedAt = time.Now().Add(-25 * time.Hour)
	store.mu.Unlock()

	_, err := store.GetSession(id)
	if err == nil {
		t.Fatal("expected error for expired session, got nil")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("error = %q, want it to contain 'expired'", err.Error())
	}
}

func TestPendingAuthOneTimeUse(t *testing.T) {
	store := NewStore()

	store.StorePending("key1", PendingAuth{
		ClientID:          "client-1",
		CodeChallenge:     "challenge",
		ClientRedirectURI: "http://127.0.0.1:3000/callback",
		State:             "state-abc",
	})

	// First retrieval should succeed.
	p, err := store.GetPending("key1")
	if err != nil {
		t.Fatalf("first GetPending failed: %v", err)
	}
	if p.ClientID != "client-1" {
		t.Errorf("ClientID = %q, want %q", p.ClientID, "client-1")
	}
	if p.State != "state-abc" {
		t.Errorf("State = %q, want %q", p.State, "state-abc")
	}

	// Second retrieval should fail (one-time use).
	_, err = store.GetPending("key1")
	if err == nil {
		t.Fatal("expected error on second GetPending, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to contain 'not found'", err.Error())
	}
}

func TestPendingAuthExpiry(t *testing.T) {
	store := NewStore()

	store.StorePending("key-expired", PendingAuth{
		ClientID: "client-1",
		State:    "state-xyz",
	})

	// Manually set CreatedAt to 6 minutes ago.
	store.pendingMu.Lock()
	store.pending["key-expired"].CreatedAt = time.Now().Add(-6 * time.Minute)
	store.pendingMu.Unlock()

	_, err := store.GetPending("key-expired")
	if err == nil {
		t.Fatal("expected error for expired pending auth, got nil")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("error = %q, want it to contain 'expired'", err.Error())
	}
}

func TestAuthCodeOneTimeUse(t *testing.T) {
	store := NewStore()

	store.StoreAuthCode("code1", AuthCode{
		ClientID:      "client-1",
		SessionID:     "session-abc",
		CodeChallenge: "challenge",
	})

	// First retrieval should succeed.
	ac, err := store.GetAuthCode("code1")
	if err != nil {
		t.Fatalf("first GetAuthCode failed: %v", err)
	}
	if ac.ClientID != "client-1" {
		t.Errorf("ClientID = %q, want %q", ac.ClientID, "client-1")
	}
	if ac.SessionID != "session-abc" {
		t.Errorf("SessionID = %q, want %q", ac.SessionID, "session-abc")
	}

	// Second retrieval should fail (one-time use).
	_, err = store.GetAuthCode("code1")
	if err == nil {
		t.Fatal("expected error on second GetAuthCode, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to contain 'not found'", err.Error())
	}
}

func TestAuthCodeExpiry(t *testing.T) {
	store := NewStore()

	store.StoreAuthCode("code-expired", AuthCode{
		ClientID:  "client-1",
		SessionID: "session-abc",
	})

	// Manually set CreatedAt to 6 minutes ago.
	store.codesMu.Lock()
	store.codes["code-expired"].CreatedAt = time.Now().Add(-6 * time.Minute)
	store.codesMu.Unlock()

	_, err := store.GetAuthCode("code-expired")
	if err == nil {
		t.Fatal("expected error for expired auth code, got nil")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("error = %q, want it to contain 'expired'", err.Error())
	}
}

func TestRegisterClient(t *testing.T) {
	store := NewStore()

	clientID := store.RegisterClient(Client{
		ClientName:              "Test App",
		RedirectURIs:            []string{"http://127.0.0.1:3000/callback"},
		GrantTypes:              []string{"authorization_code"},
		TokenEndpointAuthMethod: "none",
	})

	if len(clientID) != 32 { // 16 random bytes = 32 hex chars
		t.Fatalf("expected client ID of length 32, got %d", len(clientID))
	}

	client, err := store.GetClient(clientID)
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}
	if client.ClientName != "Test App" {
		t.Errorf("ClientName = %q, want %q", client.ClientName, "Test App")
	}
	if len(client.RedirectURIs) != 1 || client.RedirectURIs[0] != "http://127.0.0.1:3000/callback" {
		t.Errorf("RedirectURIs = %v, want [http://127.0.0.1:3000/callback]", client.RedirectURIs)
	}
	if client.TokenEndpointAuthMethod != "none" {
		t.Errorf("TokenEndpointAuthMethod = %q, want %q", client.TokenEndpointAuthMethod, "none")
	}
	if client.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set by RegisterClient")
	}
}

func TestGetClientNotFound(t *testing.T) {
	store := NewStore()

	_, err := store.GetClient("nonexistent-client-id")
	if err == nil {
		t.Fatal("expected error for nonexistent client, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to contain 'not found'", err.Error())
	}
}

func TestRandomHex(t *testing.T) {
	result := randomHex(32)

	// 32 bytes encoded as hex = 64 characters.
	if len(result) != 64 {
		t.Fatalf("expected length 64, got %d", len(result))
	}

	// All characters must be valid hex.
	if _, err := hex.DecodeString(result); err != nil {
		t.Fatalf("result is not valid hex: %v", err)
	}

	// Two calls should produce different values (probabilistically).
	result2 := randomHex(32)
	if result == result2 {
		t.Error("two calls to randomHex returned identical values, which is extremely unlikely")
	}
}
