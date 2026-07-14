# E10-3 — Rate limiting, security headers, CSRF, dependency audit, path-id 404

Status: verified

Tujuan:

Hardening lapisan HTTP API sebelum pilot (backlog E10-3), plus pembersihan
temuan Low laporan uji E03: path id non-UUID → 500 generik.

Keputusan desain (Lead, 14 Jul 2026):

- **CSRF: tidak berlaku by design.** Autentikasi memakai Bearer token di header
  `Authorization` (web menyimpan token di memori/storage, bukan cookie; tidak
  ada kredensial ambient). Browser tidak mengirim header Authorization lintas
  origin tanpa CORS allow. Wajib DIDOKUMENTASIKAN (komentar di router +
  bagian keamanan di handoff), bukan diimplementasikan.
- **Rate limiting: Redis fixed-window, fail-open.** Bila Redis error, request
  diloloskan (availability > strictness) dengan log peringatan. Kunci:
  `ratelimit:{scope}:{ip|user}`. `INCR` + `EXPIRE` (set TTL hanya saat count==1).
- **Security headers untuk API JSON**, bukan halaman HTML.

Perilaku/acceptance criteria:

1. **Rate limiting** (middleware baru `backend/internal/middleware/ratelimit.go`):
   - Endpoint auth publik (`/auth/login`, `/auth/forgot-password`,
     `/auth/reset-password`, `/auth/refresh`): 15 request/menit per IP
     (default; env `RATE_LIMIT_AUTH_PER_MINUTE`, 0 = nonaktif).
   - Endpoint terautentikasi: 300 request/menit per user (env
     `RATE_LIMIT_API_PER_MINUTE`, 0 = nonaktif). Kunci per `CtxUserID`.
   - Saat limit terlampaui: 429 `{"error":"terlalu banyak permintaan, coba lagi nanti"}`
     + header `Retry-After` (detik sisa window).
   - Fail-open saat Redis tidak tersedia; jangan menjatuhkan request.
   - IP klien: `c.ClientIP()` (gin sudah menangani X-Forwarded-For sesuai
     trusted proxies default).
2. **Security headers** (middleware baru, dipasang global di router):
   - `X-Content-Type-Options: nosniff`
   - `X-Frame-Options: DENY`
   - `Referrer-Policy: no-referrer`
   - `Cache-Control: no-store` untuk path `/api/` (respons berisi data
     organisasi sensitif; JANGAN memasang no-store pada unduhan streaming bila
     mengganggu — boleh seragam no-store karena unduhan berjalan sekali).
   - `Strict-Transport-Security: max-age=31536000` hanya bila
     `APP_ENV=production` (produksi di belakang TLS).
   - `Content-Security-Policy: default-src 'none'` untuk respons API JSON.
3. **Path id non-UUID → 404** (middleware baru atau helper di router):
   - Untuk seluruh route terautentikasi, param path bernama `id` atau berakhiran
     `_id` yang bukan UUID valid → 404 `{"error":"data tidak ditemukan"}`
     sebelum menyentuh handler/DB. Param non-UUID yang sah (`:scope` pada
     coordination rules; `:token` pada verify publik) TIDAK terdampak
     (validasi hanya nama `id`/`*_id`, dan `:token` ada di grup publik).
   - Menutup temuan Low laporan E03 (500 pada `DELETE /delegations/:id` dan
     `POST /letters/view/:id/cancel` dengan id non-UUID) secara lintas endpoint.
4. **Dependency audit di CI** (owner: Lead, file `.github/workflows/ci.yml`):
   - Job `security`: `govulncheck ./...` (backend) dan `npm audit --omit=dev
     --audit-level=high` (web). Gagal = blokir merge.
5. Konfigurasi baru dibaca di `internal/config/config.go` dengan default aman;
   `.env` dev TIDAK perlu diubah (default aktif).

Scope owner:
- Backend: `backend/internal/middleware/` (file baru), `internal/server/router.go`,
  `internal/config/config.go`, test middleware (unit + integrasi ringan).
- Web: tidak ada (verifikasi manual bahwa web dev tetap berfungsi dengan
  header baru — tidak ada perubahan kode).
