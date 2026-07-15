# eOffice Pro — Backlog Epic & Ticket
**Turunan dari:** [PRD-eOfficePro.md](../PRD-eOfficePro.md) · Estimasi memakai story point (SP): 1 = ±setengah hari, 13 = ±2 minggu.
**Asumsi tim:** 1 tech lead, 2 backend, 2 frontend web, 1–2 Android, 1 QA, 1 UI/UX (sebagian paruh waktu).

---

## Ringkasan Fase

| Fase | Epic | Total SP (indikatif) |
|------|------|----------------------|
| Fase 1 — MVP Web | E00–E08, E10 | ± 280 SP |
| Fase 2 — Android | E09 | ± 90 SP |
| Fase 3 — Rollout | E11 | ± 40 SP |
| Fase 4 — P1 | E12+ | menyusul |

---

## EPIC 00 — Fondasi Proyek & Infrastruktur
> Prasyarat semua epic lain. Ref: NFR PRD §9, Open Question Q3 (on-prem vs cloud).

| ID | Ticket | AC singkat | SP |
|----|--------|-----------|----|
| E00-1 | Setup repo, CI/CD, environment dev/staging/prod | Pipeline build+test otomatis; deploy staging 1 klik | 5 |
|  | ↳ Progres 14 Jul 2026: CI GitHub Actions aktif (`.github/workflows/ci.yml`) — backend go vet+test integrasi (Postgres 16 service + migrasi), web lint+build, mobile analyze+test, pada push `main` dan semua PR. Deploy staging/prod belum. | | |
| E00-2 | Arsitektur dasar API (REST, versioning, error format standar) | ADR tertulis; skeleton API jalan | 5 |
| E00-3 | Setup database + migration tooling | Migration up/down teruji | 3 |
| E00-4 | Object storage lampiran (S3-compatible/MinIO) + enkripsi at-rest | Upload/download via pre-signed URL | 5 |
| E00-5 | Logging terstruktur, monitoring, health check | Dashboard uptime & error rate | 3 |
| E00-6 | Backup otomatis harian + prosedur restore teruji | Restore drill sukses di staging | 3 |
| E00-7 | Setup Playwright smoke test alur kritis web (login, tulis surat, approval, disposisi) | Berjalan lokal & di CI; kegagalan memblokir merge | 3 |

## EPIC 01 — Autentikasi & Manajemen Pengguna/Organisasi (P0-1, P0-10) ✅ SELESAI (7 Jul 2026)
> Catatan penyelesaian: semua ticket terimplementasi & teruji end-to-end.
> E01-7: pengalihan surat pending menyusul di E03 (belum ada entitas surat).
> E01-8: via SMTP; tanpa SMTP_HOST berjalan mode dev (link reset dicetak ke log API).
> Tambahan di luar rencana: endpoint template import xlsx, logout-all devices, halaman web login/lupa-reset password/organisasi/pengguna.

| ID | Ticket | AC singkat | SP |
|----|--------|-----------|----|
| E01-1 | Login email/NIK + password, kebijakan kekuatan, lockout 5× gagal | Sesuai P0-10 | 5 |
| E01-2 | Manajemen sesi: JWT/refresh token, timeout, logout semua perangkat | | 3 |
| E01-3 | CRUD struktur organisasi hierarkis (Direktorat→Biro→Dept→Division) + atribut lokasi (HO/Reg I/Reg II/Rep. Office) | Snapshot versi struktur; struktur aktif ter-input penuh | 8 |
| E01-4 | Master jabatan & penempatan pengguna→jabatan (dukung rangkap/Plt) | Mutasi pegawai tidak mengubah surat historis | 5 |
| E01-5 | RBAC: role Pembuat/Approver/Sekretaris/Auditor/Admin + permission matrix | Uji akses negatif otomatis | 5 |
| E01-6 | Import pengguna via Excel + validasi & report error per baris | 500 baris < 1 menit | 3 |
| E01-7 | Nonaktifkan pengguna + wizard pengalihan surat pending | Tidak ada surat yatim | 3 |
| E01-8 | Reset password self-service via email | | 2 |

## EPIC 02 — Template & Pembuatan Surat (P0-2)

