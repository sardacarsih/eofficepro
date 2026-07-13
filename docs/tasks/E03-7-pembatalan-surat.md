# E03-7 — Pembatalan surat oleh pembuat sebelum approval final

Status: contract-stable

Tujuan:

Pembuat surat dapat membatalkan suratnya sendiri sebelum approval final,
dengan jejak yang dapat diaudit (ref backlog E03-7, PRD P0-3). Saat ini
satu-satunya jalur ke status `cancelled` adalah reject oleh approver.

Perilaku/acceptance criteria:

- Pembuat dapat membatalkan surat miliknya ketika status
  `draft | revision | in_approval`; alasan wajib (jejak).
- Surat `approved | published | cancelled` tidak dapat dibatalkan → 409
  dengan pesan spesifik; surat orang lain / tidak ada → 404 (konsisten
  penyembunyian keberadaan surat).
- Pembatalan atomik: step `pending|waiting` menjadi `skipped`, status surat
  `cancelled`, `current_step_order` NULL, kolom jejak terisi
  (`cancelled_at`, `cancelled_by_user_id`, `cancel_reason`).
- Surat yang dibatalkan tidak pernah menerima nomor (`letter_number` tetap
  NULL). Race cancel vs approve final: tepat satu yang menang — approve dan
  cancel memakai urutan lock yang sama, baris `letters` dulu baru
  `approval_steps` (perubahan bersama dengan E03-5).
- `approval_actions` (termasuk tanda tangan) approver yang sudah bertindak
  tidak disentuh — tetap utuh untuk audit.
- Approver yang sudah approve pada cycle berjalan dan approver yang step-nya
  di-skip (termasuk delegate aktif E03-5) menerima notifikasi in-app + email
  "surat dibatalkan oleh pembuat". Jangan reuse `notifyApprovalResult`
  (copy `cancelled`-nya berkonteks reject).
- Tercatat di `audit_logs`: action `letter_cancelled`, detail
  `{"reason", "previous_status"}`.
- Detail surat mengekspos jejak pembatalan + flag `can_cancel` server-side
  untuk gate tombol UI.

Scope owner:

- Backend: migrasi `0025` (bagian kolom jejak — satu file migrasi bersama
  E03-5), handler baru `backend/internal/handler/letter_cancel.go` + test,
  helper `notifyLetterCancelledByCreator`, perubahan `letter_detail.go`
  (field jejak + `can_cancel`), route di `internal/server/router.go`,
  perubahan urutan lock `ActApprovalStep` (letters dulu, baru step) di
  `approval_workflow.go`.
- Web: `cancelLetter` di `web/src/lib/api.ts`, tombol "Batalkan surat" +
  dialog alasan + banner jejak di `web/src/app/(app)/letters/[id]/page.tsx`;
  verifikasi label status `cancelled` pada daftar surat.
- Mobile: parsing field jejak + `can_cancel` (tampilkan jejak batal pada
  detail); aksi cancel TIDAK dibuat di mobile pada ticket ini.
- QA: race cancel vs approve, idempotensi (cancel ulang → 409), notifikasi,
  audit trail, nomor tidak terbit.

Di luar scope:

- Pembatalan surat `approved`/`published` (perlu kebijakan terpisah).
- Pembatalan oleh admin/sekretaris atas nama pembuat.
- Aksi cancel dari mobile.

Kontrak data/API:

Migrasi `0025` (bagian E03-7):

```sql
ALTER TABLE letters
    ADD COLUMN cancelled_at timestamptz,
    ADD COLUMN cancelled_by_user_id uuid REFERENCES users(id),
    ADD COLUMN cancel_reason text;
```

Down: drop ketiga kolom.

Endpoint baru (grup `authed`, kepemilikan dicek in-handler):

- `POST /letters/view/:id/cancel` body `{ "reason": string }`
  - 200: `{ "letter": { "id", "status": "cancelled", "cancelled_at",
    "cancelled_by_name", "cancel_reason" } }`
  - 400: reason kosong setelah trim (maks 1000 karakter, hitung rune).
  - 404: surat tidak ada atau caller bukan pembuat.
  - 409: status bukan `draft|revision|in_approval` —
    `{"error":"surat sudah disetujui final"}` atau
    `{"error":"surat sudah dibatalkan"}`.

