# PRD — eOffice Pro: Sistem Surat Menyurat Internal Digital
## PT Kalimantan Sawit Kusuma Group (KSK Group)

| | |
|---|---|
| **Dokumen** | Product Requirements Document (PRD) |
| **Versi** | 1.0 — Draft |
| **Tanggal** | 7 Juli 2026 |
| **Produk** | eOffice Pro (Web App + Android App) |
| **Referensi** | Struktur Organisasi KSK Group Rev. 8 (01-01-2026), disahkan Pontianak 24 Februari 2026 |
| **Status** | Menunggu review stakeholder |

---

## 1. Ringkasan Eksekutif

eOffice Pro adalah aplikasi surat menyurat internal (korespondensi dinas) berbasis **web dan Android** untuk seluruh entitas KSK Group. Aplikasi menggantikan alur surat fisik/manual dengan alur digital penuh: **pembuatan → persetujuan berjenjang (approval) → distribusi/disposisi → pelacakan (tracking) → pengarsipan (archiving)** — mendukung inisiatif **paperless office**.

Cakupan organisasi mengikuti struktur aktif KSK Group: President Director, Vice President Director, Inspectorate (IA & Risk Management), 10 Direktorat beserta Biro, Department, Sub Department Head, Division Head, hingga level Assistant — termasuk struktur **Regional I & Regional II**, **Head Office Pontianak (Graha Fajar)**, serta **Representative Office Jakarta dan Pangkalan Bun**.

---

## 2. Latar Belakang & Problem Statement

### Masalah
Proses surat menyurat internal KSK Group saat ini berjalan manual/berbasis kertas dan tersebar di banyak lokasi (HO Pontianak, kebun/mill di Regional I & II, Rep. Office Jakarta & Pangkalan Bun). Akibatnya:

1. **Approval lambat** — surat fisik harus menunggu pejabat berada di lokasi; pejabat yang sering berpindah antara HO dan site menjadi bottleneck.
2. **Tidak ada visibilitas** — pengirim tidak tahu posisi surat (sudah dibaca? di meja siapa? ditolak kenapa?).
3. **Arsip rawan hilang & sulit dicari** — pencarian surat lama memakan waktu berjam-jam; tidak ada jejak audit yang andal untuk kebutuhan Inspectorate/Internal Audit.
4. **Biaya & risiko** — biaya kertas, cetak, kurir antar-site; risiko surat rahasia terbaca pihak yang tidak berwenang.

### Dampak jika tidak diselesaikan
Keputusan operasional tertunda (terutama lintas HO–Regional), risiko temuan audit karena jejak persetujuan tidak terdokumentasi, dan biaya administrasi yang terus berulang.

### Siapa yang terdampak
Seluruh karyawan struktural dari level Assistant hingga President Director (±semua unit pada 10 direktorat + Inspectorate), dengan intensitas tertinggi pada Sekretaris (Director/GM), Kepala Department, GM Biro, dan Direksi.

---

## 3. Goals (Tujuan)

| # | Goal | Ukuran Keberhasilan |
|---|------|---------------------|
| G1 | Mempercepat siklus persetujuan surat | Rata-rata waktu draft→approved turun dari hitungan hari menjadi **< 24 jam** (surat normal), **< 4 jam** (surat urgent) dalam 3 bulan pasca go-live |
| G2 | Visibilitas penuh posisi surat | 100% surat memiliki status real-time yang dapat dilihat pembuat & pihak terkait |
| G3 | Digitalisasi arsip & auditability | 100% surat baru terarsip digital dengan audit trail lengkap; pencarian arsip **< 30 detik** |
| G4 | Mengurangi penggunaan kertas | Pengurangan volume cetak surat internal **≥ 80%** dalam 6 bulan |
| G5 | Adopsi menyeluruh | **≥ 90%** surat internal dibuat via sistem dalam 6 bulan pasca rollout penuh |

**Goal pengguna:** membuat, menyetujui, dan menemukan surat dari mana saja (HO, kebun, mill, rep. office) via web atau Android.
**Goal bisnis:** efisiensi biaya administrasi, kepatuhan audit (mendukung kerja Inspectorate IA & Risk Management), dan tata kelola dokumen korporat.

---

## 4. Non-Goals (Di Luar Cakupan v1)

