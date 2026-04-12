package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/sync/singleflight"
)

type GoogleOAuth struct {
	config       *oauth2.Config
	Store        *Store
	refreshGroup singleflight.Group
	// TrustProxy enables reading X-Forwarded-Proto from reverse proxies.
	// Only set to true when running behind a trusted LB/ingress.
	TrustProxy bool
}

func NewGoogleOAuth(clientID, clientSecret, redirectURL string) *GoogleOAuth {
	return &GoogleOAuth{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL, // YOUR server's /callback
			Scopes: []string{
				"https://www.googleapis.com/auth/cloud-platform",
			},
			Endpoint: google.Endpoint,
		},
		Store: NewStore(),
	}
}

// Register mounts OAuth endpoints on the given mux with rate limiting.
func (g *GoogleOAuth) Register(mux *http.ServeMux, rl func(http.Handler) http.Handler) {
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", g.handleMetadata)
	mux.Handle("POST /register", rl(http.HandlerFunc(g.handleRegister)))
	mux.Handle("GET /authorize", rl(http.HandlerFunc(g.handleAuthorize)))
	mux.Handle("GET /callback", rl(http.HandlerFunc(g.handleCallback)))
	mux.Handle("POST /token", rl(http.HandlerFunc(g.handleToken)))
}

// handleMetadata tells Claude Code where to send users for login.
func (g *GoogleOAuth) handleMetadata(w http.ResponseWriter, r *http.Request) {
	// Only trust X-Forwarded-Proto when running behind a trusted proxy.
	scheme := ""
	if g.TrustProxy {
		if proto := r.Header.Get("X-Forwarded-Proto"); proto == "https" || proto == "http" {
			scheme = proto
		}
	}
	if scheme == "" {
		scheme = "https"
		if r.TLS == nil {
			scheme = "http"
		}
	}
	base := fmt.Sprintf("%s://%s", scheme, r.Host)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"issuer":                                base,
		"authorization_endpoint":                base + "/authorize",
		"token_endpoint":                        base + "/token",
		"registration_endpoint":                 base + "/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none", "client_secret_post"},
	})
}

