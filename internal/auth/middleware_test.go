package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMiddlewareNoToken(t *testing.T) {
	g := newTestGoogleOAuth()

	handler := g.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when no token is provided")
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareInvalidToken(t *testing.T) {
	g := newTestGoogleOAuth()

	handler := g.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with invalid token")
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareValidToken(t *testing.T) {
	g := newTestGoogleOAuth()

	// Create a session with an access token that won't expire soon,
	// so EnsureFreshToken does not attempt a Google refresh.
	sessionID := g.Store.CreateSession(Session{
		Email:        "test@example.com",
		AccessToken:  "google-access-token",
		RefreshToken: "google-refresh-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	})

	var receivedSession *Session
	handler := g.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, err := SessionFromContext(r.Context())
		if err != nil {
			t.Errorf("SessionFromContext failed: %v", err)
			return
		}
		receivedSession = sess
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+sessionID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if receivedSession == nil {
		t.Fatal("handler did not receive session in context")
	}
	if receivedSession.Email != "test@example.com" {
		t.Errorf("session Email = %q, want %q", receivedSession.Email, "test@example.com")
	}
	if receivedSession.AccessToken != "google-access-token" {
		t.Errorf("session AccessToken = %q, want %q", receivedSession.AccessToken, "google-access-token")
	}
}

func TestSessionFromContextWithSession(t *testing.T) {
	sess := &Session{
		Email:        "ctx@example.com",
		AccessToken:  "tok-abc",
		RefreshToken: "ref-xyz",
		ExpiresAt:    time.Now().Add(time.Hour),
		CreatedAt:    time.Now(),
	}

	ctx := context.WithValue(context.Background(), sessionCtxKey, sess)

	got, err := SessionFromContext(ctx)
	if err != nil {
		t.Fatalf("SessionFromContext failed: %v", err)
	}
	if got.Email != "ctx@example.com" {
		t.Errorf("Email = %q, want %q", got.Email, "ctx@example.com")
	}
	if got.AccessToken != "tok-abc" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "tok-abc")
	}
}

func TestSessionFromContextWithoutSession(t *testing.T) {
	ctx := context.Background()

	_, err := SessionFromContext(ctx)
	if err == nil {
		t.Fatal("expected error from empty context, got nil")
	}
}

func TestExtractBearer(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "valid bearer token",
			header: "Bearer abc123",
			want:   "abc123",
		},
		{
			name:   "basic auth scheme",
			header: "Basic xyz",
			want:   "",
		},
		{
			name:   "empty header",
			header: "",
			want:   "",
		},
		{
			name:   "bearer with long token",
			header: "Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9",
			want:   "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9",
		},
		{
			name:   "lowercase bearer (invalid)",
			header: "bearer abc123",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			got := extractBearer(req)
			if got != tt.want {
				t.Errorf("extractBearer() = %q, want %q", got, tt.want)
			}
		})
	}
}
