# E10-1 — Enforcement klasifikasi Biasa/Terbatas/Rahasia + pencatatan akses ditolak

Status: verified

Tujuan:

AC backlog: "Uji akses URL langsung ditolak & tercatat". Audit kode 14 Jul 2026
menunjukkan enforcement klasifikasi inti SUDAH terpasang server-side:

- `publishedRecipientAccessSQL` (letter_inbox.go): surat `rahasia` hanya untuk
  penerima `to` (CC tidak); dipakai konsisten di detail, inbox, search,
  dashboard, download.
- `userCanDownloadLetter` (letter_download.go): unduhan `rahasia` hanya
  creator/approver/penerima `to`.
- Disposisi tidak memberi akses view untuk `rahasia` (letter_detail.go).
- Scope auditor dibatasi `max_classification` (authorization.go).

Gap yang tersisa (scope task ini):

1. **Penolakan tidak tercatat.** Akses URL langsung yang ditolak (detail,
   unduh PDF final, unduh lampiran) mengembalikan 404 tanpa jejak audit.
   AC menuntut "ditolak & tercatat".
2. **Belum ada test matriks klasifikasi.** Tidak ada test integrasi adversarial
   yang mengunci perilaku klasifikasi (mis. CC pada rahasia = 404; disposisi
   pada rahasia = 404; lintas company = 404).

Perilaku/acceptance criteria:

- Ketika user terautentikasi mencoba membuka detail surat / unduh PDF final /
  unduh lampiran dan otorisasinya ditolak (bukan karena surat tidak ada),
  respons tetap 404 (tidak membocorkan keberadaan surat) DAN tercatat baris
  `audit_logs`: entity `letter`, entity_id = letter id, action
  `access_denied`, detail JSON `{"via": "detail"|"download_pdf"|"download_attachment"}`,
  actor = user penyebab, IP tercatat.
- Penolakan HANYA dicatat bila surat memang ada (hindari spam audit untuk id
  acak yang tidak ada; cek existence setelah cek akses gagal, dalam satu query
  bila mungkin).
- Test integrasi matriks klasifikasi (file baru
  `backend/internal/handler/classification_access_test.go`):
  - `rahasia`: penerima CC → detail 404 + tercatat; unduh 404; penerima To →
    200; approver → 200; disposisi TIDAK memberi akses.
  - `terbatas`/`biasa`: penerima To/CC → 200; user lain se-company tanpa
    hubungan → 404; user company lain → 404.
  - `access_denied` muncul di audit_logs untuk kasus penolakan di atas.
- Perilaku akses yang sudah benar TIDAK berubah (regresi suite existing hijau).

Scope owner:
- Backend: `backend/internal/handler/letter_detail.go`,
  `letter_download.go`, test baru. TIDAK menyentuh `router.go` (dipakai E10-3).
- Web: tidak ada.
- Mobile: tidak ada.
- QA: verifikasi adversarial + laporan.

Di luar scope:

- E10-2 watermark PDF Rahasia (ticket terpisah).
- Perbedaan visibilitas `biasa` vs `terbatas`: PRD §7.5 menyebut Biasa "dapat
  dilihat unit terkait" vs Terbatas "hanya To/CC dan rantai approval".
  Implementasi saat ini memperlakukan keduanya identik (akses = To/CC + rantai
  approval + disposisi; keduanya TIDAK terlihat unit lain non-penerima).
  Ini lebih ketat dari PRD untuk `biasa` dan sudah memenuhi PRD untuk
  `terbatas`. JANGAN mengubah aturan bisnis ini tanpa keputusan produk —
  didokumentasikan sebagai catatan, bukan bug.

Kontrak data/API:

Tidak ada perubahan bentuk request/response. Hanya efek samping INSERT
`audit_logs` (append-only; ingat aturan non-negotiable: tanpa UPDATE/DELETE).

Authorization dan company scope:

Tidak ada jalur akses baru. Pencatatan memakai `h.audit` yang sudah ada.

Dependency/order:

Independen dari E10-3, tapi hindari menyentuh `router.go` (owner E10-3 di
gelombang yang sama).

Verification:

- `go test ./internal/handler/ -run TestClassification -count=1` (DB integrasi)
- `go test ./...` penuh hijau
- `go vet ./...`

Handoff yang diwajibkan: ringkasan perilaku, file diubah, hasil test, risiko.

## Handoff — Backend

Ringkasan perilaku (selesai, 14 Jul 2026):

- Penolakan otorisasi pada akses URL langsung kini tercatat: `GET
  /letters/view/:id` (via `detail`), `GET /letters/view/:id/final-pdf` (via
  `download_pdf`), dan `GET /letters/view/:id/attachments/:attachment_id/download`
  (via `download_attachment`) menulis baris `audit_logs` dengan entity
  `letter`, entity_id = id surat, action `access_denied`, actor = user
  penyebab, detail `{"via": "..."}`, dan IP klien. Respons ke klien tetap 404
  tanpa membocorkan keberadaan surat.
