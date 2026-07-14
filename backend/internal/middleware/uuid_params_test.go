package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newUUIDParamsEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	group := r.Group("", ValidateUUIDPathParams())
	ok := func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) }
	group.GET("/letters/view/:id", ok)
	group.GET("/letters/view/:id/attachments/:attachment_id/download", ok)
	group.PUT("/coordination-scope-rules/:scope", ok)
	group.DELETE("/delegations/:id", ok)
	return r
}

func TestValidateUUIDPathParams(t *testing.T) {
	r := newUUIDParamsEngine()
	const validUUID = "3f2b8c1e-9d4a-4f6b-8a2c-5e7d9b1a3c5f"

	tests := []struct {
		name     string
		method   string
		path     string
		wantCode int
	}{
		{"id_uuid_valid", http.MethodGet, "/letters/view/" + validUUID, http.StatusOK},
		{"id_uuid_huruf_besar", http.MethodGet, "/letters/view/3F2B8C1E-9D4A-4F6B-8A2C-5E7D9B1A3C5F", http.StatusOK},
		{"id_non_uuid", http.MethodGet, "/letters/view/not-a-uuid", http.StatusNotFound},
		{"id_non_uuid_36_char", http.MethodGet, "/letters/view/zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz", http.StatusNotFound},
		{"delegation_id_non_uuid", http.MethodDelete, "/delegations/123", http.StatusNotFound},
		{"attachment_id_non_uuid", http.MethodGet, "/letters/view/" + validUUID + "/attachments/abc/download", http.StatusNotFound},
		{"attachment_id_valid", http.MethodGet, "/letters/view/" + validUUID + "/attachments/" + validUUID + "/download", http.StatusOK},
		{"scope_bukan_uuid_tidak_terdampak", http.MethodPut, "/coordination-scope-rules/cross_directorate", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			if rec.Code != tt.wantCode {
				t.Fatalf("%s %s status = %d, want %d; body = %s", tt.method, tt.path, rec.Code, tt.wantCode, rec.Body.String())
			}
			if tt.wantCode == http.StatusNotFound {
				var body struct {
					Error string `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("json.Unmarshal(404 body) error: %v", err)
				}
				if body.Error != "data tidak ditemukan" {
					t.Errorf("404 error = %q, want \"data tidak ditemukan\"", body.Error)
				}
			}
		})
	}
}
