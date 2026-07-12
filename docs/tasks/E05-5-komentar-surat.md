# E05-5 — Komentar internal per surat

Status: integrated (2026-07-13)

Tujuan:

Pengguna yang berhak melihat sebuah surat dapat berdiskusi lewat komentar
internal pada surat itu (ref backlog E05-5, PRD P0-5). Komentar bukan bagian
isi resmi surat: tidak masuk PDF final, tidak mengubah status, versi, atau
alur approval.

Perilaku/acceptance criteria:

- Pengguna yang lolos cek akses surat dapat menambah komentar dan membaca
  daftar komentar surat tersebut, terurut dari yang terlama (kronologis).
- Pengguna tanpa akses surat mendapat 404 (konsisten dengan `GetLetterDetail`
  yang menyembunyikan keberadaan surat), bukan 403.
- Komentar plain text, wajib non-kosong setelah trim, maksimal 2000 karakter.
  Tidak ada rich text; web wajib me-render sebagai teks (escape default React).
- Komentar tidak dapat diedit atau dihapus (append-only, menjaga jejak).
- Penambahan komentar tercatat di `audit_logs` mengikuti pola aksi surat yang
  sudah ada.
- Daftar komentar dipaginasi memakai konvensi `parsePagination` yang ada.

Scope owner:

- Backend: migrasi `0024_letter_comments` (pasangan up/down), handler baru
  `backend/internal/handler/letter_comment.go` + test, registrasi route di
  `internal/server/router.go` (grup `authed`, pola
  `/letters/view/:id/comments`).
- Web: method API komentar di `web/src/lib/api.ts` (task ini owner file hub
  itu untuk gelombang ini), section komentar di
  `web/src/app/(app)/letters/[id]/page.tsx` (daftar + form tambah), dengan
  state loading/empty/error/success dan disable tombol saat submit.
- Mobile: tidak ada.
- QA: verifikasi skenario di bawah + regression `go test ./...`, lint/build
  web.

Di luar scope:

- Edit/hapus komentar, mention, balasan berthread.
- Notifikasi komentar baru (menyusul via EPIC 08).
- Tampilan komentar di mobile.
- Komentar pada draft yang belum pernah diajukan (komentar mengikuti akses
  surat; kalau `userCanViewLetter` mengizinkan draft bagi pembuat, itu perilaku
  yang diterima).

Kontrak data/API:

Tabel `letter_comments`:

```sql
id         uuid primary key default gen_random_uuid()
letter_id  uuid not null references letters(id)
user_id    uuid not null references users(id)
body       text not null            -- divalidasi 1..2000 char di handler
created_at timestamptz not null default now()
-- index (letter_id, created_at)
```

Endpoint (grup `authed`):

> Catatan Backend 2026-07-13: envelope daftar mengikuti konvensi codebase
> (`{"data": [...], "meta": {...}}` seperti `ListLetterDispositions` dan
> `ListIncomingLetters`), bukan `{"comments": [...], "page", ...}` seperti
> draft awal. `meta` memuat `page`, `page_size`, `total`, `total_pages`.
> Respons create dibungkus `{"comment": {...}}` (mengikuti pola
> `{"letter": {...}}` pada detail surat).

- `GET /letters/view/:id/comments?page=&page_size=`
  - 200: `{ "data": [ { "id", "user_id", "user_name", "position_title",
    "body", "created_at" } ], "meta": { "page", "page_size", "total",
    "total_pages" } }` — terurut `created_at` menaik (terlama dulu);
    `position_title` adalah jabatan aktif komentator saat ini dan boleh null.
  - 400: pagination tidak valid (`{"error": "..."}`).
  - 404: surat tidak ada atau user tidak berhak.
- `POST /letters/view/:id/comments` body `{ "body": string }`
  - 201: `{ "comment": { ...shape sama dengan item daftar } }`.
  - 400: body kosong setelah trim, atau lebih dari 2000 karakter (rune).
  - 404: surat tidak ada atau user tidak berhak.

Authorization dan company scope:

- Kedua endpoint memakai `h.userCanViewLetter(ctx, userID, letterID)`;
  kegagalan cek = 404. Company scope tertutup oleh cek akses surat itu.
- Tidak ada requirement role khusus; hak komentar = hak melihat surat.

Dependency/order:

1. Backend (kontrak + migrasi + endpoint + test) — wave 1.
2. Web — wave 2, mulai setelah handoff Backend.
3. QA — wave 3, setelah handoff Web.

Verification:

- Backend: `go test ./...` dari `backend/`, termasuk test baru: happy path
  list+create, unauthenticated, user lintas company → 404, user tanpa relasi
  surat → 404, body kosong/2001 char → 400, pagination.
- Web: `npm run lint`, `npm run build`; uji manual alur tambah/lihat komentar
  dan laporkan langkahnya.
- QA: jalankan skenario `.agents/qa-security.md` yang relevan + migrasi
  `go run ./cmd/migrate up` lalu `down` lalu `up`.

Handoff yang diwajibkan:

Sesuai format root `AGENTS.md`, ditulis pada bagian di bawah ini.

## Handoff — Backend

Tanggal: 2026-07-13. Status: selesai, kontrak final (lihat catatan tanggal di
bagian "Kontrak data/API").

Ringkasan perilaku:

- Pengguna yang lolos `h.userCanViewLetter` (pembuat, approver, penerima,
  partisipan disposisi, audit assignment) dapat menambah dan membaca komentar
  surat. Gagal cek akses = 404 "surat tidak ditemukan" (konsisten
  `GetLetterDetail`, tidak membocorkan keberadaan surat).
- Komentar plain text, append-only (tidak ada endpoint edit/hapus), divalidasi
  di handler: trim dulu, wajib non-kosong, maksimal 2000 karakter (dihitung
  per rune via `utf8.RuneCountInString`, bukan byte).
- Daftar terurut kronologis (`created_at` menaik, tie-break `id`), dipaginasi
  `parsePagination` (default page=1, page_size=20, maks 100).
- `position_title` pada item = jabatan aktif komentator saat ini (lookup
  lateral seperti holder disposisi); null bila penugasan sudah berakhir.
- Setiap create tercatat di `audit_logs`: entity_type `letter_comment`,
  entity_id = id komentar, action `create`, detail `{"letter_id": ...}`,
  tanpa isi komentar (menghindari bocor konten surat rahasia ke log).

File yang diubah:

- `backend/migrations/0024_letter_comments.up.sql` + `.down.sql` (baru).
- `backend/internal/handler/letter_comment.go` (baru): `ListLetterComments`,
  `CreateLetterComment`.
- `backend/internal/handler/letter_comment_test.go` (baru): 6 test integrasi.
- `backend/internal/server/router.go`: registrasi GET+POST
  `/letters/view/:id/comments` di grup `authed` (tanpa role khusus).
- `backend/migrations/embed.go` tidak perlu diubah (glob `*.sql`).

Migrasi dan dampak rollback:

- Up: tabel `letter_comments` (id uuid PK, letter_id FK letters, user_id FK
  users, body text, created_at timestamptz) + index
  `idx_letter_comments_letter_created (letter_id, created_at)`.
- Down: `DROP TABLE IF EXISTS letter_comments` — seluruh komentar hilang saat
  rollback; tidak menyentuh tabel lain, tidak ada perubahan data existing.
- Diverifikasi `go run ./cmd/migrate up` → `down` → `up` sukses di DB dev.

Shape endpoint final: lihat bagian "Kontrak data/API" di atas (sudah
diperbarui 2026-07-13: list `{"data", "meta"}`, create `{"comment"}`).

Test yang dijalankan + hasil:

- `go test ./...` dari `backend/` (tanpa env integrasi): hijau.
- `go test ./internal/handler -run TestLetterComments` dengan
  `EOFFICE_INTEGRATION_DB_URL` (Postgres dev 5433): 6/6 PASS — happy path
  create+list (termasuk trim, urutan, audit log), unauthenticated 401 via
  `RequireAuth`, user lintas company 404, user tanpa relasi surat 404, body
  kosong/whitespace/2001-char 400 (2000-char tepat = 201), pagination
  (page 1/2, meta, page=0 dan page_size=abc → 400).
- Catatan: full suite dengan env integrasi menunjukkan kegagalan pre-existing
  di `user_position_test.go` (403 company-scope pada aksi admin) — sudah gagal
  tanpa perubahan task ini (diverifikasi via `git stash`), di luar scope E05-5.

Risiko/asumsi:

- Komentar mengikuti akses surat apa adanya: pembuat bisa berkomentar pada
  draft-nya sendiri (perilaku diterima sesuai kontrak).
- `position_title` dinamis (jabatan aktif saat dibaca, bukan snapshot saat
  komentar dibuat); bila komentator kehilangan jabatan, title menjadi null.
- `letter_id` path yang bukan UUID valid menghasilkan 500 (konsisten dengan
  `GetLetterDetail` existing); web selalu memakai id dari API sehingga aman.
- FK tanpa ON DELETE CASCADE (konsisten skema lain); surat tidak pernah
  dihapus fisik di aplikasi.

Yang harus dilakukan Web (wave 2):

- `web/src/lib/api.ts`: method GET/POST `/letters/view/:id/comments`;
  list dibaca dari `data` + `meta` (bukan `comments`), create dari `comment`.
- Render body sebagai plain text (escape default React), tampilkan
  `user_name`, `position_title` (boleh null → sembunyikan), `created_at`.
- Form komentar: disable tombol saat submit, validasi client 1..2000 char
  setelah trim (server tetap memvalidasi), tangani 400/404 dari server.
- Urutan sudah kronologis dari server; untuk "muat semua" gunakan pagination
  (`total_pages` di meta).

## Handoff — Web

Tanggal: 2026-07-13. Status: selesai, sesuai kontrak final Backend.

Ringkasan perilaku:

- Halaman detail surat menampilkan section "Komentar" (kolom samping, di
  bawah Timeline Approval) untuk semua surat yang bisa dilihat pengguna —
  daftar kronologis (nama, jabatan bila ada, waktu, isi plain text dengan
  `whitespace-pre-wrap`, escape default React) + form tambah komentar.
- Komentar dimuat setelah detail surat berhasil dimuat (akses komentar =
  akses surat, jadi tidak ada request komentar saat surat 404).
- Setelah kirim sukses: input dibersihkan dan daftar dimuat ulang pada
  halaman terakhir (dihitung dari `meta.total` + 1) sehingga komentar baru
  langsung tampil.

File yang diubah:

- `web/src/lib/api.ts`: type `LetterComment` (sumber type bersama), method
  `listLetterComments(letterID, opts)` (GET, baca `{data, meta}`) dan
  `createLetterComment(letterID, body)` (POST, baca `{comment}`), section
  "---- Komentar internal surat ----" setelah `getLetterDetail`.
- `web/src/app/(app)/letters/[id]/page.tsx`: section Komentar + form,
  handler `handleCreateComment`, callback `loadComments(page)`, reuse
  komponen `@/components/Pagination` untuk `total_pages > 1`.

State UI yang ditangani:

- Loading: "Memuat komentar...".
- Empty: "Belum ada komentar." (kartu dashed, pola sama dengan disposisi).
- Error load: pesan `role="alert"` + tombol "Coba lagi" (retry halaman yang
  sama).
- Success: daftar + badge total; pagination "Halaman X dari Y" (komponen
  Pagination existing, otomatis tersembunyi bila 1 halaman).
- Form: validasi client wajib non-kosong setelah trim dan maks 2000 karakter
  (dihitung per code point via `Array.from`, konsisten rune server;
  `maxLength=2000` + counter "n/2000"), tombol disable + label "Mengirim..."
  saat submit, error 400/404 server ditampilkan `role="alert"`.
- Aksesibilitas: textarea berlabel ("Tambah komentar"), focus ring mengikuti
  pola form halaman, error memakai `role="alert"`, pagination berupa button
  keyboard-accessible.

Verifikasi:

- `npm run lint` dari `web/`: hijau.
- `npm run build` dari `web/`: hijau (24/24 halaman).
- Uji manual (dev server Next + backend `go run ./cmd/api`, login
  `admin@ksk.local`, surat `f36f50f9-...` "Uji E2E Persetujuan Legal"):
  empty state; submit kosong → "Komentar wajib diisi."; kirim komentar
  (dengan spasi pinggir + multiline) → 201, tampil langsung dengan nama +
  jabatan + waktu, body ter-trim, input bersih, badge naik; seed 22+
  komentar → "Halaman 1 dari 2", tombol Sebelumnya/Berikutnya berfungsi,
  halaman 2 berisi sisa komentar; kirim komentar dari halaman 2 → daftar
  pindah/tetap di halaman terakhir dan komentar baru tampil; error load
  dirender dengan tombol "Coba lagi" (teramati saat backend lama tanpa route
  masih jalan → 404). Tidak ada error console.