Urutan dalam transaksi: lock surat (`FOR UPDATE`, filter
`creator_user_id`), validasi status, skip step `pending|waiting`, update
letters + kolom jejak, notifikasi (insert `notifications` + outbox), commit,
lalu audit.

Perubahan respons endpoint existing:

- `GET /letters/view/:id`: tambah `"cancelled_at": string|null`,
  `"cancelled_by_name": string|null`, `"cancel_reason": string|null`,
  `"can_cancel": bool` (true bila caller pembuat dan status cancellable).

Authorization dan company scope:

- Hanya pembuat (`creator_user_id`) yang dapat membatalkan; bukan pemilik →
  404 (tidak membocorkan keberadaan surat). Company scope tertutup oleh
  filter kepemilikan.

Dependency/order:

1. Backend wave 1, satu gelombang dengan E03-5 (migrasi 0025 + lock bersama).
2. Web wave 2; Mobile wave 2 (minimal, parsing field).
3. QA wave 3.

Verification:

- Backend: `go test ./...`; test integrasi `letter_cancel_test.go` minimal:
  happy path per status cancellable, 404 non-pemilik/lintas company, 400
  reason kosong, 409 approved/published/cancelled, cancel ulang 409, step
  ter-skip + actions utuh, notifikasi tercipta, audit row ada, race dengan
  approve final (dua goroutine → tepat satu sukses, surat batal tanpa nomor).
  Migrasi up→down→up.
- Web: `npm run lint`, `npm run build`; uji manual batalkan surat
  `in_approval` → banner jejak, tombol hilang pada status final.
- Mobile: `flutter analyze`, `flutter test`.

Handoff yang diwajibkan: sesuai format root `AGENTS.md`, pada bagian di bawah.

## Handoff — Backend

Status: selesai (wave 1), bersama E03-5. Tanggal: 2026-07-13.

Ringkasan perilaku:

- `POST /letters/view/:id/cancel` (grup `authed`): pembuat membatalkan surat
  `draft|revision|in_approval` dengan alasan wajib (trim, maks 1000 rune).
  404 bila surat tidak ada/bukan pembuat; 409 "surat sudah dibatalkan" atau
  "surat sudah disetujui final" untuk status lain.
- Transaksi: lock baris `letters` (`FOR UPDATE`, filter `creator_user_id`) →
  validasi status → step `pending|waiting` menjadi `skipped` → update status
  `cancelled` + `current_step_order` NULL + kolom jejak → notifikasi in-app +
  outbox email → commit → audit `letter_cancelled` `{"reason",
  "previous_status"}`. `approval_actions` yang sudah ada tidak disentuh.
- Notifikasi lewat helper baru `notifyLetterCancelledByCreator` (bukan
  `notifyApprovalResult`): target = pemegang aktif posisi step cycle berjalan
  berstatus `approved|skipped` + delegate aktif (E03-5), dedup per user;
  event_type `letter_cancelled`, deep link ke detail surat.
- `GET /letters/view/:id` menambah `cancelled_at`, `cancelled_by_name`,
  `cancel_reason`, dan `can_cancel` (server-side: pembuat + status
  cancellable) untuk gate tombol UI.
- Race cancel vs approve final: `ActApprovalStep` diubah memakai urutan lock
  yang sama dengan cancel — baris `letters` dikunci lebih dulu dalam statement
  terpisah, baru step (`FOR UPDATE OF s`). Varian awal `FOR UPDATE OF s, l`
  satu statement terbukti deadlock (SQLSTATE 40P01) pada race test karena
  urutan lock dalam satu statement tidak dijamin planner; sudah diperbaiki.
  Surat yang dibatalkan tidak pernah menerima nomor.

File diubah/baru:

- Baru: `backend/internal/handler/letter_cancel.go`,
  `backend/internal/handler/letter_cancel_test.go`; migrasi
  `0025_delegation_lifecycle_and_cancel_trace` (bagian kolom jejak letters,
  satu file bersama E03-5).