// handleRegister implements OAuth 2.0 Dynamic Client Registration (RFC 7591).
// MCP clients call this to obtain a client_id before starting the OAuth flow.
func (g *GoogleOAuth) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientName              string   `json:"client_name"`
		RedirectURIs            []string `json:"redirect_uris"`
		GrantTypes              []string `json:"grant_types"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_client_metadata"}`, http.StatusBadRequest)
		return
	}

	// Validate redirect URIs — all must be loopback per our security policy.
	if len(req.RedirectURIs) == 0 {
		http.Error(w, `{"error":"invalid_redirect_uri","error_description":"at least one redirect_uri required"}`, http.StatusBadRequest)
		return
	}
	for _, uri := range req.RedirectURIs {
		if !isLoopbackURI(uri) {
			http.Error(w, `{"error":"invalid_redirect_uri","error_description":"only loopback redirect URIs allowed"}`, http.StatusBadRequest)
			return
		}
	}

	// Defaults.
	if len(req.GrantTypes) == 0 {
		req.GrantTypes = []string{"authorization_code"}
	}
	if req.TokenEndpointAuthMethod == "" {
		req.TokenEndpointAuthMethod = "none"
	}
	if req.TokenEndpointAuthMethod != "none" && req.TokenEndpointAuthMethod != "client_secret_post" {
		http.Error(w, `{"error":"invalid_client_metadata","error_description":"token_endpoint_auth_method must be none or client_secret_post"}`, http.StatusBadRequest)
		return
	}

	client := Client{
		ClientName:              req.ClientName,
		RedirectURIs:            req.RedirectURIs,
		GrantTypes:              req.GrantTypes,
		TokenEndpointAuthMethod: req.TokenEndpointAuthMethod,
	}

	// Generate client_secret for confidential clients.
	if req.TokenEndpointAuthMethod != "none" {
		client.ClientSecret = randomHex(32)
	}

	clientID := g.Store.RegisterClient(client)

	// RFC 7591 response.
	resp := map[string]interface{}{
		"client_id":                  clientID,
		"client_name":               req.ClientName,
		"redirect_uris":             req.RedirectURIs,
		"grant_types":               req.GrantTypes,
		"token_endpoint_auth_method": req.TokenEndpointAuthMethod,
		"client_id_issued_at":       time.Now().Unix(),
		"client_secret_expires_at":  0,
	}
	if client.ClientSecret != "" {
		resp["client_secret"] = client.ClientSecret
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// handleAuthorize receives the request from Claude Code and redirects to Google.
//
// Claude Code sends:
//   - state (CSRF token)
//   - code_challenge + code_challenge_method (PKCE)
//   - redirect_uri (Claude Code's local loopback listener)
func (g *GoogleOAuth) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	state := r.URL.Query().Get("state")
	codeChallenge := r.URL.Query().Get("code_challenge")
	codeChallengeMethod := r.URL.Query().Get("code_challenge_method")
	clientRedirectURI := r.URL.Query().Get("redirect_uri")

	if clientID == "" || state == "" || codeChallenge == "" || clientRedirectURI == "" {
		http.Error(w, `{"error":"invalid_request","hint":"missing client_id, state, code_challenge, or redirect_uri"}`, http.StatusBadRequest)
		return
	}

	// Validate registered client.
	client, err := g.Store.GetClient(clientID)
	if err != nil {
		http.Error(w, `{"error":"invalid_client"}`, http.StatusBadRequest)
		return
	}

	// Validate redirect_uri matches a registered URI for this client.
	// Per RFC 8252 Section 7.3, loopback URIs must allow any port.
	if !matchesRegisteredURI(clientRedirectURI, client.RedirectURIs) {
		http.Error(w, `{"error":"invalid_redirect_uri","hint":"redirect_uri not registered for this client"}`, http.StatusBadRequest)
		return
	}

	// Only accept S256 PKCE challenges.
	if codeChallengeMethod != "S256" {
		http.Error(w, `{"error":"invalid_request","hint":"code_challenge_method must be S256"}`, http.StatusBadRequest)
		return
	}

	// Generate a key to link Google's callback back to this request.
	pendingKey := randomHex(16)
	g.Store.StorePending(pendingKey, PendingAuth{
		ClientID:          clientID,
		CodeChallenge:     codeChallenge,
		ClientRedirectURI: clientRedirectURI,
		State:             state,
	})

	// Redirect user's browser to Google login.
	// We use pendingKey as Google's state param so we can look up
	// the original Claude Code request when Google calls us back.
	authURL := g.config.AuthCodeURL(pendingKey,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)

	slog.Info("redirecting to Google login", "pending_key", pendingKey)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleCallback receives the redirect from Google after the user logs in.
func (g *GoogleOAuth) handleCallback(w http.ResponseWriter, r *http.Request) {
	googleCode := r.URL.Query().Get("code")
	pendingKey := r.URL.Query().Get("state") // this is our pendingKey, not Claude Code's state

	if googleCode == "" || pendingKey == "" {
		http.Error(w, "missing code or state from Google", http.StatusBadRequest)
		return
	}

	// Look up the original request from Claude Code.
	pending, err := g.Store.GetPending(pendingKey)
	if err != nil {
		http.Error(w, "auth session expired, try again", http.StatusBadRequest)
		return
	}

	// Exchange Google's auth code for tokens.
	token, err := g.config.Exchange(context.Background(), googleCode)
	if err != nil {
		slog.Error("google token exchange failed", "error", err)
		http.Error(w, "failed to exchange code with Google", http.StatusInternalServerError)
		return
	}

	// Optionally fetch the user's email for logging.
	email := fetchEmail(token.AccessToken)

	// Create a persistent session with the user's Google tokens.
	sessionID := g.Store.CreateSession(Session{
		Email:        email,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    token.Expiry,
	})

	slog.Info("session created", "email", email, "session_id", sessionID[:8]+"...")

	// Generate an auth code that Claude Code will exchange for the session.
	ourCode := randomHex(16)
	g.Store.StoreAuthCode(ourCode, AuthCode{
		ClientID:      pending.ClientID,
		SessionID:     sessionID,
		CodeChallenge: pending.CodeChallenge,
	})

	// Redirect back to Claude Code's loopback listener.
	redirectURL := fmt.Sprintf("%s?code=%s&state=%s",
		pending.ClientRedirectURI, ourCode, pending.State)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// handleToken is called by Claude Code to exchange our auth code for a bearer token.
// This is the MCP OAuth token endpoint, not Google's.
func (g *GoogleOAuth) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	grantType := r.FormValue("grant_type")
	clientID := r.FormValue("client_id")
	code := r.FormValue("code")
	codeVerifier := r.FormValue("code_verifier")

	if grantType != "authorization_code" || clientID == "" || code == "" || codeVerifier == "" {
		http.Error(w, `{"error":"invalid_request","hint":"missing grant_type, client_id, code, or code_verifier"}`, http.StatusBadRequest)
		return
	}

	// Validate registered client.
	client, err := g.Store.GetClient(clientID)
	if err != nil {
		http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
		return
	}

	// For confidential clients, validate client_secret.
	if client.TokenEndpointAuthMethod != "none" {
		clientSecret := r.FormValue("client_secret")
		if clientSecret != client.ClientSecret {
			http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
			return
		}
	}

	// Look up our auth code.
	authCode, err := g.Store.GetAuthCode(code)
	if err != nil {
		http.Error(w, `{"error":"invalid_grant","hint":"invalid or expired auth code"}`, http.StatusBadRequest)
		return
	}

	// Verify the client_id matches the one that initiated the authorization.
	if authCode.ClientID != clientID {
		http.Error(w, `{"error":"invalid_grant","hint":"client_id mismatch"}`, http.StatusBadRequest)
		return
	}

	// Verify PKCE: SHA256(code_verifier) must match code_challenge.
	h := sha256.Sum256([]byte(codeVerifier))
	computedChallenge := base64.RawURLEncoding.EncodeToString(h[:])
	if computedChallenge != authCode.CodeChallenge {
		slog.Warn("PKCE verification failed")
		http.Error(w, `{"error":"invalid_grant","hint":"PKCE verification failed"}`, http.StatusBadRequest)
		return
	}

	slog.Info("PKCE verified, issuing bearer token", "session_id", authCode.SessionID[:8]+"...")

	// Return our session ID as the MCP bearer token.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token": authCode.SessionID,
		"token_type":   "bearer",
		"expires_in":   int(sessionTTL.Seconds()),
	})
}

