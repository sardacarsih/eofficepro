# eOffice Pro

Sistem surat menyurat internal digital PT Kalimantan Sawit Kusuma Group —
pembuatan surat, approval berjenjang, disposisi, tracking, arsip, paperless.

📄 Dokumen produk: [PRD-eOfficePro.md](PRD-eOfficePro.md) ·
[Backlog](docs/BACKLOG.md) · [Skema DB](docs/DATABASE-SCHEMA.md) ·
[Ringkasan Eksekutif](docs/RINGKASAN-EKSEKUTIF.md)

## Stack

| Bagian | Teknologi |
|--------|-----------|
| `backend/` | Go 1.25 · Gin · pgx · golang-migrate |
| `web/` | Next.js 16 (App Router, TypeScript, Tailwind) |
| `mobile/` | Flutter (Android; butuh Flutter SDK — lihat [mobile/README.md](mobile/README.md)) |
| Data | PostgreSQL 16 · Redis 7 (cache/antrian) · MinIO (lampiran) |

## Menjalankan Development

Prasyarat: Docker Desktop, Go 1.25+, Node 22+.

```powershell
# 1. Infrastruktur (Postgres, Redis, MinIO + bucket otomatis)
docker compose up -d

# 2. Konfigurasi & migrasi database
Copy-Item .env.example backend\.env
cd backend
go run ./cmd/migrate up

# 3. API (port 8080)
go run ./cmd/api
# cek: http://localhost:8080/healthz

# 4. Web (port 3000) — terminal terpisah
cd ..\web
npm run dev
```

Konsol MinIO: http://localhost:9001 (user/pass di docker-compose.yml, khusus dev).

## Struktur

```
backend/
  cmd/api/          entrypoint HTTP API
  cmd/migrate/      runner migrasi (embed SQL)
  internal/config/  konfigurasi env
  internal/server/  router & handler (diisi per epic BACKLOG.md)
  internal/store/   koneksi Postgres, Redis, MinIO
  migrations/       skema SQL ber-versi
web/                Next.js app (inbox, composer, approval, arsip)
mobile/             Flutter app (fokus approver, offline-first)
docs/               PRD turunan: backlog, skema DB, ringkasan eksekutif
```

## Konvensi

- Setiap perubahan skema lewat file migrasi baru di `backend/migrations/`
  (format `NNNN_nama.up.sql` + `.down.sql`) — jangan mengubah migrasi lama.
- `audit_logs` append-only; jangan pernah menulis UPDATE/DELETE ke tabel itu.
- Aksi approval dari mobile wajib membawa `client_action_id` (idempotency).
- Urutan pengerjaan fitur mengikuti [docs/BACKLOG.md](docs/BACKLOG.md).
