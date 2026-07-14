package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// fakeRateLimitStore mensimulasikan INCR/EXPIRE/TTL Redis in-memory dengan jam
// yang bisa dimajukan (uji reset window tanpa sleep).
type fakeRateLimitStore struct {
	mu     sync.Mutex
	now    time.Time
	counts map[string]int64
	expiry map[string]time.Time
}

func newFakeRateLimitStore() *fakeRateLimitStore {
	return &fakeRateLimitStore{
		now:    time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC),
		counts: map[string]int64{},
		expiry: map[string]time.Time{},
	}
}

func (s *fakeRateLimitStore) advance(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = s.now.Add(d)
}

func (s *fakeRateLimitStore) expireIfDueLocked(key string) {
	if deadline, ok := s.expiry[key]; ok && !s.now.Before(deadline) {
		delete(s.expiry, key)
		delete(s.counts, key)
	}
}

func (s *fakeRateLimitStore) Incr(_ context.Context, key string) *redis.IntCmd {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireIfDueLocked(key)
	s.counts[key]++
	return redis.NewIntResult(s.counts[key], nil)
}

func (s *fakeRateLimitStore) Expire(_ context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expiry[key] = s.now.Add(expiration)
	return redis.NewBoolResult(true, nil)
}

func (s *fakeRateLimitStore) TTL(_ context.Context, key string) *redis.DurationCmd {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireIfDueLocked(key)
	if deadline, ok := s.expiry[key]; ok {
		return redis.NewDurationResult(deadline.Sub(s.now), nil)
	}
	return redis.NewDurationResult(-1, nil)
}

// errRateLimitStore mensimulasikan Redis yang tidak tersedia.
type errRateLimitStore struct{}

var errRedisDown = errors.New("redis down (simulasi)")

func (errRateLimitStore) Incr(context.Context, string) *redis.IntCmd {
	return redis.NewIntResult(0, errRedisDown)
}

func (errRateLimitStore) Expire(context.Context, string, time.Duration) *redis.BoolCmd {
	return redis.NewBoolResult(false, errRedisDown)
}

func (errRateLimitStore) TTL(context.Context, string) *redis.DurationCmd {
	return redis.NewDurationResult(0, errRedisDown)
}

func newRateLimitEngine(limiter gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", limiter, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func doRateLimitRequest(r *gin.Engine) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(rec, req)
	return rec
}

func TestRateLimitExceededReturns429WithRetryAfter(t *testing.T) {
	store := newFakeRateLimitStore()
	r := newRateLimitEngine(RateLimitByIP(store, "t-exceed", 3))

	for i := 1; i <= 3; i++ {
		if rec := doRateLimitRequest(r); rec.Code != http.StatusOK {
			t.Fatalf("request #%d status = %d, want 200; body = %s", i, rec.Code, rec.Body.String())
		}
	}
	rec := doRateLimitRequest(r)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("request #4 status = %d, want 429; body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal(429 body) error: %v", err)
	}
	if body.Error != "terlalu banyak permintaan, coba lagi nanti" {
		t.Errorf("429 error = %q, want pesan kontrak", body.Error)
	}
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil {
		t.Fatalf("Retry-After %q bukan angka: %v", rec.Header().Get("Retry-After"), err)
	}
	if retryAfter < 1 || retryAfter > 60 {
		t.Errorf("Retry-After = %d, want 1..60 detik", retryAfter)
	}
}

func TestRateLimitWindowReset(t *testing.T) {
	store := newFakeRateLimitStore()
	r := newRateLimitEngine(RateLimitByIP(store, "t-reset", 1))

	if rec := doRateLimitRequest(r); rec.Code != http.StatusOK {
		t.Fatalf("request pertama status = %d, want 200", rec.Code)
	}
	if rec := doRateLimitRequest(r); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("request kedua status = %d, want 429", rec.Code)
	}

	store.advance(61 * time.Second)
	if rec := doRateLimitRequest(r); rec.Code != http.StatusOK {
		t.Errorf("request setelah window berlalu status = %d, want 200 (reset)", rec.Code)
	}
}

func TestRateLimitFailOpenWhenRedisDown(t *testing.T) {
	r := newRateLimitEngine(RateLimitByIP(errRateLimitStore{}, "t-failopen", 1))
	for i := 1; i <= 5; i++ {
		if rec := doRateLimitRequest(r); rec.Code != http.StatusOK {
			t.Fatalf("fail-open request #%d status = %d, want 200", i, rec.Code)
		}
	}
}

func TestRateLimitDisabled(t *testing.T) {
	// perMinute 0 = nonaktif; store nil juga tidak boleh membatasi.
	for name, limiter := range map[string]gin.HandlerFunc{
		"limit_nol": RateLimitByIP(newFakeRateLimitStore(), "t-off", 0),
		"store_nil": RateLimitByIP(nil, "t-nil", 10),
	} {
		r := newRateLimitEngine(limiter)
		for i := 1; i <= 20; i++ {
			if rec := doRateLimitRequest(r); rec.Code != http.StatusOK {
				t.Fatalf("%s: request #%d status = %d, want 200", name, i, rec.Code)
			}
		}
	}
}

func TestRateLimitByUserSeparateKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := newFakeRateLimitStore()
	r := gin.New()
	// Middleware pengganti RequireAuth: user id diambil dari header uji.
	r.GET("/x", func(c *gin.Context) {
		c.Set(CtxUserID, c.GetHeader("X-Test-User"))
		c.Next()
	}, RateLimitByUser(store, "t-user", 1), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	do := func(user string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("X-Test-User", user)
		r.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := do("user-a"); code != http.StatusOK {
		t.Fatalf("user-a #1 status = %d, want 200", code)
	}
	if code := do("user-a"); code != http.StatusTooManyRequests {
		t.Errorf("user-a #2 status = %d, want 429 (limit per user)", code)
	}
	if code := do("user-b"); code != http.StatusOK {
		t.Errorf("user-b #1 status = %d, want 200 (kunci terpisah per user)", code)
	}
}

// TestRateLimitIPIgnoresSpoofedXFFWithoutTrustedProxies membuktikan resolusi
// temuan QA E10-3 (XFF bypass): dengan SetTrustedProxies(nil) — wiring
// router saat TRUSTED_PROXIES kosong — dua request dari koneksi yang sama
// dengan X-Forwarded-For berbeda tetap dihitung pada SATU kunci IP
// (RemoteAddr); header spoof diabaikan.
func TestRateLimitIPIgnoresSpoofedXFFWithoutTrustedProxies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := newFakeRateLimitStore()
	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		t.Fatalf("SetTrustedProxies(nil) error: %v", err)
	}
	r.GET("/x", RateLimitByIP(store, "t-xff", 1), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	do := func(spoofedIP string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil) // RemoteAddr sama (192.0.2.1)
		req.Header.Set("X-Forwarded-For", spoofedIP)
		r.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := do("203.0.113.7"); code != http.StatusOK {
		t.Fatalf("request #1 (XFF 203.0.113.7) status = %d, want 200", code)
	}
	if code := do("203.0.113.8"); code != http.StatusTooManyRequests {
		t.Errorf("request #2 (XFF beda, koneksi sama) status = %d, want 429 — XFF spoof harus diabaikan, kunci = RemoteAddr", code)
	}
}

// TestRateLimitIPHonorsXFFFromTrustedProxy mendokumentasikan mode produksi:
// bila alamat koneksi termasuk TRUSTED_PROXIES, ClientIP() memakai XFF
// sehingga tiap klien asli mendapat kunci sendiri.
func TestRateLimitIPHonorsXFFFromTrustedProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := newFakeRateLimitStore()
	r := gin.New()
	if err := r.SetTrustedProxies([]string{"192.0.2.0/24"}); err != nil {
		t.Fatalf("SetTrustedProxies(192.0.2.0/24) error: %v", err)
	}
	r.GET("/x", RateLimitByIP(store, "t-xff-trusted", 1), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	do := func(clientIP string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil) // RemoteAddr 192.0.2.1 = proxy dipercaya
		req.Header.Set("X-Forwarded-For", clientIP)
		r.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := do("203.0.113.7"); code != http.StatusOK {
		t.Fatalf("klien A #1 status = %d, want 200", code)
	}
	if code := do("203.0.113.8"); code != http.StatusOK {
		t.Errorf("klien B #1 status = %d, want 200 (kunci per IP asli dari XFF)", code)
	}
	if code := do("203.0.113.7"); code != http.StatusTooManyRequests {
		t.Errorf("klien A #2 status = %d, want 429", code)
	}
}

// TestRateLimitLoginFixedWindow_Integration memakai Redis nyata (fixed window
// INCR+EXPIRE) meniru wiring /auth/login: 15 percobaan pertama diteruskan ke
// handler (401 kredensial salah), percobaan ke-16 mendapat 429 + Retry-After.
func TestRateLimitLoginFixedWindow_Integration(t *testing.T) {
	addr := os.Getenv("EOFFICE_INTEGRATION_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis %s tidak tersedia (%v); jalankan docker compose untuk test integrasi rate limit", addr, err)
	}

	scope := fmt.Sprintf("auth-it-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		// Kunci rate limit boleh dibersihkan (bukan audit_logs).
		_ = client.Del(context.Background(), "ratelimit:"+scope+":192.0.2.1").Err()
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/auth/login", RateLimitByIP(client, scope, 15), func(c *gin.Context) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "kombinasi kredensial tidak dikenal"})
	})

	for i := 1; i <= 15; i++ {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("login #%d status = %d, want 401 (masih di bawah limit); body = %s", i, rec.Code, rec.Body.String())
		}
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("login #16 status = %d, want 429; body = %s", rec.Code, rec.Body.String())
	}
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || retryAfter < 1 || retryAfter > 60 {
		t.Errorf("Retry-After = %q, want angka 1..60 detik (err %v)", rec.Header().Get("Retry-After"), err)
	}
}
