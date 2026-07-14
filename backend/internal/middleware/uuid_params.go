package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ValidateUUIDPathParams menolak dini param path bernama `id` atau berakhiran
// `_id` yang bukan UUID valid: 404 sebelum menyentuh handler/DB (menutup 500
// generik dari cast uuid Postgres yang gagal). Param non-UUID yang sah tidak
// terdampak karena namanya berbeda (`:scope` pada coordination rules; `:token`
// pada verify publik yang berada di luar grup terautentikasi).
func ValidateUUIDPathParams() gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, param := range c.Params {
			if param.Key != "id" && !strings.HasSuffix(param.Key, "_id") {
				continue
			}
			if !isCanonicalUUID(param.Value) {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "data tidak ditemukan"})
				return
			}
		}
		c.Next()
	}
}

// isCanonicalUUID memeriksa bentuk kanonis 8-4-4-4-12 heksadesimal
// (case-insensitive), sama dengan representasi uuid yang dikeluarkan Postgres.
func isCanonicalUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i := 0; i < len(value); i++ {
		ch := value[i]
		switch i {
		case 8, 13, 18, 23:
			if ch != '-' {
				return false
			}
		default:
			if !isHexDigit(ch) {
				return false
			}
		}
	}
	return true
}

func isHexDigit(ch byte) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}