| # | Non-Goal | Alasan |
|---|----------|--------|
| NG1 | Surat eksternal keluar/masuk perusahaan (ke vendor, pemerintah, mitra) | Fokus v1 internal; alur eksternal punya kebutuhan penomoran & legalitas berbeda — kandidat v2 |
| NG2 | Aplikasi iOS | Mayoritas perangkat lapangan Android; web responsif menjadi fallback pengguna iOS |
| NG3 | Manajemen dokumen umum (kontrak, SOP, drawing teknik) | Berbeda domain (DMS); arsitektur arsip dirancang agar dapat diperluas kelak |
| NG4 | Tanda tangan elektronik tersertifikasi PSrE (BSrE/privy dsb.) | v1 memakai approval elektronik internal + QR verifikasi; integrasi PSrE dievaluasi di v2 bila dibutuhkan untuk dokumen berkonsekuensi hukum |
| NG5 | Integrasi HRIS/ERP otomatis (sinkronisasi karyawan) | v1 kelola master data pengguna manual/import Excel oleh admin; integrasi menyusul |
| NG6 | Chat/kolaborasi real-time antar pengguna | Fitur komentar per surat sudah cukup; chat adalah produk terpisah |

---

## 5. Persona & Peran

Peran dipetakan langsung dari Struktur Organisasi Rev. 8:

| Persona | Jabatan pada Struktur | Kebutuhan Utama |
|---------|----------------------|-----------------|
| **Eksekutif** | President Director, Vice President Director | Approve/tolak cepat dari mana saja (mayoritas via Android), delegasi saat berhalangan, ringkasan surat menunggu |
| **Direktur / GM Biro** | Direktur 10 direktorat, GM Biro | Approval berjenjang, disposisi ke department, monitor SLA unitnya |
| **Kepala Department / Division Head** | Department & Division semua direktorat, Regional I/II | Membuat & mereview surat, menindaklanjuti disposisi |
| **Division Head / Assistant** | Division Head, Assistant | Membuat draft, menerima disposisi, melaporkan tindak lanjut |
| **Sekretaris** | Secretary President/VP/Director/GM | Drafting atas nama pimpinan, registrasi & penomoran, distribusi, pengelolaan arsip unit |
| **Auditor** | Inspectorate IA & Risk Management | Akses baca lintas unit sesuai penugasan, audit trail lengkap, ekspor bukti audit |
| **Admin Sistem** | Direktorat Information System (Dept. IT Software) | Kelola pengguna, struktur organisasi, template, alur approval, master penomoran |

---

## 6. User Stories

### Pembuat Surat (Staff/Division Head/Department Head)
1. Sebagai **staff department**, saya ingin membuat surat dari **template resmi** (memo, nota dinas, surat edaran, dst.) agar format konsisten tanpa mengetik ulang kop/format.
2. Sebagai **pembuat surat**, saya ingin melihat **status real-time** (draft, menunggu approval di siapa, disetujui, ditolak, terdistribusi) agar tahu posisi surat tanpa bertanya.
3. Sebagai **pembuat surat**, saya ingin **notifikasi** saat surat saya disetujui/ditolak beserta alasannya agar bisa segera menindaklanjuti.
4. Sebagai **pembuat surat**, saya ingin melampirkan file pendukung (PDF, gambar, Excel) agar approver punya konteks lengkap.

### Approver (Kepala Dept, GM, Direktur, VP, Presdir)
5. Sebagai **Direktur**, saya ingin menyetujui/menolak surat **dari HP Android** — termasuk saat di kebun dengan sinyal terbatas — agar approval tidak menunggu saya kembali ke kantor.
6. Sebagai **approver**, saya ingin **menolak dengan catatan revisi** sehingga surat kembali ke pembuat dengan arahan jelas.
7. Sebagai **GM Biro**, saya ingin **mendelegasikan wewenang approval** ke pejabat pengganti selama saya cuti/dinas agar alur tidak macet.
8. Sebagai **Vice President Director**, saya ingin melihat **antrian surat menunggu** terurut prioritas & umur agar surat urgent tidak terlewat.