- Mobile: tidak ada.
- QA: verifikasi limit, headers, 404 non-UUID, fail-open Redis mati.
- Lead: `.github/workflows/ci.yml` (job dependency audit).

Di luar scope:

- WAF/proxy produksi, TLS termination (E11/deploy).
- Lockout akun login 5x (sudah ada sejak E01; rate limit IP adalah lapisan
  tambahan, bukan pengganti).

Kontrak data/API:

- Error baru: 429 dengan body `{"error":"terlalu banyak permintaan, coba lagi nanti"}`
  dan header `Retry-After: <detik>`. Konsumen (web/mobile) sudah menampilkan
  `error` dari body secara generik — tidak ada perubahan kontrak lain.
- 404 `{"error":"data tidak ditemukan"}` untuk path id non-UUID (sebelumnya 500).

Authorization dan company scope:

Tidak ada perubahan jalur otorisasi. Rate limit per-user memakai `CtxUserID`
setelah `RequireAuth`.

Dependency/order:

Menyentuh `router.go` — jangan digabung dengan task lain yang menyentuh file
itu pada gelombang yang sama (E10-1 sudah dikontrak untuk tidak menyentuhnya).

Verification:

- Unit test middleware (limit terlampaui → 429 + Retry-After; reset window;
  fail-open saat Redis nil/error; UUID validator: valid lolos, non-UUID → 404,
  `:scope` tidak terdampak).
- Integrasi: login 16x beruntun → ke-16 mendapat 429 (atau sesuai default).
- `go test ./...`, `go vet ./...` hijau.
- Manual: `curl -i` cek header keamanan muncul; web dev login tetap jalan.

Handoff yang diwajibkan: ringkasan, file diubah, konfigurasi baru + default,
hasil test, risiko (termasuk catatan CSRF-not-applicable).

## Handoff — Backend

Ringkasan perilaku (selesai, 14 Jul 2026):

1. Rate limiting Redis fixed-window (`INCR` + `EXPIRE` saat count==1), kunci
   `ratelimit:{scope}:{ip|user}`, window 60 detik, fail-open dengan log
   peringatan saat Redis error:
   - Scope `auth` per IP (`c.ClientIP()`): dipasang pada `POST /auth/login`,
     `/auth/refresh`, `/auth/forgot-password`, `/auth/reset-password`.
     `/auth/logout` dan `GET /verify/:token` tidak dibatasi.
   - Scope `api` per user (`CtxUserID`, fallback IP): dipasang pada seluruh
     grup terautentikasi setelah `RequireAuth`.
   - Limit terlampaui → 429 `{"error":"terlalu banyak permintaan, coba lagi nanti"}`
     + header `Retry-After` (detik sisa window, dibulatkan ke atas, min 1).
     Bila kunci kehilangan TTL (EXPIRE awal gagal), TTL dipasang ulang agar
     window tidak menolak permanen.
2. Security headers global (`middleware.SecurityHeaders`): `X-Content-Type-
   Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`,
   `Content-Security-Policy: default-src 'none'` di semua respons;
   `Cache-Control: no-store` untuk path `/api/` (termasuk unduhan streaming,
   seragam); `Strict-Transport-Security: max-age=31536000` hanya saat
   `APP_ENV=production`.
3. Path id non-UUID → 404 (`middleware.ValidateUUIDPathParams` pada grup
   terautentikasi): param `id` atau `*_id` yang bukan UUID kanonis 8-4-4-4-12
   (case-insensitive) → 404 `{"error":"data tidak ditemukan"}` sebelum
   handler/DB. `:scope` (coordination rules) dan `:token` (verify publik)
   tidak terdampak. Menutup temuan Low E03 lintas endpoint.
4. CSRF: TIDAK berlaku by design — autentikasi memakai Bearer token pada
   header `Authorization` (token di memori/storage web, bukan cookie), tidak
   ada kredensial ambient yang dikirim otomatis lintas origin, dan browser
   tidak menyertakan header Authorization tanpa CORS allow. Didokumentasikan
   sebagai komentar di `router.go`; tidak ada middleware CSRF.
