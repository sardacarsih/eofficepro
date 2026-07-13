# E03-5 — Delegasi wewenang berbatas waktu + pencatatan "a.n."

Status: contract-stable

Tujuan:

Pejabat approver yang cuti/dinas dapat mendelegasikan wewenang approval
posisinya kepada pengguna lain untuk rentang tanggal terbatas (ref backlog
E03-5, PRD P0-3). Selama delegasi aktif, surat yang menunggu di posisi
delegator dapat ditindaklanjuti oleh delegate, dan aksi tersebut tercatat
sebagai "a.n." (atas nama) posisi delegator. Delegasi aktif/berakhir otomatis
sesuai tanggal — tanpa job/cron.

Perilaku/acceptance criteria:

- Pemegang aktif sebuah posisi (atau admin company posisi itu) dapat membuat
  delegasi: posisi delegator, user delegate, alasan, `valid_from`, `valid_to`.
- Status delegasi diturunkan dari waktu query: `scheduled` (belum mulai),
  `active` (`now() >= valid_from AND now() < valid_to AND revoked_at IS NULL`),
  `expired` (lewat `valid_to`), `revoked` (dicabut manual).
- Selama aktif, delegate melihat step `waiting` posisi delegator di approval
  inbox (ditandai `is_delegated` + judul posisi asal), dapat membuka surat
  terkait, dan dapat approve/reject/request_revision. Aksi menyimpan
  `approval_actions.on_behalf_delegation_id`.
- Kapasitas langsung menang: bila delegate juga pemegang langsung posisi step,
  aksi tercatat sebagai diri sendiri (`on_behalf_delegation_id` NULL).
- Delegator (pemegang posisi) tetap dapat bertindak selama delegasi aktif;
  aksi pertama yang menang (perilaku step existing).
- Timeline surat dan PDF final menampilkan penanda "a.n." (nama delegate,
  jabatan delegator).
- Notifikasi approver menunggu (`notifyWaitingApprovers`) dan SLA
  reminder/eskalasi juga menjangkau delegate aktif; delegator tetap menerima.
- Tidak boleh ada dua delegasi non-revoked yang tumpang tindih rentang
  waktunya untuk posisi yang sama (dijaga constraint DB, uji konkuren → 409).
- Tidak ada self-delegation (delegate bukan pemegang aktif posisi delegator).
- Revoke berlaku seketika: akses inbox/aksi delegate hilang saat itu juga.
- Pembuatan dan pencabutan delegasi tercatat di `audit_logs` dan memicu
  notifikasi in-app + email ke delegate dan pemegang posisi delegator.

Scope owner:

- Backend: migrasi `0025` (bagian delegasi), handler baru
  `backend/internal/handler/delegation.go` + test, integrasi delegasi pada
  `approval_workflow.go` (`ActApprovalStep`, `ListApprovalInbox`),
  `letter_detail.go` (`userCanViewLetter`, `loadLetterApprovalActions`),
  `notification.go` (`notifyWaitingApprovers`), `sla_watcher.go`,
  `approval_signature.go` + `letter_e02.go` (blok ttd PDF), route di
  `internal/server/router.go`.
- Web: method API delegasi di `web/src/lib/api.ts` (owner file hub gelombang
  web), halaman baru `web/src/app/(app)/delegations/page.tsx`, item nav,
  badge "a.n." di `approvals/page.tsx` dan timeline `letters/[id]/page.tsx`.
- Mobile: parsing field baru inbox/detail di
  `mobile/lib/features/letters/data/letter_repository.dart` + models, chip
  "a.n." pada kartu approval dan timeline di `home_page.dart`. Manajemen
  delegasi TIDAK dibuat di mobile.
- QA: skenario batas waktu, revoke seketika, overlap konkuren, "a.n." di
  inbox/timeline/PDF, regression suite tiga stack.

Di luar scope:

- Delegasi berantai (delegate mendelegasikan lagi).
- Delegasi sebagian jenis surat / filter per letter type.
- Manajemen delegasi dari mobile.
- Mekanisme sekretaris `letters.on_behalf_of_position_id` (sisi pembuatan
  draft) — konsep terpisah, jangan disentuh.

Kontrak data/API:

Migrasi `0025` (bagian E03-5) — tabel `delegations` (migrasi 0001) di-ALTER,
jangan edit 0001:

```sql
CREATE EXTENSION IF NOT EXISTS btree_gist;
ALTER TABLE delegations
    ADD COLUMN revoked_at timestamptz,
    ADD COLUMN revoked_by uuid REFERENCES users(id);
ALTER TABLE delegations
    ADD CONSTRAINT delegations_no_overlap
    EXCLUDE USING gist (delegator_position_id WITH =,
                        tstzrange(valid_from, valid_to) WITH &&)
    WHERE (revoked_at IS NULL);
CREATE INDEX idx_delegations_delegate_active
    ON delegations(delegate_user_id, valid_from, valid_to)
    WHERE revoked_at IS NULL;
```

Down: drop constraint, index, dan kedua kolom; extension dibiarkan.

Fragmen otorisasi delegasi aktif (const Go tunggal, dipakai inbox/act/view/
notify/SLA):

```sql
EXISTS (SELECT 1 FROM delegations d
        WHERE d.delegator_position_id = <step.approver_position_id>
          AND d.delegate_user_id = <user_id>
          AND now() >= d.valid_from AND now() < d.valid_to
          AND d.revoked_at IS NULL)
```

Endpoint baru (grup `authed`, otorisasi in-handler):

- `POST /delegations`
  body `{ "delegator_position_id", "delegate_user_id", "reason",
  "valid_from", "valid_to" }` (RFC3339).
  - 201: `{ "delegation": { "id", "delegator_position_id",
    "delegator_position_title", "delegate_user_id", "delegate_name",
    "reason", "valid_from", "valid_to", "revoked_at": null,
    "status": "scheduled"|"active", "created_by_name", "created_at" } }`
  - 400: field kosong/format salah, `valid_from >= valid_to`,
    `valid_to <= now()`, self-delegation, delegate tidak aktif/beda company.
  - 403: caller bukan pemegang aktif posisi delegator dan bukan admin
    company posisi tersebut.
  - 409: rentang tumpang tindih dengan delegasi non-revoked lain
    (tangkap SQLSTATE 23P01).
- `GET /delegations?scope=delegator|delegate&include_past=true|false`
  - 200: `{ "data": [ ...shape sama dengan objek create... ] }`, terurut
    `valid_from` menurun. Tanpa `scope`: gabungan keduanya (dan, untuk admin,
    seluruh delegasi company-nya). `include_past=false` (default)
    menyembunyikan `expired`/`revoked`.
- `DELETE /delegations/:id`
  - 200: `{ "delegation": { ..., "status": "revoked" } }`
  - 403: bukan pembuat, bukan pemegang aktif posisi delegator, bukan admin
    company.
  - 404: id tidak ada / beda company.
  - 409: sudah `expired` atau `revoked`.
- `GET /delegations/delegate-options?position_id=`
  - 200: `{ "data": [ { "user_id", "full_name", "position_titles": [] } ] }`
    — user aktif pemegang posisi aktif se-company dengan `position_id`,
    tanpa pemegang posisi delegator itu sendiri.
  - 403/404 seperti POST.

Perubahan respons endpoint existing:

- `GET /approvals/inbox` item: tambah `"is_delegated": bool` dan
  `"delegated_from_title": string|null` (judul posisi delegator bila item
  tampil karena delegasi).
- `GET /letters/view/:id` pada `approval_actions[]`: tambah
  `"on_behalf_of": bool` dan `"on_behalf_of_position_title": string|null`.
- `POST /approvals/steps/:id/action` (shape existing): tidak berubah bagi
  klien; server yang meresolusi delegasi dan mengisi
  `on_behalf_delegation_id`. Idempotency `client_action_id` tidak berubah.

Authorization dan company scope:

- Semua validasi server-side. Company scope: posisi delegator menentukan
  company; delegate wajib pemegang posisi aktif pada company yang sama;
  admin hanya dapat mengelola delegasi company-nya (pola
  `RequireCompanyAdministrator` existing, diterapkan in-handler).
- Aksi step oleh delegate memakai fragmen delegasi aktif di atas — bukan
  kepercayaan pada payload klien.

Dependency/order:

1. Backend wave 1 (bersama E03-7 — berbagi migrasi 0025 dan perubahan urutan
   lock `ActApprovalStep` (letters dulu, baru step); kerjakan migrasi + lock dulu).
2. Web dan Mobile wave 2 setelah handoff Backend (boleh paralel; `api.ts`
   satu owner).
3. QA wave 3.

