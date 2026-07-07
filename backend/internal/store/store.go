// Package store menyediakan koneksi ke infrastruktur data:
// PostgreSQL (data utama), Redis (cache & antrian notifikasi),
// dan MinIO (object storage lampiran surat).
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"

	"github.com/kskgroup/eofficepro/internal/config"
)

type Store struct {
	DB     *pgxpool.Pool
	Redis  *redis.Client
	Minio  *minio.Client
	Bucket string
}

func New(ctx context.Context, cfg *config.Config) (*Store, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})

	mc, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Secure: cfg.MinioUseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio: %w", err)
	}

	return &Store{DB: pool, Redis: rdb, Minio: mc, Bucket: cfg.MinioBucket}, nil
}

// Health mengembalikan status tiap dependency; dipakai endpoint /healthz.
func (s *Store) Health(ctx context.Context) map[string]string {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	status := map[string]string{"postgres": "ok", "redis": "ok", "minio": "ok"}

	if err := s.DB.Ping(ctx); err != nil {
		status["postgres"] = err.Error()
	}
	if err := s.Redis.Ping(ctx).Err(); err != nil {
		status["redis"] = err.Error()
	}
	if _, err := s.Minio.BucketExists(ctx, s.Bucket); err != nil {
		status["minio"] = err.Error()
	}
	return status
}

func (s *Store) Close() {
	s.DB.Close()
	_ = s.Redis.Close()
}
