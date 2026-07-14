package middleware

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// rateLimitWindow adalah lebar fixed window pembatas laju.
const rateLimitWindow = time.Minute

// RateLimitStore adalah subset perintah Redis yang dipakai penghitung
// fixed-window. *redis.Client memenuhinya; test memakai fake in-memory.
type RateLimitStore interface {
	Incr(ctx context.Context, key string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	TTL(ctx context.Context, key string) *redis.DurationCmd
}

// RateLimitByIP membatasi laju request per IP klien (endpoint publik seperti
// login/forgot-password). perMinute <= 0 menonaktifkan pembatasan.
func RateLimitByIP(store RateLimitStore, scope string, perMinute int) gin.HandlerFunc {
	return rateLimit(store, scope, perMinute, func(c *gin.Context) string {
		return c.ClientIP()
	})
}

// RateLimitByUser membatasi laju request per user terautentikasi; dipasang
// SETELAH RequireAuth sehingga CtxUserID tersedia. Bila user id kosong
// (seharusnya tidak terjadi di jalur terautentikasi), jatuh ke IP klien.
func RateLimitByUser(store RateLimitStore, scope string, perMinute int) gin.HandlerFunc {
	return rateLimit(store, scope, perMinute, func(c *gin.Context) string {
		if userID := c.GetString(CtxUserID); userID != "" {
			return userID
		}
		return c.ClientIP()
	})
}

// rateLimit mengimplementasikan fixed window di Redis: INCR + EXPIRE (TTL
// dipasang hanya saat hitungan pertama). Fail-open: bila Redis error, request
// diloloskan dengan log peringatan (availability > strictness).
func rateLimit(store RateLimitStore, scope string, perMinute int, keyFunc func(*gin.Context) string) gin.HandlerFunc {
	if store == nil || perMinute <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		key := "ratelimit:" + scope + ":" + keyFunc(c)

		count, err := store.Incr(ctx, key).Result()
		if err != nil {
			log.Printf("rate limit %s: Redis tidak tersedia, request diloloskan: %v", scope, err)
			c.Next()
			return
		}
		if count == 1 {
			if err := store.Expire(ctx, key, rateLimitWindow).Err(); err != nil {
				log.Printf("rate limit %s: gagal memasang TTL window: %v", scope, err)
			}
		}
		if count <= int64(perMinute) {
			c.Next()
			return
		}

		retryAfter := rateLimitWindow
		if ttl, err := store.TTL(ctx, key).Result(); err == nil {
			if ttl > 0 {
				retryAfter = ttl
			} else {
				// Kunci tanpa TTL (EXPIRE awal gagal): pasang ulang agar window
				// tidak macet menolak permanen.
				_ = store.Expire(ctx, key, rateLimitWindow).Err()
			}
		}
		seconds := int((retryAfter + time.Second - 1) / time.Second)
		if seconds < 1 {
			seconds = 1
		}
		c.Header("Retry-After", strconv.Itoa(seconds))
		c.AbortWithStatusJSON(http.StatusTooManyRequests,
			gin.H{"error": "terlalu banyak permintaan, coba lagi nanti"})
	}
}