- Pencatatan memakai satu query `INSERT .. SELECT FROM letters WHERE id = $1`
  (helper baru `auditLetterAccessDenied` di `letter_detail.go`): bila surat
  tidak ada, tidak ada baris yang ditulis — id acak tidak menghasilkan spam
  audit. `audit_logs` tetap append-only (hanya INSERT).
- Kegagalan menulis audit hanya dicatat ke log aplikasi (tidak menggagalkan
  respons), konsisten dengan helper `h.audit` yang ada.
- Perubahan urutan kecil di `letter_download.go`: cek akses unduh kini
  dilakukan SEBELUM cek ketersediaan MinIO. Dampak: user yang ditolak
  otorisasinya menerima 404 + tercatat meskipun object storage sedang mati
  (sebelumnya 503, membocorkan keadaan storage). Jalur user yang berhak tidak
  berubah.

File diubah:

- `backend/internal/handler/letter_detail.go` — audit denial di
  `GetLetterDetail` + helper `auditLetterAccessDenied`.
- `backend/internal/handler/letter_download.go` — audit denial di
  `DownloadFinalLetterPDF` dan `DownloadLetterAttachment`; cek akses
  dipindah sebelum cek MinIO.
- `backend/internal/handler/classification_access_test.go` (baru) — matriks
  klasifikasi integrasi.

Kontrak/migrasi: tidak ada perubahan bentuk request/response, tidak ada
migrasi. `router.go` TIDAK disentuh (sesuai kontrak, dipakai E10-3).

Test yang dijalankan (14 Jul 2026, DB integrasi Postgres 5433 aktif, TANPA
skip):

- `go test ./internal/handler/ -run TestClassification -count=1 -v` — PASS:
  `TestClassificationRahasiaMatrix_Integration` (To/approver 200; CC detail
  404 + audit; CC unduh PDF/lampiran 404 + audit; penerima disposisi 404 +
  audit; `userCanDownloadLetter` To/approver true, CC/disposisi false),
  `TestClassificationTerbatasDanBiasaMatrix_Integration` (To/CC 200; unrelated
  se-company 404 + audit detail & download_pdf; company lain 404 + audit),
  `TestClassificationMissingLetterNotAudited_Integration` (id tidak ada → 404
  tanpa baris audit).
- `go vet ./...` — bersih.
- `go test ./... -count=1` penuh dengan `EOFFICE_INTEGRATION_DB_URL` — 92
  top-level test PASS, 0 FAIL, 0 SKIP.

Risiko/asumsi:

- Bila `letterID` pada path bukan UUID, query audit gagal cast dan hanya
  tercatat di log aplikasi (bukan `audit_logs`); setelah E10-3, path id
  non-UUID sudah dipotong 404 oleh middleware sebelum mencapai handler.
- Penolakan pada endpoint lain yang memakai `userCanViewLetter` (komentar,
  disposisi) belum diaudit — di luar scope task ini (AC hanya detail + unduh).
- Catatan produk `biasa` vs `terbatas` (identik, lebih ketat dari PRD untuk
  `biasa`) tidak diubah, sesuai bagian "Di luar scope".

## Handoff — QA

Verifikasi adversarial 14 Jul 2026 (laporan lengkap:
`docs/LAPORAN-UJI-E10-1-E10-3.md`). Semua AC terverifikasi independen; enforcement
E10-1 diterima.

Defect terkonfirmasi: tidak ada pada scope E10-1.

Verifikasi yang lolos:
- Matriks klasifikasi (DB nyata, tidak skip): CC-rahasia detail/unduh-PDF/
  unduh-lampiran → 404 + `access_denied` (`via` sesuai); disposisi-rahasia →
  404 + audit; lintas company → 404 + audit; To/approver → 200; surat tidak ada
  → 404 tanpa baris audit.
- Konsistensi `publishedRecipientAccessSQL`/`userCanViewLetter` dicek lintas
  detail, inbox, search, dashboard, download, dan **komentar surat**
  (`letter_comment.go` memakai `userCanViewLetter`) — tidak ada jalur baca yang
  lupa di-enforce. Disposisi & auditor dikecualikan/ dibatasi untuk rahasia.
- Audit append-only (INSERT..SELECT, kolom cocok skema); tidak ada
  UPDATE/DELETE `audit_logs` di kode produk.
- Probe HTTP end-to-end: admin tanpa hubungan → 404 pada detail & final-pdf
  surat nyata + tepat 2 baris `access_denied` dengan IP tercatat.

Gap test / risiko (bukan blocker):
- Denial pada komentar/disposisi belum diaudit — sesuai batas AC (hanya
  detail+unduh); enforcement 404-nya benar. Kandidat perluasan.
- Probe CC-rahasia via token penerima CC nyata (HTTP murni) tidak diulang;
  terkunci oleh test integrasi handler + probe admin-denied end-to-end.

Regression gate: `go vet ./...` bersih; `go test ./... -count=1` (DB+Redis) —
paket `handler` & `middleware` `ok`, 0 FAIL, 0 SKIP.