5. Dependency audit CI: TIDAK dikerjakan di sini — owner Lead
   (`.github/workflows/ci.yml`), belum diverifikasi oleh Backend.
6. **Resolusi temuan QA Medium (XFF bypass, 14 Jul 2026):** default gin
   mempercayai semua proxy sehingga klien langsung bisa memalsukan
   `X-Forwarded-For` dan mem-bypass rate limit per-IP. Diperbaiki: router kini
   memanggil `r.SetTrustedProxies(...)` dari konfigurasi `TRUSTED_PROXIES` —
   kosong (default) = `SetTrustedProxies(nil)` = tidak mempercayai proxy mana
   pun, sehingga `ClientIP()` = alamat koneksi langsung (RemoteAddr) dan XFF
   diabaikan; benar untuk dev direct-connection dan aman sebagai default.
   Nilai `TRUSTED_PROXIES` tidak valid → `log.Fatal` saat startup.

Konfigurasi baru (dibaca di `internal/config/config.go`, default aktif —
`.env` dev tidak perlu diubah):

- `RATE_LIMIT_AUTH_PER_MINUTE` (default 15, 0 = nonaktif).
- `RATE_LIMIT_API_PER_MINUTE` (default 300, 0 = nonaktif).
- `TRUSTED_PROXIES` (default KOSONG = tidak percaya proxy mana pun; produksi
  di belakang reverse proxy mengisi CIDR/IP proxy dipisah koma, mis.
  `10.0.0.0/8` — barulah `ClientIP()` membaca X-Forwarded-For).

File diubah:

- `backend/internal/middleware/ratelimit.go` (baru) — `RateLimitByIP`,
  `RateLimitByUser`, interface `RateLimitStore` (dipenuhi `*redis.Client`).
- `backend/internal/middleware/security_headers.go` (baru).
- `backend/internal/middleware/uuid_params.go` (baru).
- `backend/internal/middleware/ratelimit_test.go`,
  `security_headers_test.go`, `uuid_params_test.go` (baru).
- `backend/internal/server/router.go` — pasang ketiga middleware + komentar
  keputusan CSRF + `SetTrustedProxies` dari `TRUSTED_PROXIES` (temuan QA XFF).
- `backend/internal/config/config.go` — dua knob rate limit +
  `TRUSTED_PROXIES` (helper `getEnvList`).

Kontrak: error baru 429 + `Retry-After` (body `error` generik, konsumen tidak
perlu berubah); 404 `{"error":"data tidak ditemukan"}` untuk path id non-UUID
(sebelumnya 500). Tidak ada perubahan jalur otorisasi.

Test yang dijalankan (14 Jul 2026):

- Unit middleware (`go test ./internal/middleware/ -count=1 -v`) — PASS
  semua: limit terlampaui → 429 + Retry-After 1..60 + pesan kontrak; reset
  window; fail-open saat Redis error (5x lolos); nonaktif saat limit 0 atau
  store nil; kunci per-user terpisah; headers lengkap di `/api/`, no-store
  tidak dipasang di `/healthz`, HSTS hanya production; UUID valid/huruf besar
  lolos, non-UUID (`id`, `_id`) → 404 dengan body kontrak, `:scope` lolos.
- Integrasi Redis nyata (`TestRateLimitLoginFixedWindow_Integration`,
  localhost:6379): login 15x → 401, ke-16 → 429 + Retry-After — PASS (tidak
  skip; skip otomatis hanya bila Redis mati).
- Temuan QA XFF: `TestRateLimitIPIgnoresSpoofedXFFWithoutTrustedProxies`
  (trusted proxies nil → dua request koneksi sama dengan XFF berbeda dihitung
  SATU kunci IP, request kedua 429) dan
  `TestRateLimitIPHonorsXFFFromTrustedProxy` (proxy dipercaya → kunci per IP
  asli dari XFF) — keduanya PASS.
- `go vet ./...` bersih; `go test ./... -count=1` penuh dengan
  `EOFFICE_INTEGRATION_DB_URL` — 94 top-level test PASS, 0 FAIL, 0 SKIP
  (rerun setelah perbaikan XFF).