- Diubah: `letter_detail.go` (field jejak + `can_cancel`),
  `approval_workflow.go` (urutan lock letters → step), `notification.go`
  (event `letter_cancelled` → section inbox), `internal/server/router.go`
  (route cancel), `docs/DATABASE-SCHEMA.md`.

Migrasi + rollback: lihat handoff E03-5 (migrasi bersama); up → down → up
terverifikasi di DB dev (versi 24 → 25 → 24 → 25).

Test yang dijalankan (Postgres dev 5433, `EOFFICE_INTEGRATION_DB_URL`):

- `TestCancelLetterInApproval_Integration` — happy path in_approval: step
  ter-skip, actions utuh, jejak terisi, notifikasi + audit tercipta — PASS.
- `TestCancelLetterPerStatusAndValidation_Integration` — cancellable per
  status draft/revision, 400 reason kosong/kepanjangan, 404 non-pemilik dan
  lintas company, 409 approved/published/cancelled + cancel ulang — PASS.
- `TestCancelLetterRaceWithApproveFinal_Integration` — 3 iterasi × `-count=3`:
  tepat satu pemenang; cancel menang → status `cancelled` tanpa nomor,
  approve menang → cancel 409 — PASS (setelah perbaikan urutan lock).
- `go test ./...` penuh: kegagalan tersisa hanya 11 test pre-existing
  company-scope (identik pada tree bersih), di luar scope.

Risiko/asumsi:

- Pembatalan `approved`/`published` dan cancel oleh admin tetap di luar scope.
- Aksi cancel tidak dibuat di mobile (sesuai kontrak); mobile hanya parsing
  field jejak.

Instruksi Web/Mobile (wave 2):

- Web: `cancelLetter(id, reason)` → `POST /letters/view/:id/cancel`; gate
  tombol pakai `can_cancel` dari detail (jangan hitung sendiri); banner jejak
  dari `cancelled_at`/`cancelled_by_name`/`cancel_reason`.
- Mobile: parsing `cancelled_at`, `cancelled_by_name`, `cancel_reason`,
  `can_cancel` pada detail surat; tampilkan jejak batal; tanpa aksi cancel.

## Handoff — Web

Status: selesai (wave 2). Tanggal: 2026-07-13.

Ringkasan perilaku:

- `cancelLetter(id, reason)` di `web/src/lib/api.ts` →
  `POST /letters/view/:id/cancel`; type `LetterDetail` diperluas dengan
  `cancelled_at`, `cancelled_by_name`, `cancel_reason`, `can_cancel`.
- Detail surat (`/letters/[id]`): tombol "Batalkan Surat" hanya dirender bila
  `can_cancel` dari server (klien tidak menghitung kelayakan sendiri). Dialog
  konfirmasi dengan alasan wajib maks 1000 karakter (hitung code point,
  konsisten validasi rune server) + counter; error server (400/404/409)
  ditampilkan di dialog apa adanya.
- Setelah sukses, detail di-refresh dari server: banner jejak "Dibatalkan
  oleh <cancelled_by_name>: <cancel_reason>" + timestamp muncul ketika
  `cancelled_at` terisi, dan tombol hilang karena `can_cancel` menjadi false.
- Label status `cancelled` pada daftar surat diverifikasi tanpa perubahan:
  "Dibatalkan" di `/inbox` (tab Terkirim), `/compose` (daftar surat saya),
  `/search`, dan header detail — sudah wajar.

File diubah:

- `web/src/lib/api.ts` (extend `LetterDetail`, type `CancelLetterResult`,
  method `cancelLetter` — satu gelombang dengan perubahan E03-5, owner
  tunggal file hub).
- `web/src/app/(app)/letters/[id]/page.tsx` (tombol + dialog alasan + banner
  jejak + refresh detail setelah sukses).

Migrasi/perubahan kontrak: tidak ada; mengikuti kontrak + handoff Backend.

Check yang dijalankan:

- `npm run lint` dari `web/` — hijau.
- `npm run build` — hijau (bersama perubahan E03-5).
- Uji manual browser belum dilakukan pada sesi ini (butuh backend hidup);
  langkah uji disarankan: buka surat `in_approval` milik sendiri → tombol
  tampil → batalkan dengan alasan → banner jejak muncul, tombol hilang,
  status "Dibatalkan"; buka surat `published`/milik orang lain → tombol
  tidak tampil; cancel ulang lewat API → pesan 409 server tampil.

