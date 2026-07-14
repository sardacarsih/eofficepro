# Laporan Uji E10-1 (Enforcement Klasifikasi + Audit Akses Ditolak) dan E10-3 (Hardening API)

**Tanggal pengujian:** 14 Juli 2026

**Environment:** database development lokal (Postgres 16, port 5433) + Redis 6379
+ MinIO + ClamAV via Docker (semua healthy). API live dijalankan dari
`go run ./cmd/api` (port 8080, `APP_ENV=development`).

**Metode:** review kode adversarial, test integrasi Go dengan DB nyata
(`EOFFICE_INTEGRATION_DB_URL` di-set, TIDAK ada skip), test unit + integrasi
middleware dengan Redis nyata, plus probe HTTP langsung ke server dev memakai
akun seed (`admin@ksk.local`). Perubahan belum di-commit (working tree).

## Ringkasan

Seluruh acceptance criteria kedua ticket terverifikasi secara independen (bukan
sekadar percaya handoff). Enforcement klasifikasi dan pencatatan `access_denied`
E10-1 benar, konsisten lintas jalur akses surat, dan append-only. Hardening API
E10-3 (rate limit, security headers, 404 non-UUID, fail-open) berfungsi live.

Satu temuan keamanan **Medium** dikonfirmasi live pada putaran pertama: rate
limit login per-IP dapat di-bypass total dengan memalsukan header
`X-Forwarded-For` karena gin memercayai semua proxy (`SetTrustedProxies`
default). Temuan ini **sudah diperbaiki Backend** (config `TRUSTED_PROXIES`,
default kosong → `SetTrustedProxies(nil)` → `ClientIP()` = RemoteAddr, XFF
diabaikan) dan **diverifikasi ulang QA lewat re-probe live yang identik**: kini
XFF palsu tidak lagi memecah kuota (lihat temuan #1). Tidak ada defect yang
memblokir acceptance criteria; regression gate hijau.

Catatan operasional produksi (WAJIB): saat aplikasi berjalan di belakang
reverse proxy/WAF (E11), `TRUSTED_PROXIES` **harus** diisi CIDR/IP proxy agar
`ClientIP()` membaca IP klien asli dari `X-Forwarded-For`. Bila dibiarkan
kosong di produksi, seluruh trafik terlihat berasal dari IP proxy dan **berbagi
satu kuota `ratelimit:auth` per menit** — ini fail-closed yang aman (bukan
bypass), tetapi bisa menolak pengguna sah secara massal. Jadi: default kosong
aman untuk dev direct-connection, wajib diisi sebelum ada proxy di depannya.

## Matriks skenario — E10-1 Enforcement Klasifikasi

| Skenario | Bukti | Hasil |
|---|---|---|
| Rahasia: penerima To → detail 200 | `TestClassificationRahasiaMatrix_Integration` | PASS |
| Rahasia: approver → detail 200 | idem | PASS |
| Rahasia: penerima CC → detail 404 + audit `via=detail` | idem | PASS |
| Rahasia: penerima CC → unduh PDF 404 + audit `via=download_pdf` | idem | PASS |
| Rahasia: penerima CC → unduh lampiran 404 + audit `via=download_attachment` | idem | PASS |
| Rahasia: penerima disposisi → detail 404 + audit (disposisi TIDAK memberi akses) | idem | PASS |
| Rahasia: `userCanDownloadLetter` To/approver=true, CC/disposisi=false | idem | PASS |
| Terbatas & Biasa: penerima To/CC → 200 | `TestClassificationTerbatasDanBiasaMatrix_Integration` | PASS |
| Terbatas & Biasa: se-company tanpa hubungan → 404 + audit (detail & download_pdf) | idem | PASS |
| Terbatas & Biasa: user company lain → 404 + audit (lintas company) | idem | PASS |
| Surat tidak ada → 404 TANPA baris audit (anti-spam) | `TestClassificationMissingLetterNotAudited_Integration` | PASS |
| **Probe HTTP end-to-end**: admin (super_admin, tanpa hubungan) → detail & final-pdf pada surat nyata `in_approval` | 404 + 2 baris `access_denied` (`via=detail`, `via=download_pdf`), IP `::1` tercatat, actor=admin | PASS |

Konsistensi jalur akses (review kode): `publishedRecipientAccessSQL`
(letter_inbox.go, aturan `rahasia => hanya recipient_type='to'`) dipakai identik
di **detail** (`userCanViewLetter`), **inbox** (`GetInbox` + `letterIn*Access`),
**search** (`letter_search.go`), **dashboard** (`dashboard.go`), dan **download**
(`downloadBaseAccessSQL`/`userCanDownloadLetter`). **Komentar surat**
(`letter_comment.go`, GET & POST) memakai `userCanViewLetter`, jadi ikut
ter-enforce. Disposisi eksplisit dikecualikan untuk rahasia
(`AND l.classification <> 'rahasia'`) baik di view maupun akses lain. Path
auditor (`auditLetterAccessSQL`) dibatasi `max_classification`. Tidak ditemukan
jalur baca surat yang lupa di-enforce.

Audit `access_denied`: helper `auditLetterAccessDenied` memakai satu
`INSERT ... SELECT FROM letters WHERE id=$1` (INSERT-only, tidak ada
UPDATE/DELETE `audit_logs` di kode produk — satu-satunya DELETE `audit_logs`
ada di cleanup fixture test `user_position_test.go`). Kolom INSERT
(`entity_type, entity_id, action, actor_user_id, detail, ip_address`) cocok
dengan skema `0001_init.up.sql`. Karena bentuk INSERT..SELECT, id yang tidak ada
tidak menulis baris — terbukti oleh test + probe.

## Matriks skenario — E10-3 Hardening API

| Skenario | Bukti | Hasil |
|---|---|---|
| Header keamanan pada `/api/v1` (nosniff, X-Frame DENY, Referrer no-referrer, CSP none, Cache-Control no-store) | probe `curl -i` login endpoint | PASS |
| CORS dev tidak rusak: preflight OPTIONS `Origin: localhost:3000` → 204 + `Access-Control-Allow-Origin` di-echo | probe `curl -i -X OPTIONS` | PASS |
| `Cache-Control: no-store` TIDAK dipasang pada non-`/api/` (`/healthz`) | probe `curl -i /healthz` | PASS |
| HSTS hanya production (tidak muncul di dev) | probe + `TestSecurityHeadersHSTSOnlyInProduction` | PASS |
| Login salah 15x → 401, ke-16 → 429 + `Retry-After: 59` + body kontrak | probe 16x same IP + `TestRateLimitLoginFixedWindow_Integration` | PASS |
| Rate limit per-user memakai `CtxUserID`, kunci terpisah | `TestRateLimitByUserSeparateKeys` | PASS |
| Fail-open Redis mati: authed `/auth/me` → 200, login → 401 (tetap dilayani), log peringatan | probe `docker stop eoffice-redis` | PASS |
| Redis dinyalakan lagi → PONG, `/auth/me` → 200 | probe `docker start eoffice-redis` | PASS |
| Path id non-UUID (`abc`, `xyz`, `zzzz-...`, 36-char non-hex) → 404 `{"error":"data tidak ditemukan"}` | probe authed + `TestValidateUUIDPathParams` | PASS |
| Non-UUID `attachment_id`, `DELETE /delegations/abc`, `POST /letters/view/abc/cancel` (temuan Low E03) → 404 bukan 500 | probe authed | PASS |
| UUID valid tapi tidak ada → 404 dari handler (bukan 500) | probe authed | PASS |
| Reset window setelah 60 dtk; nonaktif saat limit=0/store nil | `TestRateLimitWindowReset`, `TestRateLimitDisabled` | PASS |

## Temuan

| # | Severity | Temuan | Status |
|---|---|---|---|
| 1 | **Medium** | **Rate limit login per-IP bisa di-bypass total via `X-Forwarded-For` spoofing.** gin (sebelum fix) `SetTrustedProxies` default (percaya semua proxy), sehingga `c.ClientIP()` memakai header XFF yang dikontrol klien. Reproduksi live awal: 20 request login dengan `X-Forwarded-For: 203.0.113.<i>` berbeda-beda → **0 dari 20 mendapat 429**, 20 kunci `ratelimit:auth:<ip-palsu>` terbentuk di Redis. Praktis meniadakan proteksi brute-force login. | **DIPERBAIKI** (Backend, 14 Jul 2026) & **diverifikasi ulang QA.** Router kini `r.SetTrustedProxies(cfg.TrustedProxies)`; config baru `TRUSTED_PROXIES` default kosong → `SetTrustedProxies(nil)` → `ClientIP()` = RemoteAddr, XFF diabaikan (nilai tak valid → `log.Fatal` saat startup). **Bukti re-probe live (env identik, 20 login gagal dengan XFF `203.0.113.1..20` berbeda dari koneksi yang sama):** req 1–15 → 401, req **16–20 → 429** (5× 429, ambang tepat di ke-16), `Retry-After` hadir (mis. 42); **hanya 1 kunci** `ratelimit:auth:::1` (RemoteAddr) terbentuk di Redis — XFF palsu tidak lagi memecah kuota. Unit test baru `TestRateLimitIPIgnoresSpoofedXFFWithoutTrustedProxies` & `TestRateLimitIPHonorsXFFFromTrustedProxy` lulus. |
| 2 | Low | **Latency saat Redis benar-benar mati.** Fail-open bekerja (request dilayani), tetapi tiap request yang melewati limiter menunggu ~1,4 dtk (5x percobaan dial go-redis) sebelum diloloskan. Selama outage Redis, seluruh endpoint auth + terautentikasi mendapat tambahan latency ini. | Dilaporkan — bukan cacat keamanan; pertimbangkan dial timeout/circuit-breaker lebih pendek. Diterima untuk pilot. |
| 3 | Info | Denial pada endpoint selain detail+unduh (mis. komentar surat, disposisi) TIDAK diaudit `access_denied`. | Sesuai batas AC E10-1 (hanya detail + unduh). Enforcement-nya tetap benar (404); hanya pencatatannya yang belum. Kandidat perluasan terpisah. |
| 4 | Info | Middleware order: `RateLimitByUser` berjalan sebelum `ValidateUUIDPathParams`, jadi request id non-UUID tetap mengonsumsi 1 token rate limit sebelum di-404. | Tidak berdampak keamanan; catatan saja. |
| 5 | Info | Bump dependency `quic-go` v0.59.0 → v0.59.1 (indirect) + `toolchain go1.25.12` di `go.mod`/`go.sum`, sejalan dengan penambahan job `govulncheck` di CI. | Benign; wajar sebagai bagian dependency audit. |

Tidak ditemukan: bypass authorization lintas company, kebocoran rahasia via
CC/disposisi/komentar/search/dashboard, UPDATE/DELETE `audit_logs`, header
keamanan yang dipasang setelah abort, atau kunci rate limit per-user yang bisa
dipalsukan (per-user memakai `CtxUserID` dari JWT tervalidasi, bukan header).

## Regression gate

| Perintah | Hasil |
|---|---|
| `go vet ./...` | Bersih (exit 0) |
| `go test ./internal/handler/ -run TestClassification -count=1 -v` (DB nyata) | PASS — 4 test, 0 skip |
| `go test ./internal/middleware/ -count=1 -v` (Redis nyata) | PASS — semua unit + `TestRateLimitLoginFixedWindow_Integration` (tidak skip) |
| `go test ./... -count=1` (dengan `EOFFICE_INTEGRATION_DB_URL` + Redis) | Hijau — paket `internal/handler` & `internal/middleware` `ok`, **0 FAIL, 0 SKIP** |

Integrasi benar-benar berjalan (bukan skip diam-diam): output verbose
menunjukkan test DB & Redis RUN→PASS, dan run penuh tidak memuat baris `SKIP`.

## Risiko tersisa / belum diverifikasi

- **Bypass XFF (temuan #1) SUDAH ditutup** (default tidak percaya proxy).
  Sisa aksi bukan bug melainkan operasional: saat E11 memasang reverse proxy,
  isi `TRUSTED_PROXIES` dengan CIDR proxy — jika lupa, kuota auth per-IP
  menjadi global untuk semua klien (fail-closed, lihat catatan operasional di
  Ringkasan). Verifikasi ulang setelah topologi proxy produksi final.
- **Probe CC-rahasia unduh via HTTP murni** diverifikasi lewat test integrasi
  handler (DB nyata) + probe HTTP admin-denied end-to-end; skenario CC-spesifik
  via token penerima CC nyata tidak diulang lewat HTTP (setup token penuh),
  namun jalur kode identik dan sudah terkunci test.
- **Login web dev end-to-end** (Next.js port 3000 dengan header keamanan baru)
  TIDAK diuji di sini — tidak ada perubahan kode web; CORS preflight dev sudah
  diverifikasi lolos. Disarankan smoke test UI web sebelum rilis.
- Rate limit auth per-IP berbagi kuota gabungan login+refresh+forgot+reset
  (satu kunci `ratelimit:auth:<ip>`) — sesuai desain, tetapi klien di belakang
  NAT kantor berbagi 15/menit; naikkan via env bila pilot terganggu.
- Data uji yang tertinggal di DB dev: beberapa baris `audit_logs access_denied`
  hasil probe (append-only, tidak dihapus sesuai aturan) dan kunci rate limit
  Redis sudah dibersihkan. Proses `go run ./cmd/api` sudah dimatikan, port 8080
  bebas.
