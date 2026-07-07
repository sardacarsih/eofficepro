# eOffice Pro — Ringkasan Eksekutif untuk Direksi
**PT Kalimantan Sawit Kusuma Group** · Juli 2026 · 1 halaman

---

## Apa
Aplikasi **surat menyurat internal digital** (web + Android) untuk seluruh KSK Group: pembuatan surat dari template resmi, **persetujuan berjenjang elektronik**, disposisi & pelacakan real-time, dan arsip digital yang dapat dicari dalam hitungan detik — menggantikan surat kertas.

## Mengapa Sekarang
- Approval surat fisik **menunggu pejabat berada di lokasi** — keputusan lintas HO Pontianak ↔ Regional I/II ↔ Rep. Office tertunda berhari-hari.
- **Tidak ada visibilitas**: pengirim tidak tahu surat berhenti di meja siapa.
- Arsip kertas **rawan hilang dan sulit diaudit** — risiko temuan Inspectorate.
- Biaya kertas, cetak, dan kurir antar-site terus berulang.

## Nilai yang Dijanjikan
| Hari ini | Dengan eOffice Pro |
|----------|--------------------|
| Approval hitungan hari | **< 24 jam** normal, **< 4 jam** urgent — approve dari HP di mana pun |
| "Suratnya di mana ya?" | Status real-time + pengingat & eskalasi otomatis |
| Cari arsip berjam-jam | **< 30 detik**, full-text |
| Jejak persetujuan di kertas | Audit trail digital lengkap untuk Inspectorate |
| Cetak semua surat | Volume cetak turun **≥ 80%** dalam 6 bulan |

## Cakupan v1 (dan yang belum)
**Termasuk:** 8 jenis surat internal (memo, nota dinas, SK, surat edaran, surat tugas, undangan, berita acara, SP) · approval berjenjang sesuai Struktur Organisasi Rev. 8 · delegasi saat pejabat cuti/dinas · penomoran otomatis · klasifikasi kerahasiaan (Biasa/Terbatas/Rahasia) · Android dengan mode offline untuk site bersinyal lemah.
**Belum (v2):** surat eksternal, iOS, tanda tangan tersertifikasi PSrE, integrasi HRIS.

## Rencana
| Fase | Isi | Durasi |
|------|-----|--------|
| Persiapan | Kebijakan penomoran & keabsahan approval elektronik, template resmi | 3–4 minggu |
| MVP Web + pilot | Pilot di Direktorat Information System & HRGA | 10–14 minggu |
| Android | Fokus kebutuhan approver di lapangan | +6–8 minggu |
| Rollout Group | HO → Regional I → Regional II → Rep. Office | 8–12 minggu |

## Keputusan yang Dibutuhkan dari Direksi
1. **SK keabsahan approval elektronik internal** — prasyarat go-live; tanpa ini pengguna akan tetap meminta tanda tangan basah. *(Direksi + Legal)*
2. **Hosting**: server on-premise HO Pontianak vs cloud — menentukan arsitektur & biaya. *(Dir. Information System)*
3. **Build in-house (Dept. IT Software) vs vendor** — menentukan anggaran & timeline. *(Direksi)*
4. **Sponsor eksekutif** untuk mendorong adopsi (rekomendasi: Vice President Director).

## Ukuran Keberhasilan (dievaluasi 1 / 3 / 6 bulan)
≥ 90% surat internal via sistem · median approval < 24 jam · ≥ 50% approval Direksi/GM via Android · cetak turun ≥ 80% · nol temuan audit "dokumen persetujuan tidak lengkap".

---
*Detail lengkap: [PRD-eOfficePro.md](../PRD-eOfficePro.md) · Backlog: [docs/BACKLOG.md](BACKLOG.md) · Skema data: [docs/DATABASE-SCHEMA.md](DATABASE-SCHEMA.md)*