### Penerima & Disposisi
9. Sebagai **Direktur**, saya ingin **mendisposisikan** surat masuk ke satu/beberapa department dengan instruksi & tenggat agar tindak lanjut jelas dan terlacak.
10. Sebagai **penerima disposisi**, saya ingin menandai tindak lanjut selesai dengan bukti/laporan agar pemberi disposisi tahu status pekerjaannya.
11. Sebagai **penerima tembusan (CC)**, saya ingin surat masuk otomatis ke inbox saya sebagai "untuk diketahui" tanpa kewajiban aksi.

### Sekretaris
12. Sebagai **sekretaris direktur**, saya ingin membuat draft **atas nama pimpinan** yang tetap harus dikonfirmasi pimpinan sebelum jalan, agar workflow mencerminkan praktik kerja nyata.
13. Sebagai **sekretaris**, saya ingin **penomoran surat otomatis** sesuai kaidah penomoran per unit/jenis/tahun agar tidak ada nomor ganda atau loncat.

### Auditor & Admin
14. Sebagai **auditor Inspectorate**, saya ingin melihat **riwayat lengkap** sebuah surat (siapa membuat, mengubah, menyetujui, membaca, kapan) agar bukti audit tak terbantahkan.
15. Sebagai **admin IT**, saya ingin mengubah **struktur organisasi & alur approval tanpa coding** (drag-drop/konfigurasi) agar revisi struktur organisasi tahunan (seperti Rev. 0–8) tidak butuh pengembangan ulang.
16. Sebagai **admin IT**, saya ingin menonaktifkan pengguna yang resign dan mengalihkan surat pending-nya agar tidak ada surat yatim.

### Edge cases
17. Sebagai **approver**, jika saya tidak bertindak dalam SLA (mis. 2×24 jam), saya ingin sistem mengirim **eskalasi/reminder** ke saya dan atasan saya agar surat tidak mengendap.
18. Sebagai **pengguna di site dengan koneksi buruk**, saya ingin membaca surat yang sudah terunduh dan menyiapkan aksi **offline** yang tersinkron saat online kembali.

---

## 7. Konsep Produk

### 7.1 Jenis Surat (Master, dapat dikonfigurasi)
| Jenis | Contoh Penggunaan | Alur Umum |
|-------|-------------------|-----------|
| Memo Internal | Komunikasi antar department | 1 level approval atasan |
| Nota Dinas | Usulan/laporan ke atasan lintas jenjang | Berjenjang s.d. Direktur/VP |
| Surat Edaran | Pengumuman kebijakan ke banyak unit | Approval Direktur/Presdir → broadcast |
| Surat Keputusan (SK) | Penetapan/pengangkatan | Berjenjang s.d. Presdir |
| Surat Perintah/Tugas | Penugasan dinas | Approval atasan → penerima tugas |
| Surat Undangan Rapat | Undangan internal | 1 level approval |
| Berita Acara | Dokumentasi kejadian/serah terima | Multi-pihak menandatangani |
| Surat Peringatan (SP) | Pembinaan karyawan (HRGA) | Alur khusus HRGA, kerahasiaan tinggi |

### 7.2 Siklus Hidup Surat (State Machine)
```
Draft → Diajukan → [Review/Approval berjenjang: Menunggu → Disetujui/Ditolak/Revisi]
      → Ternomori & Terbit → Terdistribusi (To/CC) → [Disposisi → Tindak Lanjut → Selesai]
      → Diarsipkan
Cabang: Ditolak → kembali ke Draft (dengan catatan) | Dibatalkan (oleh pembuat sebelum approval final)
```

### 7.3 Alur Approval
- **Berjenjang mengikuti hierarki organisasi** (default): Assistant/Div. Head → Dept. Head → GM Biro → Direktur → VP → Presdir, berhenti di level yang dipersyaratkan jenis surat.
- **Matrix per jenis surat + nilai/kategori** dapat dikonfigurasi admin (mis. SK wajib sampai Presdir; memo cukup Dept. Head).
- **Paralel** (semua harus setuju) dan **serial** (berurutan) didukung.
- **Delegasi** dengan periode berlaku; setiap aksi delegatee tercatat "a.n." di audit trail.

### 7.4 Penomoran Otomatis
Format dapat dikonfigurasi per perusahaan/unit, contoh:
`{No.Urut}/{Kode Unit}/{Kode Jenis}/{Bulan Romawi}/{Tahun}` → `045/HRGA-HO/ND/VII/2026`
Nomor terkunci saat approval final (bukan saat draft) untuk mencegah nomor hangus.