Verification:

- Backend: `go test ./...`; test integrasi baru `delegation_test.go` minimal:
  CRUD + otorisasi (403 non-pemegang, 404 lintas company), boundary waktu
  (sebelum `valid_from`, aktif, setelah `valid_to`), overlap 409 (termasuk
  konkuren), revoke seketika, act "a.n." mengisi `on_behalf_delegation_id`,
  delegate=pemegang langsung → NULL, inbox delegate, `userCanViewLetter`
  delegate, notifikasi + SLA menjangkau delegate. Migrasi up→down→up.
- Web: `npm run lint`, `npm run build`, uji manual buat/revoke + badge a.n.
- Mobile: `flutter analyze`, `flutter test`.
- E2E: delegasi aktif → delegate approve dari mobile → timeline web & PDF
  final menampilkan "a.n. <jabatan delegator>".

Catatan kontrak (Backend, 2026-07-13):

- Route action existing adalah `POST /approvals/steps/:id/actions` (plural,
  sesuai router existing) — bukan `/action`; shape request/response tidak
  berubah bagi klien.
- Perubahan lock `ActApprovalStep`: dua statement berurutan — lock baris
  `letters` dulu (`FOR UPDATE`, cek `status='in_approval'`), baru lock step
  (`FOR UPDATE OF s` + authz/delegasi). `FOR UPDATE OF s, l` satu statement
  terbukti deadlock (40P01) pada race test karena planner mengunci `s` lebih
  dulu; urutan letters → step konsisten dengan `CancelLetter` (E03-7).
- Notifikasi delegasi memakai `event_type` `delegation_created` dan
  `delegation_revoked` (`letter_id` NULL); deep link email mengarah ke
  `/delegations`.
- `GET /delegations` tanpa `scope` menggabungkan delegator + delegate + (untuk
  admin/super admin) seluruh delegasi company yang dia kelola. Respons list
  tidak dipaginasi (`{"data": [...]}` saja, tanpa `meta`).
- `DELETE /delegations/:id`: caller tanpa relasi apa pun ke company delegasi
  mendapat 404 (menyembunyikan keberadaan); caller se-company tanpa hak
  mendapat 403 — sesuai semantik kontrak "404 beda company".
- Validasi tambahan: `reason` maksimal 255 karakter (batas kolom DB).

Handoff yang diwajibkan: sesuai format root `AGENTS.md`, pada bagian di bawah.

## Handoff — Backend

Status: selesai (wave 1), bersama E03-7.

Ringkasan perilaku:

- Migrasi `0025` menambah `delegations.revoked_at/revoked_by`, constraint
  anti-overlap `delegations_no_overlap` (EXCLUDE gist + `btree_gist`), dan
  index partial `idx_delegations_delegate_active`.
- Endpoint baru: `POST/GET /delegations`, `DELETE /delegations/:id`,
  `GET /delegations/delegate-options?position_id=` (grup `authed`, otorisasi
  in-handler: pemegang aktif posisi delegator atau admin company posisi itu).
- Status delegasi diturunkan dari waktu query (`scheduled/active/expired/
  revoked`) — tanpa cron; revoke berlaku seketika.
- Fragmen SQL delegasi aktif didefinisikan satu kali
  (`activeDelegationExistsSQLTemplate` di `delegation.go`) dan dipakai oleh
  inbox, act, view, notifikasi, dan SLA.
- `ActApprovalStep`: delegate aktif boleh approve/reject/request_revision;
  `approval_actions.on_behalf_delegation_id` terisi (NULL bila user juga
  pemegang langsung — kapasitas langsung menang); detail audit menambah
  `on_behalf_delegation_id`.
- `GET /approvals/inbox`: item menambah `is_delegated` dan
  `delegated_from_title` (judul posisi delegator; NULL bila akses langsung).
- `GET /letters/view/:id`: `userCanViewLetter` mendapat cabang delegasi aktif;
  `approval_actions[]` menambah `on_behalf_of` (bool) dan
  `on_behalf_of_position_title` (string|null).
- `notifyWaitingApprovers`: UNION delegate aktif, dedup per user.
- SLA watcher: reminder dan eskalasi menjangkau delegate aktif.
- PDF final: blok tanda tangan memberi prefix `a.n. ` pada nama approver bila
  aksinya on-behalf (jabatan yang tampil tetap jabatan delegator).
