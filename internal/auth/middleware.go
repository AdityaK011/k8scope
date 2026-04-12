package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

type contextKey string

const sessionCtxKey contextKey = "session"

// Middleware returns an http.Handler that extracts the session ID from
// the Authorization header and injects the Session into the request context.
// If the token is missing or invalid, it returns 401.
func (g *GoogleOAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearer(r)
		if token == "" {
			http.Error(w, `{"error":"unauthorized","hint":"no bearer token"}`, http.StatusUnauthorized)
			return
		}

		session, err := g.Store.GetSession(token)
		if err != nil {
			http.Error(w, `{"error":"session_expired","hint":"re-authenticate"}`, http.StatusUnauthorized)
			return
		}

		// Refresh Google access token if needed.
		if err := g.EnsureFreshToken(token, session); err != nil {
			slog.Error("token refresh failed", "email", session.Email, "error", err)
			http.Error(w, `{"error":"token_refresh_failed","hint":"re-authenticate"}`, http.StatusUnauthorized)
			return
		}
		g.Store.UpdateSession(token, session)

		// Inject session into context for tool handlers.
		ctx := context.WithValue(r.Context(), sessionCtxKey, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// SessionFromContext retrieves the Session injected by the auth middleware.
// Returns an error if called outside of an authenticated request.
func SessionFromContext(ctx context.Context) (*Session, error) {
	sess, ok := ctx.Value(sessionCtxKey).(*Session)
	if !ok {
		return nil, fmt.Errorf("no session in context: request is not authenticated")
	}
	return sess, nil
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}
