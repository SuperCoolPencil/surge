package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCorsVulnerability confirms that the fix restricts origins correctly.
func TestCorsVulnerability(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := corsMiddleware(handler)

	// Helper function to test an origin
	testOrigin := func(origin string, expectedAllowed bool) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		corsHandler.ServeHTTP(rec, req)

		allowOrigin := rec.Header().Get("Access-Control-Allow-Origin")
		if expectedAllowed {
			if allowOrigin != origin {
				t.Errorf("Origin '%s' should be allowed, got '%s'", origin, allowOrigin)
			}
		} else {
			if allowOrigin != "" {
				t.Errorf("Origin '%s' should be blocked (empty header), got '%s'", origin, allowOrigin)
			}
		}
	}

	// Malicious origins
	testOrigin("http://malicious.com", false)
	testOrigin("https://attacker.site", false)

	// Bypass attempts (subdomain/prefix matching)
	testOrigin("http://localhost.evil.com", false)
	testOrigin("http://127.0.0.1.nip.io", false)
	testOrigin("http://localhost-evil.com", false)

	// Valid origins
	testOrigin("chrome-extension://abcdefghijklmnop", true)
	testOrigin("moz-extension://abcdef-1234-5678", true)
	testOrigin("http://localhost:3000", true)
	testOrigin("http://127.0.0.1:8080", true)
	testOrigin("http://localhost", true)
	testOrigin("http://127.0.0.1", true)
}
