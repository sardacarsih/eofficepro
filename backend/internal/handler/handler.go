// Package handler berisi HTTP handler per domain (Epic E01: auth & organisasi).
package handler

import (
	"context"
	"encoding/json"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"

	"github.com/kskgroup/eofficepro/internal/auth"
	"github.com/kskgroup/eofficepro/internal/config"
	"github.com/kskgroup/eofficepro/internal/mail"
)

type Handler struct {
	DB     *pgxpool.Pool
	Redis  *redis.Client
	Minio  *minio.Client
	Bucket string
	Cfg    *config.Config
	Tokens *auth.TokenIssuer
	Mailer *mail.Mailer
}

func New(db *pgxpool.Pool, rdb *redis.Client, mc *minio.Client, bucket string, cfg *config.Config) *Handler {
	return &Handler{
		DB:     db,
		Redis:  rdb,
		Minio:  mc,
		Bucket: bucket,
		Cfg:    cfg,
		Tokens: auth.NewTokenIssuer(cfg.JWTSecret, cfg.JWTAccessTTLMinutes),
		Mailer: mail.New(cfg),
	}
}

// audit menulis jejak ke audit_logs; kegagalan hanya dicatat ke log aplikasi
// agar tidak menggagalkan aksi utama (P0-10: semua aksi penting terekam).
func (h *Handler) audit(ctx context.Context, entityType string, entityID *string,
	action string, actorUserID *string, detail map[string]any, ip string) {

	detailJSON, _ := json.Marshal(detail)
	_, err := h.DB.Exec(ctx, `
		INSERT INTO audit_logs (entity_type, entity_id, action, actor_user_id, detail, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		entityType, entityID, action, actorUserID, detailJSON, ip)
	if err != nil {
		log.Printf("audit log gagal (%s/%s): %v", entityType, action, err)
	}
}

func (h *Handler) userRoles(ctx context.Context, userID string) ([]string, error) {
	rows, err := h.DB.Query(ctx, `
		SELECT r.code FROM roles r
		JOIN user_roles ur ON ur.role_id = r.id
		WHERE ur.user_id = $1 ORDER BY r.code`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := []string{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		roles = append(roles, code)
	}
	return roles, rows.Err()
}
