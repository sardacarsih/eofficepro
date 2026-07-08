// Seed data dev minimal: go run ./cmd/seed
// Idempoten — aman dijalankan ulang untuk menyiapkan akun admin dan jabatan pembuat.
package main

import (
	"context"
	"errors"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
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

	email := getenv("ADMIN_EMAIL", "admin@ksk.local")
	password := getenv("ADMIN_PASSWORD", "GantiSegera#2026")
	userID := ensureAdminUser(ctx, db, email, password)
	companyID := ensureCompany(ctx, db)
	directorateID := ensureOrgUnit(ctx, db, companyID, nil, "DEV-DIR", "Direktorat Development", "directorate")
	departmentID := ensureOrgUnit(ctx, db, companyID, &directorateID, "DEV-DEPT", "Department Development", "department")
	presidentID := ensurePosition(ctx, db, directorateID, "President Director Development", "president_director", nil, true)
	directorID := ensurePosition(ctx, db, directorateID, "Director Development", "director", &presidentID, true)
	creatorPositionID := ensurePosition(ctx, db, departmentID, "Department Head Development", "dept_head", &directorID, true)
	ensureUserPosition(ctx, db, userID, creatorPositionID)
	approverID := ensureDevUser(ctx, db, "APR001", getenv("APPROVER_EMAIL", "approver@ksk.local"), "Approver Development", password, []string{"approver"})
	ensureUserPosition(ctx, db, approverID, directorID)

	log.Printf("seed siap: %s (NIK ADM001) punya role admin/creator; approver dev punya jabatan aktif", email)
}

func ensureAdminUser(ctx context.Context, db *pgxpool.Pool, email string, password string) string {
	var userID string
	err := db.QueryRow(ctx, `
		SELECT id::text
		FROM users
		WHERE nik = 'ADM001' OR email = $1
		LIMIT 1`, email).Scan(&userID)
	if err == nil {
		ensureUserRoles(ctx, db, userID, []string{"admin", "creator"})
		return userID
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		log.Fatalf("cek admin: %v", err)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}
	if err := db.QueryRow(ctx, `
		INSERT INTO users (nik, email, full_name, password_hash)
		VALUES ('ADM001', $1, 'Administrator Sistem', $2) RETURNING id::text`,
		email, hash).Scan(&userID); err != nil {
		log.Fatalf("insert admin: %v", err)
	}
	ensureUserRoles(ctx, db, userID, []string{"admin", "creator"})
	return userID
}

func ensureDevUser(ctx context.Context, db *pgxpool.Pool, nik string, email string, fullName string, password string, roles []string) string {
	var userID string
	err := db.QueryRow(ctx, `
		SELECT id::text
		FROM users
		WHERE nik = $1 OR email = $2
		LIMIT 1`, nik, email).Scan(&userID)
	if err == nil {
		ensureUserRoles(ctx, db, userID, roles)
		return userID
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		log.Fatalf("cek user %s: %v", nik, err)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("hash password %s: %v", nik, err)
	}
	if err := db.QueryRow(ctx, `
		INSERT INTO users (nik, email, full_name, password_hash)
		VALUES ($1, $2, $3, $4) RETURNING id::text`,
		nik, email, fullName, hash).Scan(&userID); err != nil {
		log.Fatalf("insert user %s: %v", nik, err)
	}
	ensureUserRoles(ctx, db, userID, roles)
	return userID
}

func ensureUserRoles(ctx context.Context, db *pgxpool.Pool, userID string, roles []string) {
	if _, err := db.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id
		FROM roles
		WHERE code = ANY($2)
		ON CONFLICT DO NOTHING`, userID, roles); err != nil {
		log.Fatalf("assign role: %v", err)
	}
}

func ensureCompany(ctx context.Context, db *pgxpool.Pool) string {
	var id string
	err := db.QueryRow(ctx, `
		INSERT INTO companies (code, name)
		VALUES ('KSK', 'PT Kalimantan Sawit Kusuma')
		ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name
		RETURNING id::text`).Scan(&id)
	if err != nil {
		log.Fatalf("ensure company: %v", err)
	}
	return id
}

func ensureOrgUnit(ctx context.Context, db *pgxpool.Pool, companyID string, parentID *string, code string, name string, level string) string {
	var id string
	err := db.QueryRow(ctx, `
		INSERT INTO org_units (company_id, parent_id, code, name, unit_level, region)
		VALUES ($1, $2, $3, $4, $5, 'HO')
		ON CONFLICT (company_id, code)
		DO UPDATE SET name = EXCLUDED.name, parent_id = EXCLUDED.parent_id, unit_level = EXCLUDED.unit_level
		RETURNING id::text`, companyID, parentID, code, name, level).Scan(&id)
	if err != nil {
		log.Fatalf("ensure org unit %s: %v", code, err)
	}
	return id
}

func ensurePosition(ctx context.Context, db *pgxpool.Pool, orgUnitID string, title string, positionType string, reportsTo *string, isApprover bool) string {
	var id string
	err := db.QueryRow(ctx, `
		SELECT id::text
		FROM positions
		WHERE org_unit_id = $1 AND title = $2 AND position_type = $3 AND is_active
		LIMIT 1`, orgUnitID, title, positionType).Scan(&id)
	if err == nil {
		if _, err := db.Exec(ctx, `
			UPDATE positions
			SET reports_to = $2, is_approver = $3
			WHERE id = $1`, id, reportsTo, isApprover); err != nil {
			log.Fatalf("update position %s: %v", title, err)
		}
		return id
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		log.Fatalf("cek position %s: %v", title, err)
	}

	if err := db.QueryRow(ctx, `
		INSERT INTO positions (org_unit_id, title, position_type, reports_to, is_approver)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text`, orgUnitID, title, positionType, reportsTo, isApprover).Scan(&id); err != nil {
		log.Fatalf("insert position %s: %v", title, err)
	}
	return id
}

func ensureUserPosition(ctx context.Context, db *pgxpool.Pool, userID string, positionID string) {
	var hasActivePosition bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM user_positions
			WHERE user_id = $1
			  AND position_id = $2
			  AND current_date >= valid_from
			  AND (valid_to IS NULL OR current_date <= valid_to)
		)`, userID, positionID).Scan(&hasActivePosition)
	if err != nil {
		log.Fatalf("cek penempatan admin: %v", err)
	}
	if hasActivePosition {
		return
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO user_positions (user_id, position_id, assignment_type)
		VALUES ($1, $2, 'definitive')`, userID, positionID); err != nil {
		log.Fatalf("assign admin position: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