Catatan lingkungan:

- Backend di port 8080 semula masih binary lama (api.exe yatim start
  2026-07-12) tanpa route komentar → 404 "page not found". Diperbaiki:
  `go run ./cmd/migrate up` (0024 terpasang), proses lama dihentikan, backend
  dijalankan ulang dari kode terkini. Tidak ada file backend yang diubah.
- Uji manual meninggalkan ±23 komentar seed pada surat uji di DB dev
  (append-only, tidak ada endpoint hapus).

Risiko/asumsi:

- Skenario 400 server (body >2000 rune) tidak dapat dipicu dari UI karena
  `maxLength` textarea sudah membatasi input; jalur render error server tetap
  teruji lewat kasus 404 route di atas.
- Daftar komentar tidak auto-refresh (tanpa polling); pengguna melihat
  komentar user lain setelah reload/navigasi — konsisten dengan section lain
  di halaman itu.

## Handoff — QA

Tanggal: 2026-07-13. Status: selesai — tidak ada defect terkonfirmasi;
2 risiko untuk keputusan Lead, 3 gap test.

Scope yang diverifikasi: migrasi `0024_letter_comments` (up→down→up),
handler `letter_comment.go` + route GET/POST `/letters/view/:id/comments`,
kontrak `web/src/lib/api.ts`, render section Komentar di
`web/src/app/(app)/letters/[id]/page.tsx`.

### (a) Defect terkonfirmasi

Tidak ada.

### (b) Risiko yang butuh keputusan Lead

1. **Tidak ada idempotency pada POST komentar — duplicate submit
   terkonfirmasi menghasilkan baris ganda.** Bukti: 2 request POST konkuren
   body identik (curl paralel, akun admin, surat `f36f50f9-...`) → keduanya
   201, 2 baris `letter_comments` tercipta (`created_at` selisih ~1,5 ms).
   Kontrak memang tidak mensyaratkan `client_action_id` (itu aturan aksi
   approval mobile), dan web men-disable tombol saat submit, tetapi retry
   jaringan/tab ganda tetap bisa menduplikasi. Keputusan: terima sebagai
   perilaku komentar (wajar untuk chat-like feature) atau jadwalkan
   idempotency key menyusul. Rekomendasi QA: terima, dokumentasikan.
2. **Tidak ada batas laju/kuota komentar.** Pengguna dengan akses surat
   dapat membanjiri komentar (2000 char per komentar, tanpa cap jumlah).
   Konsisten dengan endpoint lain (tidak ada rate limit per-endpoint di
   codebase), jadi bukan regresi task ini — hanya dicatat sebagai vektor
   pertumbuhan data/log bila diperlukan kelak.

Diketahui dan diterima (sudah didokumentasikan Backend, diverifikasi ulang
QA, bukan temuan baru): path id non-UUID → 500 (konsisten `GetLetterDetail`,
diuji GET+POST → 500); rollback `down` menghapus seluruh komentar;
`position_title` dinamis (bukan snapshot).

### (c) Gap test

1. Tidak ada test yang mendokumentasikan perilaku duplicate/concurrent
   submit (lihat risiko b.1) — kalau Lead memutuskan "terima", test yang
   mengunci perilaku itu tetap berguna.
2. Tidak ada test untuk path id non-UUID (saat ini 500); bila suatu saat
   dinormalisasi menjadi 404, tidak ada test yang menangkap perubahannya.
3. Alur web komentar hanya teruji manual (Playwright masih backlog E00-7).

### Checks yang dijalankan + hasil

Lingkungan: Docker compose aktif (Postgres 5433, Redis, MinIO, ClamAV),
backend dijalankan dari working tree (`go run ./cmd/api`, port 8080,
dimatikan lagi setelah pengujian), web dev server port 3000.

- Review kontrak: shape handler (`{"data", "meta"}` + `{"comment"}`, key
  `page/page_size/total/total_pages`) cocok dengan `Paginated<LetterComment>`
  dan `PageMeta` di `api.ts`, dan dengan bagian "Kontrak data/API" file ini.