- Notifikasi in-app + outbox email untuk create/revoke delegasi ke delegate
  dan seluruh pemegang aktif posisi delegator; audit `delegation/create` dan
  `delegation/revoke`.

File diubah/baru:

- Baru: `backend/migrations/0025_delegation_lifecycle_and_cancel_trace.up.sql`
  + `.down.sql` (bersama E03-7), `backend/internal/handler/delegation.go`,
  `backend/internal/handler/delegation_test.go`.
- Diubah: `approval_workflow.go` (urutan lock letters → step + delegasi pada
  act/inbox), `letter_detail.go`, `notification.go` (union delegate + helper
  `notificationLink`), `notification_outbox.go` (pakai `notificationLink`),
  `sla_watcher.go`, `approval_signature.go` (flag `OnBehalf`),
  `letter_e02.go` (prefix `a.n. `), `internal/server/router.go` (route),
  `docs/DATABASE-SCHEMA.md` (kolom & constraint delegasi).

Migrasi + rollback:

- `0025` up menambah kolom/constraint/index; down menghapusnya (extension
  `btree_gist` dibiarkan). Rollback aman: kolom baru nullable, tidak ada
  backfill. Bila sudah ada delegasi ter-revoke lalu down dijalankan, informasi
  revoke hilang (dapat diterima — rollback skema).
- Terverifikasi up -> down -> up di DB dev.

Shape endpoint final: sesuai bagian "Kontrak data/API" di atas + "Catatan
kontrak (Backend, 2026-07-13)".

Test yang dijalankan (verifikasi ulang oleh Lead, 2026-07-13):

- `go build ./...` dan `go test ./...` dari `backend/` tanpa DB — hijau.
- Migrasi `up → down → up` di Postgres dev 5433 — versi 24 → 25 → 24 → 25;
  constraint `delegations_no_overlap` dan kolom cancel trace terpasang.
- Suite integrasi (`EOFFICE_INTEGRATION_DB_URL`, Postgres dev 5433), semua
  PASS: `TestDelegationCreateListRevoke`, `...CreateValidationAndAuthorization`
  (empty_reason/bad_valid_from/from_after_to/to_in_past/self_delegation),
  `...OverlapConcurrent` (tepat satu 201 + satu 409),
  `...ActOnBehalfAndBoundaries` (scheduled/expired tak bisa bertindak; aktif
  mengisi `on_behalf_delegation_id`), `...DirectCapacityWinsAndRevokeImmediate`,
  `...NotificationsAndSLAReachDelegate`, `...DelegateOptions`.
- Kegagalan tersisa di suite handler hanyalah 11 test pre-existing keluarga
  company-scope (`user_position_test.go`, `secretary_flow_test.go` —
  403/"tidak punya akses ke perusahaan ini"), terkonfirmasi gagal identik pada
  tree bersih tanpa perubahan wave ini; di luar scope task.

Risiko/asumsi:

- Delegasi berantai tidak dicegah level DB, tetapi tak mungkin terjadi karena
  hanya pemegang langsung/admin yang bisa membuat delegasi dan delegate tidak
  mewarisi hak itu.
- Test SLA memanggil `insertSLAReminders/Escalations` yang memproses seluruh
  step due di DB dev (dedup mencegah duplikat; hanya notifikasi in-app,
  outbox tidak di-enqueue oleh fungsi tsb).
- `GET /delegations` tanpa paginasi; volume delegasi diasumsikan kecil.

Instruksi Web/Mobile (wave 2):

- Web: pakai `POST/GET /delegations`, `DELETE /delegations/:id`,
  `GET /delegations/delegate-options?position_id=`; field list sesuai shape
  create (status server-side, jangan hitung ulang di klien; `include_past`
  default false). Badge "a.n." dari `is_delegated`/`delegated_from_title`
  (inbox) dan `on_behalf_of`/`on_behalf_of_position_title` (timeline detail).
  Notifikasi `delegation_created`/`delegation_revoked` tautkan ke
  `/delegations`.
- Mobile: cukup parsing `is_delegated`, `delegated_from_title` pada item
  inbox dan `on_behalf_of`, `on_behalf_of_position_title` pada
  `approval_actions[]`; tidak ada perubahan payload aksi approval
  (`client_action_id` tetap).

## Handoff — Web

Status: selesai (wave 2). Tanggal: 2026-07-13.

Ringkasan perilaku:

