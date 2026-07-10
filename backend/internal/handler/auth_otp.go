package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"
)

// Kode OTP reset password (E01-8): 6 digit, dikirim via email, diverifikasi
// dari aplikasi mobile. Kode disimpan sebagai hash SHA-256 di Redis.
const (
	pwResetOTPPrefix      = "pwreset_otp:"      // hash kode aktif per user
	pwResetAttemptsPrefix = "pwreset_attempts:" // penghitung percobaan verifikasi
	pwResetCooldownPrefix = "pwreset_cooldown:" // jeda minimal antar pengiriman email
	pwResetOTPTTL         = 10 * time.Minute
	pwResetMaxAttempts    = 5
	pwResetResendCooldown = 60 * time.Second
)

func generateOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func hashOTP(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func otpMatches(storedHash, code string) bool {
	return subtle.ConstantTimeCompare([]byte(storedHash), []byte(hashOTP(code))) == 1
}