- `go test ./...` (backend, tanpa env integrasi): hijau.
- `go test ./internal/handler -run TestLetterComments -count=1` dengan
  `EOFFICE_INTEGRATION_DB_URL` (Postgres dev 5433): 6/6 PASS.
- Migrasi di DB dev berisi data nyata (23 komentar sisa uji manual Web):
  `migrate down` → versi 23, tabel `letter_comments` hilang, tabel `letters`
  utuh (22 baris); `migrate up` → versi 24, tabel + index
  `idx_letter_comments_letter_created` kembali, dirty=false. Catatan: siklus
  ini menghapus 23 komentar seed milik uji Web (sesuai perilaku rollback).
- Uji API langsung (akun seed + user uji lintas company
  `qa.crossco@bbl.test` yang dibuat QA di company BBL, surat uji
  `f36f50f9-...` published/terbatas milik admin):
  - Creator: GET → 200; POST body berspasi pinggir → 201, body ter-trim.
  - Lintas company (BBL): GET → 404, POST → 404 `{"error":"surat tidak
    ditemukan"}`, tidak ada baris tercipta.
  - User se-company tanpa relasi surat (`division.creator`): GET → 404.
  - Tanpa token: GET/POST → 401, tidak ada baris tercipta.
  - UUID valid tapi tidak ada → 404 dengan body identik dengan kasus lintas
    company → tidak ada oracle enumerasi keberadaan surat.
  - Pagination invalid (`page=0`) oleh user tanpa akses: 400 identik untuk
    surat yang ada maupun tidak ada → urutan cek (400 sebelum 404) tidak
    membocorkan keberadaan surat.
  - Validasi: whitespace-only → 400; `{"body":123}` → 400; 2000 emoji
    (8000 byte, 2000 rune) → 201 dan 2001 emoji → 400 → penghitungan
    per-rune terbukti, bukan byte.
  - XSS: body `<script>alert(1)</script><img src=x onerror=alert(2)>` →
    201 (tersimpan apa adanya, benar untuk plain text).
- Audit log: setiap create tercatat `entity_type=letter_comment`,
  `entity_id=<id komentar>`, `action=create`, `detail={"letter_id":...}` —
  tanpa isi komentar (tidak bocor konten surat rahasia ke log). Kode baru
  hanya INSERT ke `audit_logs` (append-only terjaga).
- Render web nyata (Next dev + token admin, halaman detail surat uji):
  section Komentar tampil, payload XSS dirender sebagai teks inert —
  tidak ada elemen `<script>`/`<img>` tersuntik di DOM, tidak ada dialog
  alert, console bersih (0 error/warning).
- `npm run lint` dan `npm run build` dari `web/`: hijau (24/24 halaman).
- Pre-existing di luar scope (tidak dilaporkan ulang): kegagalan integrasi
  `user_position_test.go` (403 company scope).

Data uji yang tertinggal di DB dev (disengaja, dicatat): user
`qa.crossco@bbl.test` + posisi "QA BBL Head" di company BBL, 6 komentar QA
pada surat `f36f50f9-...` (termasuk 1 payload XSS inert dan 1 komentar 2000
emoji), serta entri audit_logs terkait. Komentar append-only sehingga tidak
dihapus lewat API; boleh dibersihkan via SQL bila mengganggu.

## Integrasi — Lead

Tanggal: 2026-07-13. Task ditutup sebagai **integrated**.

Keputusan atas risiko QA:

1. Duplicate submit POST komentar: **diterima** sebagai perilaku wajar fitur
   komentar (bukan aksi approval; append-only, tanpa efek samping status).
   Requirement `client_action_id` tetap khusus aksi approval mobile.
2. Rate limit komentar: **ditunda** — tercakup ticket backlog E10-3 (rate
   limiting menyeluruh), bukan pekerjaan task ini.

Gap test yang dicatat untuk backlog: test duplicate submit dan path id
non-UUID (pola 500 pre-existing) menyusul; alur web otomatis menunggu E00-7
(Playwright).

Pemeriksaan integrasi akhir: `go test ./...` backend hijau, test integrasi
komentar 6/6 PASS, `npm run lint` + `npm run build` web hijau, migrasi
up→down→up diverifikasi (laporan di Handoff — QA). Kontrak konsisten di
Backend dan Web; Mobile tidak terdampak.
