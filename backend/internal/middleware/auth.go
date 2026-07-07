package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/auth"
)

const (
	CtxUserID = "auth.userID"
	CtxName   = "auth.name"
	CtxRoles  = "auth.roles"
)

func RequireAuth(issuer *auth.TokenIssuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		tokenStr, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token tidak ditemukan"})
			return
		}
		claims, err := issuer.ParseAccess(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token tidak valid atau kedaluwarsa"})
			return
		}
		c.Set(CtxUserID, claims.Subject)
		c.Set(CtxName, claims.Name)
		c.Set(CtxRoles, claims.Roles)
		c.Next()
	}
}

// RequireRole meloloskan pengguna yang memiliki salah satu role yang diminta.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRoles, _ := c.Get(CtxRoles)
		have, _ := userRoles.([]string)
		for _, want := range roles {
			if slices.Contains(have, want) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "tidak punya akses untuk aksi ini"})
	}
}
