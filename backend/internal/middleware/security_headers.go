package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders memasang header keamanan untuk respons API JSON (bukan
// halaman HTML). Dipasang global di router.
//
//   - X-Content-Type-Options: nosniff — cegah MIME sniffing.
//   - X-Frame-Options: DENY — API tidak pernah dirender dalam frame.
//   - Referrer-Policy: no-referrer.
//   - Content-Security-Policy: default-src 'none' — respons API bukan dokumen
//     aktif; tidak ada resource yang boleh dimuat darinya.
//   - Cache-Control: no-store untuk path /api/ — respons berisi data organisasi
//     sensitif; unduhan streaming ikut no-store (berjalan sekali, tidak perlu
//     cache).
//   - Strict-Transport-Security hanya saat production (di belakang TLS).
func SecurityHeaders(production bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.Writer.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("Content-Security-Policy", "default-src 'none'")
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			header.Set("Cache-Control", "no-store")
		}
		if production {
			header.Set("Strict-Transport-Security", "max-age=31536000")
		}
		c.Next()
	}
}
