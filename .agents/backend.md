# Backend & Data Agent

## Ownership

- `backend/cmd/`
- `backend/internal/`
- `backend/migrations/`
- bagian backend/data pada `docs/DATABASE-SCHEMA.md`

## Mission

Implementasikan kontrak API dan aturan domain secara aman, transaksional, dan
tenant-aware. Backend adalah sumber kebenaran authorization dan state change.

## Working rules

- Telusuri route, middleware, handler, query, dan test terkait sebelum edit.
- Validasi identity, role, company scope, ownership, dan current state.
- Gunakan transaksi untuk perubahan yang mencakup surat, approval, nomor,
  notifikasi, outbox, atau audit secara bersamaan.
- Pertahankan idempotency untuk aksi yang dapat di-retry.
- Hindari query per-row; cek pagination dan index untuk endpoint daftar.
- Sanitasi rich text dan validasi file/lampiran pada trust boundary.
- Buat migrasi nomor berikutnya tanpa mengubah sejarah migrasi.

## Required tests

Untuk perubahan aturan bisnis, sertakan variasi yang relevan:

- happy path;
- unauthenticated/unauthorized;
- akses lintas company;
- invalid state transition;
- duplicate/retry/idempotency;
- rollback ketika salah satu write gagal.

Verifikasi minimum: `go test ./...` dari `backend/`.

## Handoff

Tulis pada bagian `## Handoff — Backend` di file task (`docs/tasks/`).
Laporkan endpoint dan contoh shape yang berubah, status/error semantics,
migrasi dan dampak rollback, test yang dijalankan, serta hal yang harus diubah
oleh Web atau Mobile.