| ID | Ticket | AC singkat | SP |
|----|--------|-----------|----|
| E02-1 | Master jenis surat (8 jenis awal PRD §7.1) + klasifikasi default | Admin dapat CRUD | 3 |
| E02-2 | Engine template: kop per perusahaan, placeholder field, posisi ttd & QR | Template Nota Dinas & Memo jadi acuan | 8 |
| E02-3 | Composer surat (rich text terbatas: heading, list, tabel, bold/italic) | Autosave 30 dtk; validasi field wajib | 8 |
| E02-4 | Pemilih penerima To/CC berbasis jabatan/unit dengan pencarian | Multi-penerima; broadcast ke unit; lintas direktorat hanya untuk pembuat `dept_head+` dan wajib target jabatan | 5 |
| E02-5 | Upload lampiran multi-file (25 MB/file, whitelist tipe, scan antivirus) | | 5 |
| E02-6 | Preview & render PDF final + QR verifikasi | PDF pixel-perfect vs template resmi | 8 |
| E02-7 | Halaman verifikasi QR publik-terbatas (metadata + status keaslian) | Scan QR → info surat tanpa isi konten | 3 |

## EPIC 03 — Workflow Engine Approval (P0-3)

| ID | Ticket | AC singkat | SP |
|----|--------|-----------|----|
| E03-1 | Engine rute: resolve rantai atasan dari hierarki + matrix per jenis surat | Uji kasus SK: Dept HRGA→GM→Dir→VP→Presdir | 13 |
| E03-2 | Konfigurasi approval matrix oleh admin (jenis surat → level akhir, serial/paralel) | Tanpa coding | 8 |
| E03-3 | Aksi approver: Setujui / Tolak (alasan wajib) / Minta Revisi | Approver tidak bisa edit isi | 5 |
| E03-4 | Alur revisi: kembali ke pembuat, versi isi surat ter-track | Diff antar versi terlihat | 5 |
| E03-5 | Delegasi wewenang berbatas waktu + pencatatan "a.n." | Aktif/berakhir otomatis sesuai tanggal | 5 |
| E03-6 | SLA per jenis surat + reminder 50%/100% + eskalasi ke atasan | Ref P0-8 | 5 |
| E03-7 | Pembatalan surat oleh pembuat (sebelum approval final) dengan jejak | | 2 |
| E03-8 | Mode sekretaris: draft atas nama pimpinan, konfirmasi pimpinan sebelum diajukan | Ref user story #12 | 5 |

## EPIC 04 — Penomoran Otomatis (P0-4)

| ID | Ticket | AC singkat | SP |
|----|--------|-----------|----|
| E04-1 | Master format penomoran per perusahaan/unit/jenis (pattern configurable) | Contoh: `045/HRGA-HO/ND/VII/2026` | 5 |
| E04-2 | Counter transaksional anti-duplikat (uji concurrency) + reset tahunan | 100 approval paralel → nomor unik semua | 5 |
| E04-3 | Penomoran terbit hanya saat approval final; kompensasi bila gagal render PDF | Tidak ada nomor hangus | 3 |

## EPIC 05 — Distribusi, Inbox & Disposisi (P0-5)

| ID | Ticket | AC singkat | SP |
|----|--------|-----------|----|
| E05-1 | Inbox: tab Surat Masuk (To), Tembusan (CC), Menunggu Aksi Saya, Terkirim | Badge jumlah belum dibaca | 8 |
| E05-2 | Read receipt per penerima | Tercatat di timeline | 3 |
| E05-3 | Disposisi: instruksi + tenggat + multi-penerima, berantai, terhubung surat induk | | 8 |
| E05-4 | Status tindak lanjut disposisi (proses/selesai + laporan/bukti) | Pemberi disposisi lihat status semua penerima | 5 |
| E05-5 | Fitur komentar per surat (internal, bukan bagian isi resmi) | | 3 |

## EPIC 06 — Tracking, Timeline & Audit Trail (P0-6, P0-10)

| ID | Ticket | AC singkat | SP |
|----|--------|-----------|----|
| E06-1 | Timeline visual per surat (semua event + aktor + timestamp + durasi antar step) | Indikator "menunggu di X selama N jam" | 5 |
| E06-2 | Audit log immutable (append-only) semua aksi termasuk read/download/export | Tidak ada API delete/update log | 5 |
| E06-3 | Halaman auditor: akses baca lintas unit sesuai penugasan + ekspor Excel/PDF | Ref user story #14 | 5 |

## EPIC 07 — Arsip & Pencarian (P0-7)

| ID | Ticket | AC singkat | SP |
|----|--------|-----------|----|
| E07-1 | Arsip otomatis surat terbit; folder virtual per unit/tahun/jenis | | 3 |
| E07-2 | Pencarian full-text (nomor, perihal, isi, pembuat, unit, tanggal, jenis) | < 3 dtk @100k surat; hormati klasifikasi akses | 8 |
| E07-3 | Kebijakan retensi per jenis + penandaan kedaluwarsa (tanpa hard delete) | | 3 |

## EPIC 08 — Notifikasi (P0-8)

