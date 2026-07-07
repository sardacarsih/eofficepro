package server

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/config"
	"github.com/kskgroup/eofficepro/internal/handler"
	"github.com/kskgroup/eofficepro/internal/middleware"
	"github.com/kskgroup/eofficepro/internal/store"
)

func NewRouter(cfg *config.Config, st *store.Store) *gin.Engine {
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(cors.New(cors.Config{
		// Dev: web Next.js di port 3000. Production diatur lewat reverse proxy.
		AllowOrigins:     []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	h := handler.New(st.DB, st.Redis, cfg)

	r.GET("/healthz", func(c *gin.Context) {
		deps := st.Health(c.Request.Context())
		code := http.StatusOK
		for _, v := range deps {
			if v != "ok" {
				code = http.StatusServiceUnavailable
				break
			}
		}
		c.JSON(code, gin.H{"service": "eoffice-api", "deps": deps})
	})

	api := r.Group("/api/v1")

	// Publik
	api.POST("/auth/login", h.Login)
	api.POST("/auth/refresh", h.Refresh)
	api.POST("/auth/logout", h.Logout)
	api.POST("/auth/forgot-password", h.ForgotPassword)
	api.POST("/auth/reset-password", h.ResetPassword)

	// Terproteksi
	authed := api.Group("", middleware.RequireAuth(h.Tokens))
	authed.GET("/auth/me", h.Me)
	authed.POST("/auth/logout-all", h.LogoutAll)

	authed.GET("/org-units", h.OrgTree)
	authed.GET("/positions", h.ListPositions)
	authed.GET("/letter-types", h.ListLetterTypes)

	// Khusus admin
	admin := authed.Group("", middleware.RequireRole("admin"))
	admin.POST("/org-units", h.CreateOrgUnit)
	admin.PUT("/org-units/:id", h.UpdateOrgUnit)
	admin.DELETE("/org-units/:id", h.DeactivateOrgUnit)
	admin.POST("/positions", h.CreatePosition)
	admin.POST("/positions/:id/assign", h.AssignPosition)
	admin.POST("/letter-types", h.CreateLetterType)
	admin.PUT("/letter-types/:id", h.UpdateLetterType)
	admin.DELETE("/letter-types/:id", h.DeactivateLetterType)
	admin.GET("/users", h.ListUsers)
	admin.POST("/users", h.CreateUser)
	admin.PUT("/users/:id", h.UpdateUser)
	admin.DELETE("/users/:id", h.DeactivateUser)
	admin.GET("/users/import/template", h.ImportTemplate)
	admin.POST("/users/import", h.ImportUsers)

	return r
}
