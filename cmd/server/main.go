package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/AdityaK011/k8scope/internal/auth"
	"github.com/AdityaK011/k8scope/internal/tools"
)

func main() {
	// Required env vars.
	clientID := mustEnv("GOOGLE_CLIENT_ID")
	clientSecret := mustEnv("GOOGLE_CLIENT_SECRET")
	redirectURL := mustEnv("REDIRECT_URL") // e.g. https://k8scope.example.com/callback
	port := getEnv("PORT", "8080")

	// Init OAuth handler.
	oauth := auth.NewGoogleOAuth(clientID, clientSecret, redirectURL)
	oauth.TrustProxy = getEnv("TRUST_PROXY", "") == "true"
	stopCleanup := oauth.Store.StartCleanup()
	rateLimiter, stopRateLimiter := auth.NewRateLimitMiddleware()

	// Init MCP server.
	mcpServer := server.NewMCPServer(
		"k8scope",
		"0.1.0",
		server.WithToolCapabilities(true),
	)
	tools.Register(mcpServer)

	// Create the Streamable HTTP transport.
	mcpHTTP := server.NewStreamableHTTPServer(mcpServer)

	// Wire everything together.
	mux := http.NewServeMux()

	// OAuth endpoints (no auth required).
	oauth.Register(mux, rateLimiter)

	// Health check.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	// MCP endpoints (auth required).
	// The StreamableHTTPServer handles /mcp paths.
	mux.Handle("/mcp/", oauth.Middleware(mcpHTTP))
	mux.Handle("/mcp", oauth.Middleware(mcpHTTP))

	addr := fmt.Sprintf(":%s", port)
	slog.Info("k8scope MCP server starting",
		"port", port,
		"redirect_url", redirectURL,
	)

	srv := &http.Server{
		Addr:              addr,
		Handler:           recoverMiddleware(corsMiddleware(mux)),
		ReadHeaderTimeout: 10 * time.Second, // protects against slowloris
		IdleTimeout:       120 * time.Second,
		// WriteTimeout intentionally omitted — MCP uses SSE streams
		// that stay open longer than any fixed timeout.
	}

	// Start server in background.
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for SIGTERM (Kubernetes) or SIGINT (Ctrl+C).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	sig := <-quit
	slog.Info("shutting down", "signal", sig.String())

	// Give in-flight requests 15 seconds to complete.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	// Stop background goroutines.
	stopCleanup()
	stopRateLimiter()
	slog.Info("server stopped gracefully")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env var missing", "key", key)
		os.Exit(1)
	}
	return v
}

// recoverMiddleware catches panics in handlers and returns 500 instead of crashing.
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered", "error", err, "path", r.URL.Path)
				http.Error(w, `{"error":"internal_server_error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware blocks cross-origin browser requests.
// The MCP server is not intended to be called from browser JS,
// so no Access-Control-Allow-Origin is set (effectively same-origin only).
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reject CORS preflight — no browser JS should be calling this server.
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