### 7.5 Klasifikasi & Kerahasiaan
- **Biasa** — dapat dilihat unit terkait.
- **Terbatas** — hanya To/CC dan rantai approval.
- **Rahasia** — hanya To dan approver; lampiran ter-watermark nama pembuka; tidak bisa diunduh oleh CC (mis. SP karyawan, hasil audit).

---

## 8. Requirements

### 8.1 Must-Have (P0) — v1 tidak bisa rilis tanpa ini

**P0-1. Manajemen Organisasi & Pengguna**
- Master struktur organisasi hierarkis aktif (Direktorat → Biro → Department → Division/Assistant; atribut Regional I/II, HO, Rep. Office).
- Pengguna terikat ke jabatan (bukan orang) sehingga mutasi pegawai tidak merusak alur.
- Role-based access control (RBAC): Pembuat, Approver, Sekretaris, Auditor (read-only lintas unit), Admin.
- **Kriteria terima:**
  - [ ] Admin dapat import pengguna via Excel dan menempatkannya pada jabatan
  - [ ] Perubahan struktur organisasi berlaku untuk surat baru tanpa merusak surat lama (surat historis menyimpan snapshot struktur saat itu)
  - [ ] Satu orang dapat memegang lebih dari satu jabatan (rangkap/Plt)

**P0-2. Pembuatan Surat Berbasis Template**
- Editor surat dengan template per jenis surat (kop, format, penempatan tanda tangan digital & QR).
- Lampiran multi-file (PDF/JPG/PNG/XLSX/DOCX, maks. 25 MB/file).
- Penerima To/CC berupa jabatan atau unit; simpan draft otomatis.
- **Kriteria terima:**
  - [ ] Given template Nota Dinas dipilih, when pengguna mengisi konten & penerima, then preview PDF sesuai format resmi perusahaan
  - [ ] Draft tersimpan otomatis minimal setiap 30 detik
  - [ ] Surat terbit menghasilkan PDF final ber-QR verifikasi

**P0-3. Workflow Approval Berjenjang**
- Rute approval otomatis dari hierarki + matrix jenis surat; aksi: Setujui, Tolak (wajib alasan), Minta Revisi.
- Delegasi wewenang berbatas waktu; riwayat aksi (siapa, kapan, dari perangkat apa).
- **Kriteria terima:**
  - [ ] Given surat jenis SK, when diajukan oleh Dept. Head HRGA, then rute otomatis: GM Biro HRGA → Direktur HRGA → VP → Presdir
  - [ ] Penolakan mengembalikan surat ke pembuat berstatus "Revisi" dengan catatan wajib
  - [ ] Approval oleh delegatee tercatat atas nama delegator dengan penanda "a.n."
  - [ ] Approver tidak dapat mengubah isi surat (hanya approve/tolak/revisi + catatan)

**P0-4. Penomoran Otomatis**
- Counter per unit + jenis + tahun; format dapat dikonfigurasi; reset tahunan otomatis.
- **Kriteria terima:**
  - [ ] Dua surat yang disetujui bersamaan tidak pernah mendapat nomor sama (uji concurrency)
  - [ ] Nomor hanya terbit setelah approval final

**P0-5. Distribusi, Inbox & Disposisi**
- Inbox surat masuk (To/CC terpisah), tanda baca/belum, disposisi berinstruksi + tenggat ke satu/banyak unit, status tindak lanjut.
- **Kriteria terima:**
  - [ ] Penerima To mendapat notifikasi & item inbox saat surat terbit
  - [ ] Disposisi berantai (Direktur → Dept → Division) tetap terlacak ke surat induk
  - [ ] Pemberi disposisi melihat status tindak lanjut tiap penerima

**P0-6. Tracking & Timeline**
- Timeline visual per surat: setiap perpindahan status, aktor, timestamp; indikator "menunggu di X selama N jam".
- **Kriteria terima:**
  - [ ] Pembuat melihat posisi surat real-time tanpa bertanya ke siapa pun
  - [ ] Riwayat baca (read receipt) tercatat per penerima

**P0-7. Arsip & Pencarian**
- Arsip otomatis surat terbit; pencarian full-text (nomor, perihal, isi, pembuat, unit, rentang tanggal, jenis); folder virtual per unit/tahun.
- Retensi sesuai kebijakan; surat tidak dapat dihapus, hanya dibatalkan dengan jejak.
- **Kriteria terima:**
  - [ ] Pencarian mengembalikan hasil < 3 detik untuk 100.000 surat
  - [ ] Auditor dapat mengekspor daftar surat + audit trail ke Excel/PDF

