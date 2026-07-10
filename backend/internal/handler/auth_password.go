package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/auth"
	"github.com/kskgroup/eofficepro/internal/middleware"
)

const (
	sessionsPrefix = "sessions:" // Redis SET berisi refresh token aktif per pengguna

	// Suffix pada value refresh token yang menandai sesi "ingat saya".
	// Value lama (userID polos) terbaca sebagai remember=false.
	rememberSuffix = "|remember"
)

func (h *Handler) refreshTTL(remember bool) time.Duration {
	if remember {
		return time.Duration(h.Cfg.JWTRefreshRememberTTLHours) * time.Hour
	}
	return time.Duration(h.Cfg.JWTRefreshTTLHours) * time.Hour
}

func refreshValue(userID string, remember bool) string {
	if remember {
		return userID + rememberSuffix
	}
	return userID
}

func parseRefreshValue(v string) (userID string, remember bool) {
	if id, ok := strings.CutSuffix(v, rememberSuffix); ok {
		return id, true
	}
	return v, false
}

// storeRefresh menyimpan refresh token dan mendaftarkannya ke set sesi pengguna
// agar "logout semua perangkat" bisa mencabut seluruhnya. Flag remember
// ("ingat saya") menentukan TTL dan ikut tersimpan di value agar bertahan
// saat rotasi token.
func (h *Handler) storeRefresh(ctx context.Context, userID, token string, remember bool) error {
	pipe := h.Redis.TxPipeline()
	pipe.Set(ctx, refreshPrefix+token, refreshValue(userID, remember), h.refreshTTL(remember))
	pipe.SAdd(ctx, sessionsPrefix+userID, token)
	// Set sesi selalu memakai TTL terpanjang: sesi pendek tidak boleh
	// memperpendek umur set yang masih menampung token sesi "ingat saya".
	pipe.Expire(ctx, sessionsPrefix+userID, h.refreshTTL(true))
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

// ForgotPassword (E01-8) — kirim kode OTP 6 digit via email untuk reset dari
// aplikasi. Respons selalu sama, ada/tidaknya akun tidak bocor.
func (h *Handler) ForgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email tidak valid"})
		return
	}
	ctx := c.Request.Context()
	genericMsg := gin.H{"message": "jika email terdaftar, kode reset password telah dikirim"}

	var userID, fullName string
	err := h.DB.QueryRow(ctx,
		`SELECT id::text, full_name FROM users WHERE email = $1 AND status = 'active'`,
		req.Email).Scan(&userID, &fullName)
	if err != nil {
		c.JSON(http.StatusOK, genericMsg)
		return
	}

	// Cooldown anti-spam: maksimal satu email per menit per akun,
	// respons tetap generik agar tidak membocorkan keberadaan akun.
	ok, err := h.Redis.SetNX(ctx, pwResetCooldownPrefix+userID, "1", pwResetResendCooldown).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memproses permintaan"})
		return
	}
	if !ok {
		c.JSON(http.StatusOK, genericMsg)
		return
	}

	code, err := generateOTP()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membuat kode reset"})
		return
	}
	pipe := h.Redis.TxPipeline()
	pipe.Set(ctx, pwResetOTPPrefix+userID, hashOTP(code), pwResetOTPTTL)
	pipe.Del(ctx, pwResetAttemptsPrefix+userID) // kode baru = penghitung percobaan direset
	if _, err := pipe.Exec(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan kode reset"})
		return
	}

	body := fmt.Sprintf(
		"Halo %s,\n\nKami menerima permintaan reset password akun eOffice Pro Anda.\n"+
			"Masukkan kode berikut di aplikasi:\n\n%s\n\n"+
			"Kode berlaku 10 menit dan hanya dapat digunakan sekali.\n"+
			"Abaikan email ini jika Anda tidak meminta reset password.",
		fullName, code)
	if err := h.Mailer.Send(req.Email, "Kode Reset Password eOffice Pro", body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengirim email"})
		return
	}

	h.audit(ctx, "user", &userID, "password_reset_requested", nil, nil, c.ClientIP())
	c.JSON(http.StatusOK, genericMsg)
}

type resetPasswordRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Code        string `json:"code" binding:"required,len=6,numeric"`
	NewPassword string `json:"new_password" binding:"required,min=10"`
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email, kode 6 digit, dan password minimal 10 karakter wajib diisi"})
		return
	}
	ctx := c.Request.Context()
	// Error generik untuk email tak dikenal / kode salah / kode kedaluwarsa —
	// tidak membocorkan keberadaan akun maupun status kode.
	genericErr := gin.H{"error": "kode tidak valid atau sudah kedaluwarsa"}

	var userID string
	if err := h.DB.QueryRow(ctx,
		`SELECT id::text FROM users WHERE email = $1 AND status = 'active'`,
		req.Email).Scan(&userID); err != nil {
		c.JSON(http.StatusBadRequest, genericErr)
		return
	}

	// Batasi percobaan verifikasi: 6 digit hanya 1 juta kombinasi.
	attempts, err := h.Redis.Incr(ctx, pwResetAttemptsPrefix+userID).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memproses permintaan"})
		return
	}
	if attempts == 1 {
		_ = h.Redis.Expire(ctx, pwResetAttemptsPrefix+userID, pwResetOTPTTL).Err()
	}
	if attempts > pwResetMaxAttempts {
		_ = h.Redis.Del(ctx, pwResetOTPPrefix+userID).Err()
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "terlalu banyak percobaan, minta kode baru"})
		return
	}

	storedHash, err := h.Redis.Get(ctx, pwResetOTPPrefix+userID).Result()
	if err != nil || !otpMatches(storedHash, req.Code) {
		c.JSON(http.StatusBadRequest, genericErr)
		return
	}
	_ = h.Redis.Del(ctx, pwResetOTPPrefix+userID, pwResetAttemptsPrefix+userID, pwResetCooldownPrefix+userID).Err()

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
