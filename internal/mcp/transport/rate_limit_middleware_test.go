package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"mcp-server-database/internal/observability"
)

func TestWithRateLimitShouldNoopWhenDisabled(t *testing.T) {
	handler := WithRateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), RateLimitConfig{Enabled: false}, observability.NopLogger())

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
	}
}

func TestWithRateLimitShouldRejectWhenExceeded(t *testing.T) {
	handler := WithRateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), RateLimitConfig{Enabled: true, RPS: 1, Burst: 1}, observability.NopLogger())

	req1 := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req1.RemoteAddr = "10.0.0.1:1234"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first request status 200, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req2.RemoteAddr = "10.0.0.1:1234"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request status 429, got %d", rec2.Code)
	}
}
