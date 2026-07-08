package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/auth"
	"github.com/kskgroup/eofficepro/internal/middleware"
)

const (
	maxFailedLogins = 5
	lockoutDuration = 15 * time.Minute
	refreshPrefix   = "refresh:"
)

type loginRequest struct {
	Identifier string `json:"identifier" binding:"required"` // email atau NIK
	Password   string `json:"password" binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "identifier dan password wajib diisi"})
		return
	}
	ctx := c.Request.Context()

	var (
		id, fullName, email, passwordHash, status string
		failedCount                               int
		lockedUntil                               *time.Time
	)
	err := h.DB.QueryRow(ctx, `
		SELECT id::text, full_name, email, password_hash, status, failed_login_count, locked_until
		FROM users WHERE email = $1 OR nik = $1`, req.Identifier).
		Scan(&id, &fullName, &email, &passwordHash, &status, &failedCount, &lockedUntil)
	if err != nil {
		// Respons sengaja sama dengan password salah agar tidak membocorkan
		// keberadaan akun.
		c.JSON(http.StatusUnauthorized, gin.H{"error": "identifier atau password salah"})
		return
	}

	if status != "active" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "akun tidak aktif — hubungi admin IT"})
		return
	}
	if lockedUntil != nil && lockedUntil.After(time.Now()) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "akun terkunci sementara karena percobaan gagal berulang, coba lagi nanti"})
		return
	}

	if !auth.VerifyPassword(req.Password, passwordHash) {
		_, _ = h.DB.Exec(ctx, `
			UPDATE users SET
				failed_login_count = failed_login_count + 1,
				locked_until = CASE WHEN failed_login_count + 1 >= $2 THEN now() + $3::interval ELSE locked_until END
			WHERE id = $1`, id, maxFailedLogins, lockoutDuration.String())
		h.audit(ctx, "user", &id, "login_failed", nil, map[string]any{"identifier": req.Identifier}, c.ClientIP())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "identifier atau password salah"})
		return
	}

	_, _ = h.DB.Exec(ctx, `
		UPDATE users SET failed_login_count = 0, locked_until = NULL, last_login_at = now()
		WHERE id = $1`, id)

	roles, err := h.userRoles(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat role pengguna"})
		return
	}

	access, exp, err := h.Tokens.IssueAccess(id, fullName, roles)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menerbitkan token"})
		return
	}
	refresh, err := auth.NewRefreshToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menerbitkan refresh token"})
		return
	}
	if err := h.storeRefresh(ctx, id, refresh); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan sesi"})
		return
	}

	h.audit(ctx, "user", &id, "login", &id, nil, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{
		"access_token":  access,
		"expires_at":    exp,
		"refresh_token": refresh,
		"user":          gin.H{"id": id, "full_name": fullName, "email": email, "roles": roles},
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *Handler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token wajib diisi"})
		return
	}
	ctx := c.Request.Context()

	userID, err := h.Redis.GetDel(ctx, refreshPrefix+req.RefreshToken).Result() // rotasi: sekali pakai
	if err != nil || userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "sesi tidak valid, silakan login ulang"})
		return
	}
	_ = h.Redis.SRem(ctx, sessionsPrefix+userID, req.RefreshToken).Err()

	var fullName, status string
	if err := h.DB.QueryRow(ctx,
		`SELECT full_name, status FROM users WHERE id = $1`, userID).
		Scan(&fullName, &status); err != nil || status != "active" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "akun tidak aktif"})
		return
	}

	roles, err := h.userRoles(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat role pengguna"})
		return
	}
	access, exp, err := h.Tokens.IssueAccess(userID, fullName, roles)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menerbitkan token"})
		return
	}
	newRefresh, err := auth.NewRefreshToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menerbitkan refresh token"})
		return
	}
	if err := h.storeRefresh(ctx, userID, newRefresh); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan sesi"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  access,
		"expires_at":    exp,
		"refresh_token": newRefresh,
	})
}

func (h *Handler) Logout(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err == nil && req.RefreshToken != "" {
		ctx := c.Request.Context()
		if userID, err := h.Redis.GetDel(ctx, refreshPrefix+req.RefreshToken).Result(); err == nil && userID != "" {
			_ = h.Redis.SRem(ctx, sessionsPrefix+userID, req.RefreshToken).Err()
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "logout berhasil"})
}

func (h *Handler) Me(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	ctx := c.Request.Context()

	var nik, email, fullName, status string
	if err := h.DB.QueryRow(ctx,
		`SELECT nik, email, full_name, status FROM users WHERE id = $1`, userID).
		Scan(&nik, &email, &fullName, &status); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "pengguna tidak ditemukan"})
		return
	}

	roles, _ := h.userRoles(ctx, userID)

	type positionInfo struct {
		PositionID   string `json:"position_id"`
		Title        string `json:"title"`
		PositionType string `json:"position_type"`
		OrgUnit      string `json:"org_unit"`
		Assignment   string `json:"assignment_type"`
	}
	positions := []positionInfo{}
	rows, err := h.DB.Query(ctx, `
		SELECT p.id::text, p.title, p.position_type, ou.name, up.assignment_type
		FROM user_positions up
		JOIN positions p ON p.id = up.position_id
		JOIN org_units ou ON ou.id = p.org_unit_id
		WHERE up.user_id = $1
		  AND current_date >= up.valid_from
		  AND (up.valid_to IS NULL OR current_date < up.valid_to)`, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p positionInfo
			if rows.Scan(&p.PositionID, &p.Title, &p.PositionType, &p.OrgUnit, &p.Assignment) == nil {
				positions = append(positions, p)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"id": userID, "nik": nik, "email": email, "full_name": fullName,
		"status": status, "roles": roles, "positions": positions,
	})
}