// EnsureFreshToken refreshes the Google access token if expired.
// Uses singleflight to ensure only one refresh happens per session,
// even when multiple concurrent requests arrive for the same user.
func (g *GoogleOAuth) EnsureFreshToken(sessionID string, session *Session) error {
	// 5-minute buffer before actual expiry.
	if time.Now().Before(session.ExpiresAt.Add(-5 * time.Minute)) {
		return nil
	}

	// Deduplicate concurrent refresh calls for the same session.
	// Only one goroutine calls Google; others wait and get the same result.
	_, err, _ := g.refreshGroup.Do(sessionID, func() (interface{}, error) {
		// Re-check inside singleflight — another goroutine may have already refreshed.
		if time.Now().Before(session.ExpiresAt.Add(-5 * time.Minute)) {
			return nil, nil
		}

		slog.Info("refreshing Google token", "email", session.Email)

		src := g.config.TokenSource(context.Background(), &oauth2.Token{
			RefreshToken: session.RefreshToken,
		})

		newToken, err := src.Token()
		if err != nil {
			return nil, fmt.Errorf("token refresh failed (user must re-login): %w", err)
		}

		session.AccessToken = newToken.AccessToken
		session.ExpiresAt = newToken.Expiry
		if newToken.RefreshToken != "" {
			session.RefreshToken = newToken.RefreshToken
		}

		return nil, nil
	})

	return err
}

// fetchEmail calls Google's userinfo endpoint. Optional — for logging only.
var emailClient = &http.Client{Timeout: 5 * time.Second}

func fetchEmail(accessToken string) string {
	req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return "unknown"
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := emailClient.Do(req)
	if err != nil {
		return "unknown"
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var info struct {
		Email string `json:"email"`
	}
	if json.Unmarshal(body, &info) != nil {
		return "unknown"
	}
	return info.Email
}

// isLoopbackURI returns true if the URI's host resolves to a loopback address.
func isLoopbackURI(rawURI string) bool {
	u, err := url.Parse(rawURI)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// matchesRegisteredURI checks if a redirect URI matches any registered URI.
// Per RFC 8252 Section 7.3, loopback URIs must allow any port —
// so we compare scheme + host + path, ignoring the port.
func matchesRegisteredURI(requestURI string, registeredURIs []string) bool {
	req, err := url.Parse(requestURI)
	if err != nil {
		return false
	}

	for _, registered := range registeredURIs {
		reg, err := url.Parse(registered)
		if err != nil {
			continue
		}
		// For loopback URIs: match scheme + host + path, ignore port.
		if req.Scheme == reg.Scheme && req.Hostname() == reg.Hostname() && req.Path == reg.Path {
			return true
		}
	}
	return false
}
