// Seed pengguna admin pertama: go run ./cmd/seed
// Idempoten — tidak melakukan apa pun bila sudah ada pengguna.
package main

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/kskgroup/eofficepro/internal/auth"
	"github.com/kskgroup/eofficepro/internal/config"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
		log.Fatalf("cek users: %v", err)
	}
	if count > 0 {
		log.Printf("seed dilewati: sudah ada %d pengguna", count)
		return
	}

	email := getenv("ADMIN_EMAIL", "admin@ksk.local")
	password := getenv("ADMIN_PASSWORD", "GantiSegera#2026")
	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		log.Fatalf("tx: %v", err)
	}
	defer tx.Rollback(ctx)

	var userID string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (nik, email, full_name, password_hash)
		VALUES ('ADM001', $1, 'Administrator Sistem', $2) RETURNING id::text`,
		email, hash).Scan(&userID)
	if err != nil {
		log.Fatalf("insert admin: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE code IN ('admin','creator')`, userID); err != nil {
		log.Fatalf("assign role: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit: %v", err)
	}

	log.Printf("admin dibuat: %s (NIK ADM001) — segera ganti password default", email)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