- Halaman baru `/delegations`: daftar delegasi (gabungan delegator/delegate/
  admin sesuai server, tanpa `scope`) dengan chip status `Terjadwal/Aktif/
  Kedaluwarsa/Dicabut` — status dipakai apa adanya dari server, tidak dihitung
  ulang di klien. Toggle "Tampilkan riwayat" memetakan `include_past=true`.
- Dialog buat delegasi: posisi delegator dipilih dari `me.positions`, kandidat
  delegate dimuat dari `GET /delegations/delegate-options?position_id=`
  (loading/empty/error state), rentang `datetime-local` dikonversi ke RFC3339
  (`Date#toISOString`), alasan wajib maks 255 (hitung code point). Error 409
  overlap (dan 400/403 lain) menampilkan pesan server apa adanya.
- Aksi "Cabut" hanya tampil untuk status `scheduled|active`, dengan dialog
  konfirmasi; error server (mis. 409 sudah berakhir) ditampilkan di dialog.
- Item navigasi sidebar "Delegasi" (semua pengguna login — delegate belum
  tentu approver) + judul halaman di layout app.
- Inbox approval (`/approvals`): badge "a.n. <delegated_from_title>" pada item
  `is_delegated`. Perilaku aksi tidak diubah (approve tetap Android-only;
  reject/revisi web tetap seperti sebelumnya).
- Timeline detail surat (`/letters/[id]`): baris "a.n.
  <on_behalf_of_position_title>" pada aksi dengan `on_behalf_of`.
- NotificationBell: event `delegation_created`/`delegation_revoked`
  (letter_id NULL) menaut ke `/delegations` sesuai instruksi backend.

File diubah/baru:

- Baru: `web/src/app/(app)/delegations/page.tsx`.
- Diubah: `web/src/lib/api.ts` (type `Delegation`, `DelegationStatus`,
  `DelegateOption`, `CreateDelegationPayload`; method `listDelegations`,
  `createDelegation`, `revokeDelegation`, `listDelegateOptions`; extend
  `ApprovalInboxItem` + `is_delegated`/`delegated_from_title`,
  `LetterApprovalAction` + `on_behalf_of`/`on_behalf_of_position_title`),
  `web/src/app/(app)/approvals/page.tsx` (badge a.n.),
  `web/src/app/(app)/letters/[id]/page.tsx` (a.n. di timeline),
  `web/src/components/layout/Sidebar.tsx` (nav item),
  `web/src/app/(app)/layout.tsx` (judul halaman),
  `web/src/components/layout/NotificationBell.tsx` (deep link delegasi).

Migrasi/perubahan kontrak: tidak ada; mengikuti shape kontrak + catatan
Backend 2026-07-13 apa adanya (dicek silang terhadap JSON tag handler).

Check yang dijalankan:

- `npm run lint` dari `web/` — hijau (setelah menyesuaikan pola
  `queueMicrotask` untuk setState dalam effect, konsisten halaman existing).
- `npm run build` — hijau; route `/delegations` ikut ter-generate.
- Uji manual browser belum dilakukan pada sesi ini (backend perlu jalan);
  langkah uji disarankan: buat delegasi dari pemegang posisi → cek chip
  status, coba rentang tumpang tindih → pesan 409 server tampil, revoke →
  hilang dari daftar default dan muncul di riwayat, login sebagai delegate →
  badge "a.n." di inbox approval, dan timeline surat setelah delegate
  bertindak dari mobile.

Risiko/asumsi:

- Halaman delegasi hanya menawarkan posisi milik user sebagai delegator;
  admin company yang ingin membuat delegasi untuk posisi orang lain belum
  difasilitasi UI (server sudah mendukung) — pekerjaan lanjutan bila
  dibutuhkan.
- `delegated_from_title`/`on_behalf_of_position_title` NULL di-fallback ke
  judul posisi step; secara kontrak field terisi saat flag true.
- Perluasan scope kecil yang disengaja: `NotificationBell.tsx` disentuh untuk
  deep link `/delegations` sesuai instruksi handoff Backend.

## Handoff — Mobile

Status: selesai (wave 2, scope minimal). Tanggal: 2026-07-13.

Ringkasan perilaku:

- `ApprovalInboxItem` mem-parse `is_delegated` (default false) dan
  `delegated_from_title` (nullable) — backward-compatible terhadap respons
  lama tanpa field tersebut.
