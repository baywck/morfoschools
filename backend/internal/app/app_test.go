package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestApp() *App {
	a, _ := New(Config{
		Port:   "8080",
		AppEnv: "test",
		// Tests assert that http://localhost:1666 is whitelisted; mirror
		// the dev fallback list so the assertions don't depend on env.
		AllowedOrigins: []string{
			"http://localhost:1666",
			"http://127.0.0.1:1666",
		},
	}, nil, nil)
	return a
}

func TestHealthz(t *testing.T) {
	a := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	a.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}
}

func TestReadyz(t *testing.T) {
	a := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	a.Handler().ServeHTTP(w, req)

	// Without DB, readyz still returns 200 (db shows as not_configured)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	a := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	a.Handler().ServeHTTP(w, req)

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
		"Referrer-Policy":       "strict-origin-when-cross-origin",
	}
	for key, expected := range headers {
		if got := w.Header().Get(key); got != expected {
			t.Errorf("header %s: expected %q, got %q", key, expected, got)
		}
	}
}

func TestCORSPreflight(t *testing.T) {
	a := newTestApp()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/anything", nil)
	req.Header.Set("Origin", "http://localhost:1666")
	w := httptest.NewRecorder()

	a.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:1666" {
		t.Errorf("expected allowed origin, got %q", got)
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	a := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "http://evil.com")
	w := httptest.NewRecorder()

	a.Handler().ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no CORS header for unknown origin, got %q", got)
	}
}

func TestRequestIDGenerated(t *testing.T) {
	a := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	a.Handler().ServeHTTP(w, req)

	if got := w.Header().Get("X-Request-ID"); got == "" {
		t.Error("expected X-Request-ID header to be set")
	}
}

func TestRequestIDPreserved(t *testing.T) {
	a := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", "custom-123")
	w := httptest.NewRecorder()

	a.Handler().ServeHTTP(w, req)

	if got := w.Header().Get("X-Request-ID"); got != "custom-123" {
		t.Errorf("expected custom-123, got %q", got)
	}
}