| ID | Ticket | AC singkat | SP |
|----|--------|-----------|----|
| E08-1 | Service notifikasi terpusat (event-driven) + template pesan | | 5 |
| E08-2 | Notifikasi in-app web (bell + daftar) | | 3 |
| E08-3 | Email notifikasi (SMTP) dengan deep link | | 3 |
| E08-4 | Push Android via FCM | < 1 menit dari event | 3 |
| E08-5 | Preferensi notifikasi per pengguna (kanal per jenis event) | | 3 |

## EPIC 09 — Aplikasi Android (P0-9) — Fase 2

| ID | Ticket | AC singkat | SP |
|----|--------|-----------|----|
| E09-1 | Skeleton app, login + biometrik, manajemen sesi | | 8 |
| E09-2 | Inbox & daftar "Menunggu Aksi Saya" | | 8 |
| E09-3 | Detail surat + viewer PDF/lampiran | | 8 |
| E09-4 | Aksi approve/tolak/revisi + disposisi | Retry otomatis di jaringan lemah | 8 |
| E09-5 | Offline: cache surat terbuka + antrian aksi offline tersinkron | Uji mode pesawat → online | 13 |
| E09-6 | Push notification FCM + deep link ke surat | | 5 |
| E09-7 | Pencarian arsip | | 5 |
| E09-8 | Keamanan: screenshot-block untuk surat Rahasia, root detection dasar | | 5 |
| E09-9 | Distribusi internal (Play private track / managed) + update mekanisme | | 3 |

## EPIC 10 — Keamanan & Hardening (P0-10)

| ID | Ticket | AC singkat | SP |
|----|--------|-----------|----|
| E10-1 | ✅ (14 Jul 2026) Enforcement klasifikasi Biasa/Terbatas/Rahasia di semua endpoint | Uji akses URL langsung ditolak & tercatat — enforcement terverifikasi konsisten di detail/inbox/search/dashboard/unduh/komentar; penolakan kini tercatat `access_denied` di audit_logs. Lihat `docs/LAPORAN-UJI-E10-1-E10-3.md` | 5 |
| E10-2 | Watermark PDF (nama pembuka) untuk kelas Rahasia | | 3 |
| E10-3 | ✅ (14 Jul 2026) Rate limiting, CSRF, security headers, dependency audit | Rate limit Redis (auth 15/mnt per IP + `TRUSTED_PROXIES`, API 300/mnt per user), security headers, CSRF n/a by design (Bearer header), govulncheck+npm audit di CI, path id non-UUID → 404 | 3 |
| E10-4 | Penetration test internal + perbaikan temuan | Sebelum go-live pilot | 8 |

## EPIC 11 — Pilot & Rollout — Fase 3

| ID | Ticket | AC singkat | SP |
|----|--------|-----------|----|
| E11-1 | Input master data organisasi Rev. 8 lengkap + pengguna pilot | | 5 |
| E11-2 | Digitalisasi template surat resmi (8 jenis) | Disetujui sekretaris korporat | 5 |
| E11-3 | Materi pelatihan + video singkat per persona | | 5 |
| E11-4 | Pilot Direktorat Information System & HRGA (4 minggu) + log isu | | 8 |
| E11-5 | UAT formal + sign-off | | 5 |
| E11-6 | Rollout bertahap HO → Reg I → Reg II → Rep. Office + champion per biro | | 8 |
| E11-7 | Runbook support lini pertama (Dept. IT Software) | | 3 |

## EPIC 12 — P1 Fast Follow (pasca evaluasi pilot)
Dashboard manajemen & laporan SLA · Template builder visual · Thread rujukan antar surat · OCR lampiran · Watermark kelas Terbatas · Mode gelap. *(Detail ticket disusun setelah evaluasi bulan pertama.)*

---

## Urutan Sprint yang Disarankan (2 minggu/sprint)

| Sprint | Fokus |
|--------|-------|
| 1–2 | E00 penuh + E01-1..5 |
| 3–4 | E01 sisa + E02-1..4 |
| 5–6 | E02 sisa + E03-1..3 (workflow inti) |
| 7 | E03 sisa + E04 |
| 8–9 | E05 + E08 |
| 10 | E06 + E07 |
| 11 | E10 + stabilisasi + UAT internal |
| 12–13 | E11 pilot berjalan; tim mulai E09 (Android) paralel |
| 14–16 | E09 selesai + rollout bertahap |

**Dependensi non-teknis yang harus jalan paralel sejak Sprint 1:** SK Direksi keabsahan approval elektronik (Q1), format penomoran resmi (Q2), matrix approval per jenis surat (Q4).