**P0-8. Notifikasi**
- Push notification Android (FCM), notifikasi in-app web, dan email untuk: surat menunggu approval, hasil approval, surat masuk, disposisi, tenggat, eskalasi SLA.
- **Kriteria terima:**
  - [ ] Approver menerima push < 1 menit setelah surat masuk antriannya
  - [ ] Reminder otomatis pada 50% dan 100% SLA; eskalasi ke atasan saat SLA lewat

**P0-9. Aplikasi Android**
- Fitur inti: inbox, baca surat + lampiran, approve/tolak/revisi, disposisi, notifikasi push, pencarian.
- Baca offline untuk surat yang sudah dibuka; antrian aksi offline tersinkron saat online.
- **Kriteria terima:**
  - [ ] Approve dari Android di jaringan 3G lemah berhasil dengan retry otomatis
  - [ ] Login biometrik (sidik jari/wajah) setelah login pertama

**P0-10. Keamanan & Audit Trail**
- Autentikasi (email/NIK + password, kebijakan kekuatan password, lockout), sesi timeout, enkripsi in-transit (TLS 1.2+) & at-rest untuk lampiran.
- Klasifikasi kerahasiaan (Biasa/Terbatas/Rahasia) memengaruhi visibilitas & unduhan.
- Audit trail immutable untuk semua aksi (create, edit, approve, read, download, export).
- **Kriteria terima:**
  - [ ] Pengguna non-otorisasi yang membuka URL surat Rahasia mendapat akses ditolak & tercatat
  - [ ] QR pada PDF final memverifikasi keaslian surat saat dipindai (menampilkan metadata + status dari server)

### 8.2 Nice-to-Have (P1) — fast follow setelah v1 stabil

| # | Fitur | Catatan |
|---|-------|---------|
| P1-1 | **Dashboard & laporan manajemen** — volume surat, rata-rata waktu approval per unit, bottleneck approver, kepatuhan SLA | Penting untuk Direksi & Inspectorate; menyusul 1–2 sprint setelah rilis |
| P1-2 | **Template builder visual** untuk admin (drag-drop field) | v1 cukup template dikelola tim IT |
| P1-3 | **Balasan/rujukan antar surat** (thread: surat menjawab surat) | Metadata `merujuk pada` sudah disiapkan di skema data v1 |
| P1-4 | **Pencarian OCR pada lampiran** hasil scan | Butuh pipeline OCR Bahasa Indonesia |
| P1-5 | **Watermark dinamis** (nama+waktu pengunduh) pada semua PDF kelas Terbatas/Rahasia | v1: watermark hanya kelas Rahasia |
| P1-6 | **Mode gelap & aksesibilitas** Android/web | |
| P1-7 | **Tanda tangan gambar** (specimen ttd terpampang di PDF) selain approval elektronik | Perlu kebijakan keamanan specimen |

### 8.3 Future Considerations (P2) — pagar arsitektur, bukan komitmen

| # | Fitur | Implikasi Desain v1 |
|---|-------|---------------------|
| P2-1 | Surat eksternal (masuk/keluar) + agenda registrasi | Skema penomoran & jenis surat dibuat extensible |
| P2-2 | Integrasi tanda tangan elektronik tersertifikasi (PSrE) | Abstraksi layer "signing" agar provider dapat dipasang |
| P2-3 | Integrasi HRIS (sinkron pegawai & jabatan otomatis) | Master data pengguna punya field ID eksternal |
| P2-4 | Multi-perusahaan penuh (entitas anak KSK Group dengan kop & penomoran sendiri) | Field `company` sudah ada di skema sejak v1 |
| P2-5 | iOS app | API-first: seluruh fitur via REST API yang sama |
| P2-6 | AI assist: ringkasan surat panjang, saran disposisi | Simpan konten terstruktur, bukan hanya PDF |

---

## 9. Kebutuhan Non-Fungsional