Risiko/asumsi:

- Dialog tidak melakukan konfirmasi bertingkat; teks peringatan menjelaskan
  aksi tidak dapat diurungkan sebelum submit.
- Bila terjadi race dengan approve final, server mengembalikan 409 dengan
  pesan spesifik dan UI menampilkannya tanpa mengubah state lokal; pengguna
  dapat memuat ulang halaman untuk melihat status akhir.

## Handoff — Mobile

Status: selesai (wave 2, scope minimal — parsing + tampilan jejak).
Tanggal: 2026-07-13.

Ringkasan perilaku:

- `LetterDetail` mem-parse `cancelled_at`, `cancelled_by_name`,
  `cancel_reason` (semuanya nullable) dan `can_cancel` (default false) —
  backward-compatible terhadap respons lama.
- Detail surat menampilkan banner jejak pembatalan di bagian atas ketika
  `cancelled_at` terisi: "Dibatalkan oleh <nama>: <alasan>" + timestamp
  terformat. Nama/alasan kosong ditangani ("-" / tanpa alasan).
- Tidak ada aksi cancel dari mobile (sesuai kontrak, web-only); `can_cancel`
  hanya di-parse dan disimpan di model untuk kebutuhan mendatang, tidak ada
  tombol yang di-gate olehnya di mobile.

File diubah:

- `mobile/lib/features/letters/domain/letter_models.dart` — field jejak
  pembatalan + `canCancel` pada `LetterDetail`.
- `mobile/lib/features/home/presentation/home_page.dart` — widget
  `_CancellationBanner` pada `LetterDetailPane`.
- `mobile/test/model_parsing_test.dart` — test parsing jejak pembatalan +
  `can_cancel` (terisi dan default/absen).

Migrasi/kontrak: tidak ada perubahan kontrak dari sisi mobile; payload aksi
approval tidak berubah.

Check yang dijalankan (dari `mobile/`):

- `flutter analyze` — No issues found.
- `flutter test` — All tests passed (112 test, termasuk test baru).

Risiko/asumsi:

- Banner memakai `errorContainer` theme Material 3 sehingga konsisten pada
  light/dark; verifikasi visual pada perangkat fisik/emulator belum
  dilakukan (test layout existing lulus).
- Label status `cancelled` pada daftar "Terkirim" sudah ada sebelumnya
  ("Dibatalkan") — tidak diubah.

## Handoff — QA

Status: selesai (wave 3), bersama E03-5. Tanggal: 2026-07-14.
Detail lengkap: `docs/LAPORAN-UJI-E03-5-E03-7.md`.

Verifikasi yang dijalankan (Postgres dev 5433, schema v25):

- `TestCancelLetterInApproval`, `TestCancelLetterPerStatusAndValidation` PASS:
  cancellable hanya `draft|revision|in_approval`; 400/404/409 sesuai kontrak;
  cancel ulang 409; step pending/waiting → skipped; `approval_actions` utuh;
  jejak + notifikasi + audit `letter_cancelled` `{"reason","previous_status"}`
  tercipta.
- `TestCancelLetterRaceWithApproveFinal` diulang `-count=5` (15 race): tepat
  satu pemenang; cancel menang → `cancelled` tanpa `letter_number`; approve
  menang → cancel 409. Lulus stabil setelah perbaikan urutan lock
  letters → step (temuan High gelombang ini, lihat handoff Backend).
- Probe `qa_e03_probe_test.go`: delegate/approver bukan pembuat tidak bisa
  cancel (404, keberadaan surat tidak bocor); setelah batal, delegate aktif
  tidak bisa bertindak (404); id non-UUID tidak 2xx/panic (500 generik —
  temuan Low, pola existing).
- Regression gate tiga stack hijau (kecuali 11 kegagalan pre-existing
  company-scope, identik pada tree bersih).

Risiko tersisa: uji manual E2E dari UI web (tombol Batalkan → banner jejak)
belum dijalankan dengan backend hidup; disarankan saat smoke test rilis.
