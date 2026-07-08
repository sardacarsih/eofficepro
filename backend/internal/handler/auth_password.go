package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/auth"
	"github.com/kskgroup/eofficepro/internal/middleware"
)

const (
	sessionsPrefix = "sessions:" // Redis SET berisi refresh token aktif per pengguna
	pwResetPrefix  = "pwreset:"
	pwResetTTL     = 30 * time.Minute
)

// storeRefresh menyimpan refresh token dan mendaftarkannya ke set sesi pengguna
// agar "logout semua perangkat" bisa mencabut seluruhnya.
func (h *Handler) storeRefresh(ctx context.Context, userID, token string) error {
	ttl := time.Duration(h.Cfg.JWTRefreshTTLHours) * time.Hour
	pipe := h.Redis.TxPipeline()
	pipe.Set(ctx, refreshPrefix+token, userID, ttl)
	pipe.SAdd(ctx, sessionsPrefix+userID, token)
	pipe.Expire(ctx, sessionsPrefix+userID, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (h *Handler) revokeAllSessions(ctx context.Context, userID string) int {
	tokens, err := h.Redis.SMembers(ctx, sessionsPrefix+userID).Result()
	if err != nil {
		return 0
	}
	pipe := h.Redis.TxPipeline()
	for _, t := range tokens {
		pipe.Del(ctx, refreshPrefix+t)
	}
	pipe.Del(ctx, sessionsPrefix+userID)
	_, _ = pipe.Exec(ctx)
	return len(tokens)
}

// LogoutAll (E01-2) mencabut seluruh refresh token pengguna. Access token yang
// masih beredar tetap berlaku sampai kedaluwarsa (maks. JWT_ACCESS_TTL_MINUTES).
func (h *Handler) LogoutAll(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	ctx := c.Request.Context()

	revoked := h.revokeAllSessions(ctx, userID)
	h.audit(ctx, "user", &userID, "logout_all", &userID,
		map[string]any{"revoked_sessions": revoked}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"message": "semua sesi dicabut", "revoked_sessions": revoked})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=10"`
}

// ChangePassword — pengguna login mengganti password sendiri dari menu profil.
// Password lama wajib diverifikasi; sukses = seluruh sesi dicabut (login ulang).
func (h *Handler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password lama wajib diisi dan password baru minimal 10 karakter"})
		return
	}
	userID := c.GetString(middleware.CtxUserID)
	ctx := c.Request.Context()

	var passwordHash string
	if err := h.DB.QueryRow(ctx,
		`SELECT password_hash FROM users WHERE id = $1 AND status = 'active'`, userID).
		Scan(&passwordHash); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "akun tidak aktif"})
		return
	}
	if !auth.VerifyPassword(req.CurrentPassword, passwordHash) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password lama salah"})
		return
	}
	if req.NewPassword == req.CurrentPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password baru harus berbeda dari password lama"})
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memproses password"})
		return
	}
	if _, err := h.DB.Exec(ctx, `
		UPDATE users SET password_hash = $2, failed_login_count = 0, locked_until = NULL, updated_at = now()
		WHERE id = $1`, userID, hash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan password"})
		return
	}

	// Password baru = semua sesi lama dicabut.
	h.revokeAllSessions(ctx, userID)
	h.audit(ctx, "user", &userID, "password_changed", &userID, nil, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"message": "password berhasil diubah, silakan login ulang"})
}

type forgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ForgotPassword (E01-8) — respons selalu sama, ada/tidaknya akun tidak bocor.
func (h *Handler) ForgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email tidak valid"})
		return
	}
	ctx := c.Request.Context()
	genericMsg := gin.H{"message": "jika email terdaftar, tautan reset password telah dikirim"}

	var userID, fullName string
	err := h.DB.QueryRow(ctx,
		`SELECT id::text, full_name FROM users WHERE email = $1 AND status = 'active'`,
		req.Email).Scan(&userID, &fullName)
	if err != nil {
		c.JSON(http.StatusOK, genericMsg)
		return
	}

	token, err := auth.NewRefreshToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membuat token reset"})
		return
	}
	if err := h.Redis.Set(ctx, pwResetPrefix+token, userID, pwResetTTL).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan token reset"})
		return
	}

	link := fmt.Sprintf("%s/reset-password?token=%s", h.Cfg.WebBaseURL, token)
	body := fmt.Sprintf(
		"Halo %s,\n\nKami menerima permintaan reset password akun eOffice Pro Anda.\n"+
			"Buka tautan berikut dalam 30 menit:\n\n%s\n\n"+
			"Abaikan email ini jika Anda tidak meminta reset password.",
		fullName, link)
	if err := h.Mailer.Send(req.Email, "Reset Password eOffice Pro", body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengirim email"})
		return
	}

	h.audit(ctx, "user", &userID, "password_reset_requested", nil, nil, c.ClientIP())
	c.JSON(http.StatusOK, genericMsg)
}

type resetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=10"`
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token wajib diisi dan password minimal 10 karakter"})
		return
	}
	ctx := c.Request.Context()

	userID, err := h.Redis.GetDel(ctx, pwResetPrefix+req.Token).Result() // sekali pakai
	if err != nil || userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token reset tidak valid atau sudah kedaluwarsa"})
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memproses password"})
		return
	}
	tag, err := h.DB.Exec(ctx, `
		UPDATE users SET password_hash = $2, failed_login_count = 0, locked_until = NULL, updated_at = now()
		WHERE id = $1 AND status = 'active'`, userID, hash)
	if err != nil || tag.RowsAffected() == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "akun tidak aktif"})
		return
	}

	// Password baru = semua sesi lama dicabut.
	h.revokeAllSessions(ctx, userID)
	h.audit(ctx, "user", &userID, "password_reset", &userID, nil, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"message": "password berhasil diubah, silakan login"})
}