| Aspek | Target |
|-------|--------|
| Ketersediaan | 99,5% jam kerja (05.00–22.00 WIB); maintenance window di luar jam tsb. |
| Performa | Buka inbox < 2 dtk; render surat < 3 dtk; approval action < 2 dtk |
| Skala | ± 500–2.000 pengguna aktif; 200–1.000 surat/hari; pertumbuhan arsip 5 tahun |
| Jaringan | Android tetap fungsional di 3G/sinyal lemah (payload ringan, retry, offline queue) |
| Kompatibilitas | Web: Chrome/Edge/Firefox 2 versi terakhir, responsif tablet; Android 8.0+ |
| Backup & DR | Backup harian otomatis, retensi 30 hari; RPO ≤ 24 jam, RTO ≤ 8 jam |
| Bahasa | UI Bahasa Indonesia (default); arsitektur i18n untuk Inggris kelak |
| Regulasi | UU ITE & PP 71/2019 (keabsahan dokumen elektronik); UU PDP (perlindungan data pribadi pada SP/SK personalia) |

---

## 10. Success Metrics

### Leading (0–3 bulan pasca go-live)
| Metrik | Target | Cara Ukur |
|--------|--------|-----------|
| Adopsi | ≥ 70% pengguna terdaftar login ≥ 1×/minggu | Analytics login |
| Aktivasi | ≥ 60% surat internal unit pilot dibuat via sistem di bulan pertama | Pembanding registrasi manual |
| Waktu approval | Median < 24 jam (normal), < 4 jam (urgent) | Timestamp workflow |
| Approval via mobile | ≥ 50% aksi approval Direksi/GM dari Android | Analytics per platform |
| Error rate | < 1% aksi gagal | Log aplikasi |

### Lagging (3–12 bulan)
| Metrik | Target | Cara Ukur |
|--------|--------|-----------|
| Paperless | Volume cetak surat internal turun ≥ 80% | Data pengadaan kertas/GA |
| Kepatuhan SLA | ≥ 90% surat selesai approval dalam SLA | Laporan dashboard |
| Kecepatan temu arsip | < 30 detik (dari sebelumnya jam/hari) | Sampling + survei |
| Kepuasan pengguna | Skor ≥ 4/5 survei internal semester | Survei HRGA |
| Temuan audit terkait dokumen | Nol temuan "dokumen persetujuan tidak lengkap" | Laporan Inspectorate |

Evaluasi formal: 1 bulan (pilot), 3 bulan (pasca rollout), 6 bulan (review paperless).

---

## 11. Rencana Rilis & Fase

| Fase | Lingkup | Durasi (indikatif) |
|------|---------|--------------------|
| **Fase 0 — Persiapan** | Finalisasi PRD, kebijakan penomoran & klasifikasi (SK Direksi), pemetaan template surat resmi, master data organisasi Rev. 8 | 3–4 minggu |
| **Fase 1 — MVP Web (P0-1 s.d. P0-8, P0-10)** | Web app penuh; pilot di 2 unit: **Direktorat Information System** (dogfooding) & **Direktorat HRGA HO** | 10–14 minggu |
| **Fase 2 — Android (P0-9)** | Rilis Android internal (managed distribution/Play private), fokus approver | +6–8 minggu (paralel sebagian) |
| **Fase 3 — Rollout Group** | Seluruh direktorat HO → Regional I → Regional II → Rep. Office; pelatihan & champion per biro | 8–12 minggu bertahap |
| **Fase 4 — P1** | Dashboard, template builder, OCR, dst. sesuai prioritas pasca-evaluasi | berkelanjutan |

**Dependensi kritis:**
- SK Direksi tentang keabsahan approval elektronik internal (prasyarat go-live — tanpa ini pengguna akan tetap minta tanda tangan basah).
- Kesiapan infrastruktur: server/hosting, bandwidth site Regional, perangkat Android pejabat.
- Ketersediaan tim Dept. IT Software sebagai admin & tim support lini pertama.

---

## 12. Open Questions