- Manual terhadap server berjalan (go run + curl): semua header keamanan
  muncul (termasuk `Cache-Control: no-store` di `/api/`, tanpa HSTS di dev);
  login salah 15x → 401 lalu ke-16 → 429 `Retry-After: 60` dengan body sesuai
  kontrak. Login web dev end-to-end TIDAK saya verifikasi (tidak ada perubahan
  kode web; QA memverifikasi).

Risiko/asumsi:

- Fixed window memungkinkan burst 2x limit di perbatasan window — diterima
  by design (sederhana, cukup untuk pilot).
- Fail-open berarti Redis mati = tanpa pembatasan; lockout akun login 5x
  (E01) tetap menjadi lapisan proteksi kredensial.
- Rate limit auth per IP: klien di belakang NAT/proxy kantor berbagi kuota
  15/menit gabungan untuk login+refresh+forgot+reset; naikkan lewat env bila
  pilot terganggu.
- Trusted proxies (temuan QA XFF): SUDAH ditutup — default tidak mempercayai
  proxy mana pun. Konsekuensi operasional: saat produksi berjalan di belakang
  reverse proxy, `TRUSTED_PROXIES` WAJIB diisi CIDR proxy; bila tidak, semua
  klien terlihat sebagai IP proxy dan berbagi satu kuota auth per menit
  (fail-closed yang aman, bukan bypass).
- Validasi UUID hanya pada grup terautentikasi (semua route `:id`/`:*_id`
  saat ini ada di sana); route publik `:token` tidak berubah.

## Handoff — QA

Verifikasi adversarial 14 Jul 2026 (laporan lengkap:
`docs/LAPORAN-UJI-E10-1-E10-3.md`). Semua AC berfungsi live; regression gate
hijau.

Defect terkonfirmasi & DIPERBAIKI:
- **[Medium → DIPERBAIKI] Bypass rate limit login via `X-Forwarded-For`.**
  Temuan awal (gin percaya semua proxy → XFF palsu bypass): reproduksi live
  20 login XFF berbeda → 0/20 kena 429, 20 kunci `ratelimit:auth:<palsu>`.
  Backend memperbaiki via `SetTrustedProxies(cfg.TrustedProxies)` + config
  `TRUSTED_PROXIES` (default kosong → nil → `ClientIP()`=RemoteAddr, XFF
  diabaikan). **QA re-probe live identik: req 1–15 → 401, req 16–20 → 429
  (5× 429, ambang di ke-16), `Retry-After` hadir, hanya 1 kunci
  `ratelimit:auth:::1` di Redis.** Ditutup.
  Catatan operasional: di produksi di belakang proxy, `TRUSTED_PROXIES` WAJIB
  diisi CIDR proxy; bila kosong, kuota auth per-IP jadi global (fail-closed
  aman, bukan bypass).

Risiko sisa (bukan blocker):
- **[Low] Latency saat Redis mati.** Fail-open benar (request dilayani) tetapi
  tiap request menunggu ~1,4 dtk (retry dial go-redis 5x). Pertimbangkan dial
  timeout lebih pendek untuk pilot.

Verifikasi yang lolos (probe live + unit/integrasi):
- Header keamanan lengkap pada `/api/v1`; `no-store` absen di `/healthz`; HSTS
  hanya production; CORS dev tidak rusak (preflight 204 + origin di-echo).
- Login 15x → 401, ke-16 → 429 + `Retry-After: 59` + body kontrak.
- Fail-open: `docker stop eoffice-redis` → authed 200 & login 401 (dilayani,
  log peringatan); `docker start` → PONG, authed 200.
- Path id non-UUID (`id`, `attachment_id`, `DELETE /delegations/:id`,
  `POST /letters/view/:id/cancel`) → 404 `{"error":"data tidak ditemukan"}`,
  bukan 500 (menutup temuan Low E03). UUID valid-tapi-tak-ada → 404 handler.
- Rate limit per-user tak bisa dipalsukan via header (memakai `CtxUserID` dari
  JWT tervalidasi).

Regression gate: `go vet ./...` bersih; `go test ./... -count=1` (DB+Redis) —
`handler` & `middleware` `ok`, 0 FAIL, 0 SKIP; integrasi Redis & DB benar-benar
berjalan (bukan skip).
