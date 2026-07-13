# eOffice Pro Agent Rules

Instruksi ini berlaku untuk seluruh repository. Instruksi `AGENTS.md` yang
lebih dekat dengan file yang dikerjakan menambahkan aturan khusus stack.

## Product context

eOffice Pro adalah sistem surat internal KSK Group dengan alur pembuatan,
approval berjenjang, penomoran, distribusi, disposisi, pencarian, notifikasi,
dan audit. Sumber kebenaran produk dan data:

- `PRD-eOfficePro.md`
- `docs/BACKLOG.md`
- `docs/DATABASE-SCHEMA.md`

Jangan mengubah aturan bisnis hanya berdasarkan asumsi UI. Jika dokumen dan
implementasi berbeda, catat perbedaannya dan pastikan acceptance criteria
menentukan perilaku yang diinginkan.

## Repository boundaries

- `backend/`: Go HTTP API, PostgreSQL, Redis, MinIO, FCM, dan migrasi.
- `web/`: Next.js App Router untuk admin, sekretaris, dan pengguna desktop.
- `mobile/`: Flutter Android untuk pembuat, approver, dan penerima surat.
- `docs/`: spesifikasi produk, skema, backlog, dan laporan pengujian.

Owner default per area:

- Backend agent: `backend/` dan bagian backend/data `docs/DATABASE-SCHEMA.md`.
- Web agent: `web/src/` dan `web/scripts/`.
- Mobile agent: `mobile/lib/` dan `mobile/test/`.
- QA agent: laporan pengujian `docs/LAPORAN-UJI-*.md`.
- Lead: area yang tidak disebut di atas — `docs/` umum (PRD, backlog,
  `docs/tasks/`), file konfigurasi root (`docker-compose.yml`, `.gitignore`),
  serta `AGENTS.md`, `.agents/`, dan `.claude/agents/`.

Satu file hanya boleh memiliki satu owner agent dalam satu gelombang kerja.
File hub yang disentuh banyak task (contoh: `web/src/lib/api.ts`,
`backend/internal/server/router.go`) hanya boleh diubah oleh satu task per
gelombang; Lead mengurutkan task lain yang membutuhkan file yang sama.
Jangan mengubah file di luar scope task kecuali perubahan itu diperlukan untuk
menjaga build atau kontrak lintas aplikasi; sebutkan perluasan scope tersebut
dalam handoff.

## Non-negotiable domain rules

- Semua akses data organisasi wajib menghormati company scope dan role.
- Authorization divalidasi di server; menyembunyikan kontrol di UI bukan
  mekanisme keamanan.
- Jangan melakukan `UPDATE` atau `DELETE` terhadap `audit_logs`.
- Riwayat dan versi surat tidak boleh ditimpa ketika revisi perlu dilacak.
- Perubahan status approval harus atomik dan mengikuti state transition yang
  diizinkan.
- Aksi approval dari mobile membawa `client_action_id` dan harus idempotent.
- Jangan commit secret, token, credential Firebase, atau konfigurasi produksi.
- Perubahan skema dibuat sebagai migrasi baru berpasangan `.up.sql` dan
  `.down.sql`; jangan mengedit migrasi lama yang telah digunakan.

## Team workflow

Role tim didefinisikan di `.agents/`: `lead.md`, `backend.md`, `web.md`,
`mobile.md`, dan `qa-security.md`. Sesi utama berperan sebagai Lead; specialist
dijalankan sebagai subagent Claude Code yang terdaftar di `.claude/agents/`
dengan nama `backend`, `web`, `mobile`, dan `qa-security`.

Lead menulis task contract sebagai file `docs/tasks/<id>-<slug>.md` (konvensi
di `docs/tasks/README.md`) sebelum pekerjaan dibagi. File task adalah satu-
satunya sumber kebenaran kontrak untuk task itu — termasuk kontrak API yang
dipakai Web dan Mobile untuk bekerja paralel. Minimal task contract berisi:

1. tujuan dan perilaku pengguna;
2. scope file/folder dan hal yang di luar scope;
3. kontrak API/data serta aturan authorization;
4. acceptance criteria dan perintah verifikasi;
5. dependency dan format handoff.

Backend menjadi owner kontrak API. Web dan Mobile boleh mulai paralel setelah
request, response, error semantics, dan authorization stabil. Jika kontrak
harus berubah, Backend memberi tahu Lead dan memperbarui bagian kontrak pada
file task sebelum consumer diperbarui.

Handoff setiap agent ditulis sebagai bagian `## Handoff — <Role>` pada file
task yang sama dan harus menyebutkan:

- ringkasan perilaku yang selesai;
- file yang diubah;
- migrasi atau perubahan kontrak;
- test/check yang dijalankan beserta hasilnya;
- risiko, asumsi, dan pekerjaan tersisa.

## Definition of done

Perubahan dianggap selesai jika:

- acceptance criteria terpenuhi;
- happy path, authorization failure, dan edge case utama diuji;
- test/lint/build yang relevan berhasil;
- kontrak API konsisten pada Backend, Web, dan Mobile yang terdampak;
- perubahan skema memiliki rollback yang masuk akal;
- dokumentasi diperbarui ketika perilaku atau kontrak berubah;
- tidak ada perubahan user lain yang ditimpa.

## Standard verification

Jalankan hanya checks yang relevan dengan scope, lalu laporkan yang tidak dapat
dijalankan.

```powershell
# Backend
cd backend
go test ./...

# Web
cd web
npm run lint
npm run build

# Mobile
cd mobile
flutter analyze
flutter test
```