- `LetterApprovalAction` mem-parse `on_behalf_of` (default false) dan
  `on_behalf_of_position_title` (nullable).
- Kartu approval inbox menampilkan badge "a.n. <judul posisi delegator>"
  bila `is_delegated` (fallback ke `position_title` step bila
  `delegated_from_title` null).
- Timeline approval pada detail surat menampilkan sufiks
  "(a.n. <on_behalf_of_position_title>)" setelah nama aktor untuk aksi
  on-behalf; tanpa judul, tampil "(a.n.)" saja.
- Payload aksi approval TIDAK berubah: `client_action_id` tetap dikirim
  seperti sebelumnya; resolusi delegasi sepenuhnya di server. Tidak ada
  manajemen delegasi di mobile (web-only sesuai kontrak).

File diubah:

- `mobile/lib/features/letters/domain/letter_models.dart` — field baru pada
  `ApprovalInboxItem` dan `LetterApprovalAction`.
- `mobile/lib/features/home/presentation/home_page.dart` — badge delegasi
  pada `_LetterListCard` (param opsional `badge`, hanya diisi oleh
  `ApprovalListPane`) dan helper `_onBehalfSuffix` untuk timeline.
- `mobile/test/model_parsing_test.dart` — test parsing field delegasi
  (nilai terisi + default saat field absen).

Migrasi/kontrak: tidak ada perubahan kontrak dari sisi mobile; konsumsi
sesuai "Kontrak data/API" + catatan Backend (route `POST
/approvals/steps/:id/actions` sudah dipakai mobile sejak sebelumnya).

Check yang dijalankan (dari `mobile/`):

- `flutter analyze` — No issues found.
- `flutter test` — All tests passed (112 test, termasuk test baru).

Risiko/asumsi:

- Badge memakai `Wrap`-friendly container satu baris dengan ellipsis; layout
  kartu list sama untuk two-pane tablet landscape dan compact portrait
  (diverifikasi lewat test layout existing, bukan perangkat fisik).
- Bila `is_delegated` true tetapi `delegated_from_title` null (tidak
  diharapkan dari backend), badge memakai `position_title` step sebagai
  fallback agar tidak menampilkan "a.n. " kosong.
- Verifikasi E2E delegate approve dari mobile menjadi bagian wave QA.

## Handoff — QA

Status: selesai (wave 3), bersama E03-7. Tanggal: 2026-07-14.
Detail lengkap: `docs/LAPORAN-UJI-E03-5-E03-7.md`.

Verifikasi yang dijalankan (Postgres dev 5433, schema v25):

- Seluruh suite `delegation_test.go` PASS; `TestDelegationOverlapConcurrent`
  diulang `-count=5` — tepat satu 201 + satu 409 di tiap iterasi.
- Probe adversarial baru `qa_e03_probe_test.go` (dipertahankan sebagai
  regresi), semua PASS: isolasi admin lintas company (list tidak bocor,
  revoke 404, create 403, delegate-options 403); delegasi expired → act 404 +
  inbox kosong; idempotency `client_action_id` konkuren via delegate → tepat
  satu action; delegasi revoked tidak memblokir rentang baru; id non-UUID
  tidak 2xx/panic.
- Review defensif: semua query parameterized; fragmen delegasi aktif
  konsisten di inbox/act/view/notify/SLA; tidak ada UPDATE/DELETE
  `audit_logs` di kode aplikasi; kontrak JSON backend = web `api.ts` =
  mobile `letter_models.dart` tanpa mismatch; "a.n." PDF diverifikasi via
  kode (`OnBehalf` → prefix di `writeApprovalSignaturesBlock`).
- Regression gate: web `lint`+`build` hijau; mobile `analyze`+`test` (112)
  hijau; backend full suite hanya menyisakan 11 kegagalan pre-existing
  company-scope (identik pada tree bersih — known issue, ticket terpisah).

Temuan: (1) High — deadlock 40P01 race approve-vs-cancel pada varian lock
awal, sudah diperbaiki dan diverifikasi race x5 (lihat handoff Backend
E03-7); (2) Low — id path non-UUID → 500 generik alih-alih 404, konsisten
pola endpoint existing, kandidat perbaikan lintas endpoint terpisah.

Risiko tersisa: uji manual E2E UI web + Android + render PDF fisik belum
dijalankan (butuh backend hidup + APK); direkomendasikan sebelum rilis.
