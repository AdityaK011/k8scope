package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimiterAllows(t *testing.T) {
	rl, stop := NewRateLimitMiddleware()
	defer stop()

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl(okHandler)

	// The rate limiter allows a burst of 5. Send 5 requests; all should pass.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}
}

func TestRateLimiterBlocks(t *testing.T) {
	rl, stop := NewRateLimitMiddleware()
	defer stop()

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl(okHandler)

	// Burst is 5 and rate is 10/min (1 every 6s). Sending 10 rapid requests
	// from the same IP should result in some being rate-limited.
	var okCount, blockedCount int
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		switch rec.Code {
		case http.StatusOK:
			okCount++
		case http.StatusTooManyRequests:
			blockedCount++
		default:
			t.Fatalf("request %d: unexpected status %d", i+1, rec.Code)
		}
	}

	if blockedCount == 0 {
		t.Error("expected at least one request to be rate-limited (429), but all passed")
	}
	if okCount == 0 {
		t.Error("expected at least one request to succeed (200), but all were blocked")
	}

	t.Logf("out of 10 requests: %d OK, %d blocked", okCount, blockedCount)
}
