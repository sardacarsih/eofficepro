package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newSecurityHeadersEngine(production bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders(production))
	ok := func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) }
	r.GET("/healthz", ok)
	r.GET("/api/v1/letters/inbox", ok)
	return r
}

func doHeadersRequest(r *gin.Engine, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestSecurityHeadersOnAPIPath(t *testing.T) {
	r := newSecurityHeadersEngine(false)
	rec := doHeadersRequest(r, "/api/v1/letters/inbox")

	for header, want := range map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "no-referrer",
		"Content-Security-Policy": "default-src 'none'",
		"Cache-Control":           "no-store",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("Strict-Transport-Security = %q, want kosong di luar production", got)
	}
}

func TestSecurityHeadersNonAPIPathSkipsNoStore(t *testing.T) {
	r := newSecurityHeadersEngine(false)
	rec := doHeadersRequest(r, "/healthz")

	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Errorf("Cache-Control /healthz = %q, want kosong (no-store hanya /api/)", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options /healthz = %q, want nosniff (global)", got)
	}
}

func TestSecurityHeadersHSTSOnlyInProduction(t *testing.T) {
	r := newSecurityHeadersEngine(true)
	rec := doHeadersRequest(r, "/api/v1/letters/inbox")

	if got := rec.Header().Get("Strict-Transport-Security"); got != "max-age=31536000" {
		t.Errorf("Strict-Transport-Security production = %q, want max-age=31536000", got)
	}
}