| # | Pertanyaan | Pemilik Jawaban | Blocking? |
|---|-----------|-----------------|-----------|
| Q1 | Apakah approval elektronik internal cukup, atau ada jenis surat (SK, SP) yang secara kebijakan wajib tanda tangan basah/PSrE? | Direksi + Legal (Dir. PR, Legal & Licensing) | **Ya** — menentukan cakupan NG4 |
| Q2 | Format penomoran resmi per unit & apakah nomor lama (manual) perlu dimigrasi ke arsip digital? | Sekretaris korporat / HRGA | Ya (P0-4) |
| Q3 | Hosting on-premise (server HO Pontianak) atau cloud? Bagaimana konektivitas site Regional II? | Direktorat Information System | **Ya** — arsitektur & DR |
| Q4 | Daftar final jenis surat + matrix approval per jenis (sampai level mana)? | Tiap Direktorat, dikonsolidasi HRGA | Ya (P0-3) |
| Q5 | Kebijakan retensi arsip per jenis surat (berapa tahun)? | Legal + Inspectorate | Tidak (bisa ditetapkan saat berjalan) |
| Q6 | Apakah komite (Remunerasi & Benefit, Adhoc, AIM) dan Supporting Team GM perlu peran khusus di alur? | VP Director office | Tidak |
| Q7 | Perangkat Android pejabat: BYOD atau perangkat perusahaan? (berdampak ke kebijakan keamanan/MDM) | IT + HRGA | Tidak |
| Q8 | Anggaran & preferensi build in-house (Dept. IT Software) vs. vendor? | Direksi + Dir. Information System | **Ya** — menentukan timeline |

---

## 13. Risiko & Mitigasi

| Risiko | Dampak | Mitigasi |
|--------|--------|----------|
| Resistensi pengguna senior terhadap approval digital | Adopsi rendah, dual-system berkepanjangan | Sponsor eksekutif (Presdir/VP), SK pemberlakuan wajib bertahap, pelatihan sekretaris sebagai champion |
| Konektivitas buruk di site kebun/mill | Approval macet di lapangan | Desain offline-first Android, payload ringan, delegasi mudah |
| Perubahan struktur organisasi tahunan (Rev. 9 dst.) | Alur approval rusak | Konfigurasi organisasi self-service (P0-1), snapshot struktur per surat |
| Data rahasia bocor (SP, hasil audit) | Risiko hukum & reputasi | Klasifikasi kerahasiaan, watermark, audit akses, enkripsi |
| Scope creep (minta fitur DMS/eksternal di tengah jalan) | Molor | Non-Goals disepakati di awal; permintaan baru masuk parking lot v2 |

---

## Lampiran A — Ringkasan Struktur Organisasi (basis konfigurasi)

Berdasarkan Struktur Organisasi KSK Group Rev. 8 (berlaku 01-01-2026):

- **President Director** — Senior Executive Assistant, Secretary
  - **Inspectorate Internal Audit & Risk Management** (langsung di bawah Presdir)
- **Vice President Director** — Executive Assistant, Secretary, Committee (Remunerasi & Benefit, Adhoc, AIM), Tim Supporting GM
- **10 Direktorat** (masing-masing dengan Secretary Director, Biro/GM + Secretary GM):
  1. Sales & Marketing — Dept: Marketing Relation; Sales & Administration
  2. Public Relation, Legal & Licensing, Partnership & CSR Program — Dept: Public Relation; Legal Advocasi & Licensing; Partnership; CSR Program (Regional I & II)
  3. Information System — Dept: IT Software; IT Hardware
  4. Finance & Accounting — Dept: HO Finance; HO Accounting; Finance & Accounting Regional I & II; Partnership
  5. Plantation — Dept: Plantation (Reg. I & II); Mapping & WMS; Research & Development
  6. Civil & Engineering — Dept: Palm Oil Mill (Reg. I & II); Packaging Fertilizer; Civil; FFB Grading
  7. Human Resources & General Affairs — Dept: HO HRGA; HRGA Regional I & II; Training Center; Rep. Office Jakarta & Pangkalan Bun
  8. Sustainability & Environment — Dept: Compliance; Environment; Safety; Health (Reg. I & II)
  9. Tax, Purchase & Port — Dept: Tax; Purchase & Logistic; Port
- **Jenjang jabatan**: Directorate → Biro → Department Head → Sub Department Head → Division Head / Assistant
- **Lokasi**: Head Office Pontianak (Graha Fajar, Jl. WR. Supratman No. 42), Regional I, Regional II, Rep. Office Jakarta, Rep. Office Pangkalan Bun

> Catatan: aplikasi harus menyimpan struktur ini sebagai **data konfigurasi**, bukan hard-code, karena struktur direvisi hampir setiap tahun (Rev. 0 tahun 2022 → Rev. 8 tahun 2026).
