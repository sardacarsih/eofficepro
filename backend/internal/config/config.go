package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	AppEnv   string
	HTTPPort string

	DatabaseURL string

	RedisAddr     string
	RedisPassword string

	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioUseSSL    bool

	JWTSecret            string
	JWTAccessTTLMinutes  int
	JWTRefreshTTLHours   int

	// SMTP kosong = mode dev: email dicetak ke log aplikasi.
	SMTPHost   string
	SMTPPort   int
	SMTPUser   string
	SMTPPass   string
	SMTPFrom   string
	WebBaseURL string
}

func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:               getEnv("APP_ENV", "development"),
		HTTPPort:             getEnv("HTTP_PORT", "8080"),
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://eoffice:eoffice_dev_secret@localhost:5433/eoffice?sslmode=disable"),
		RedisAddr:            getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:        getEnv("REDIS_PASSWORD", ""),
		MinioEndpoint:        getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey:       getEnv("MINIO_ACCESS_KEY", "eoffice"),
		MinioSecretKey:       getEnv("MINIO_SECRET_KEY", "eoffice_dev_secret"),
		MinioBucket:          getEnv("MINIO_BUCKET", "eoffice-attachments"),
		MinioUseSSL:          getEnvBool("MINIO_USE_SSL", false),
		JWTSecret:            getEnv("JWT_SECRET", ""),
		JWTAccessTTLMinutes:  getEnvInt("JWT_ACCESS_TTL_MINUTES", 30),
		JWTRefreshTTLHours:   getEnvInt("JWT_REFRESH_TTL_HOURS", 168),
		SMTPHost:             getEnv("SMTP_HOST", ""),
		SMTPPort:             getEnvInt("SMTP_PORT", 587),
		SMTPUser:             getEnv("SMTP_USER", ""),
		SMTPPass:             getEnv("SMTP_PASS", ""),
		SMTPFrom:             getEnv("SMTP_FROM", "eoffice@ksk.local"),
		WebBaseURL:           getEnv("WEB_BASE_URL", "http://localhost:3000"),
	}

	if cfg.AppEnv != "development" && cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET wajib diisi di luar environment development")
	}
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = "dev-only-insecure-secret"
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return fallback
}
