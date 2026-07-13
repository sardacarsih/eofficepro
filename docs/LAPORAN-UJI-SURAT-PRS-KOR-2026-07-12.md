# Laporan Uji Surat Persetujuan dan Koordinasi

**Tanggal pengujian:** 12 Juli 2026

**Environment:** database development lokal, schema version 22

**Pembuat:** Division Creator Finance — Division Head Finance
**Kategori Persetujuan:** Operasional

## Ringkasan

Tujuh skenario diuji melalui API produksi lokal. Enam draft berhasil dibuat, lima di antaranya berhasil diajukan menjadi `in_approval`, satu dipertahankan sebagai `draft` karena pilihan level akhirnya ditolak policy, dan satu permintaan lintas direktorat ditolak sebelum draft dibuat.

| Kasus | Hasil | Level akhir | Jumlah step |
|---|---|---:|---:|
| Persetujuan sampai Department Head | Berhasil diajukan | `dept_head` | 1 |
| Persetujuan sampai Director | Berhasil diajukan | `director` | 3 |
| Persetujuan sampai VP oleh Division Head | Ditolak policy; draft tersimpan | — | 0 |
| Koordinasi satu unit | Berhasil diajukan | `dept_head` | 1 |
| Koordinasi lintas department | Berhasil diajukan | `gm` | 2 |
| Koordinasi lintas biro | Berhasil diajukan | `director` | 3 |
| Koordinasi lintas direktorat oleh Division Head | Ditolak sebelum pembuatan draft | — | 0 |

## Detail Surat Persetujuan

### PRS-DeptHead — berhasil

- ID: `fe33905e-c01a-4780-9454-f7bd45c743e3`
- Status: `in_approval`
- Level dipilih dan diresolusikan: `dept_head`
- Mode: `user_selected`
- Rute: Department Head Finance & Accounting Regional I
- Step 1 berstatus `waiting`.

Hasil sesuai harapan: Sub Department Head yang kosong dilewati dan surat langsung menunggu Department Head aktif.

### PRS-Director — berhasil

- ID: `d9eeeb69-c811-4233-904f-5170ee95d2e4`
- Status: `in_approval`
- Level dipilih dan diresolusikan: `director`
- Mode: `user_selected`
- Rute:
  1. Department Head Finance & Accounting Regional I — `waiting`
  2. GM Finance & Accounting — `pending`
  3. Director Finance & Accounting — `pending`

Hasil sesuai harapan: rute serial berhenti tepat pada Director dan tersimpan menjadi tiga `approval_steps`.

### PRS-VP-NoHolder — ditolak policy

- ID draft: `e3765c98-c148-42e2-b606-91f1c3f4e84c`
- Status: `draft`
- Level yang diminta: `vp_director`
- Pesan validasi: `level akhir harus berada dalam batas kebijakan`
- Tidak ada `approval_steps` yang dibuat.

Hasil sesuai harapan untuk policy sekarang: Division Head hanya diizinkan memilih sampai Director. Draft tetap aman dan dapat diperbaiki tanpa menghasilkan rute parsial.

## Detail Surat Koordinasi

### KOR-SameUnit — berhasil

- ID: `bdbc85a7-9168-4bb6-a294-8032570b2c1e`
- Status: `in_approval`
- Cakupan: `same_unit`
- Level hasil perhitungan: `dept_head`
- Rute: Department Head Finance & Accounting Regional I — `waiting`

### KOR-CrossDepartment — berhasil

- ID: `efd9f0d3-08d5-4d0d-bd15-2cbc64bd264b`
- Status: `in_approval`
- Penerima berada pada department berbeda dalam biro Finance.
- Cakupan: `cross_department`
- Level hasil perhitungan: `gm`
- Rute:
  1. Department Head Finance & Accounting Regional I — `waiting`
  2. GM Finance & Accounting — `pending`

### KOR-CrossBiro — berhasil

- ID: `6aea0a3b-74e3-4a0a-929d-91c6062c3f5a`
- Status: `in_approval`
- Cakupan: `cross_biro`
- Level hasil perhitungan: `director`
- Rute:
  1. Department Head Finance & Accounting Regional I — `waiting`
  2. GM Finance & Accounting — `pending`
  3. Director Finance & Accounting — `pending`

### KOR-CrossDirectorate-Rejected — ditolak

- Tidak ada draft yang dibuat.
- Pembuat: Division Head Finance.
- Target: Department Head HO HRGA pada direktorat lain.
- Pesan validasi: `surat lintas direktorat hanya dapat dibuat oleh level manager ke atas`.

Hasil sesuai aturan keamanan penerima saat ini: `division_head` belum termasuk level manager lintas direktorat. Kasus ini perlu keputusan bisnis bila Division Head seharusnya boleh melakukan koordinasi lintas direktorat.

## Temuan Data dan Risiko

1. Rantai Finance lengkap sampai President Director, tetapi jabatan VP Director dan President Director belum mempunyai pemegang aktif. Rute ke level tersebut akan gagal pada validasi pemegang sebelum submit.
2. Policy Persetujuan untuk `division_head` membatasi level maksimum pada `director`; karena itu pengajuan langsung sampai VP ditolak sebelum pemeriksaan pemegang.
3. Mekanisme melewati Sub Department Head kosong bekerja pada seluruh kasus yang berhasil.
4. Revalidasi pada submit bekerja: level dan cakupan tidak hanya dipercaya dari payload atau preview klien.
5. Semua surat yang berhasil memiliki tepat satu step `waiting`; step selanjutnya tetap `pending`, sesuai approval serial.

## Rekomendasi

1. Tempatkan pemegang definitif/Plt/Plh aktif pada VP Director dan President Director sebelum membuka policy sampai level tersebut.
2. Konfirmasi apakah Division Head memang dilarang mengirim lintas direktorat. Jika diperbolehkan, ubah aturan penerima lintas direktorat secara eksplisit dan tambahkan regression test.
3. Uji lanjutan menggunakan pembuat level Department Head atau Director untuk cakupan `cross_directorate` dan `corporate` setelah pemegang VP/PD tersedia.
4. Selesaikan approval pada lima surat uji menggunakan akun Department Head, GM, dan Director untuk memverifikasi transisi `waiting → approved → published` serta penerbitan nomor dan PDF final.
5. Tandai atau arsipkan surat uji setelah verifikasi selesai agar tidak tercampur dengan data operasional.

## Verifikasi Perbaikan

Seluruh rekomendasi teknis di atas ditindaklanjuti pada schema version 22:

- VP Director dan President Director kini mempunyai pemegang aktif hasil seed idempoten.
- Division Head diizinkan mengirim Koordinasi lintas direktorat secara konsisten pada backend, web, dan mobile.
- Kasus lintas direktorat `09de495e-fd80-42e5-98a1-a8c4bd240961` berhasil masuk `in_approval` dan diresolusikan sampai `vp_director`.
- Kasus target office `f5535b94-5c2a-4efb-9130-7f37594fe7da` berhasil dideteksi sebagai `corporate` dan diresolusikan sampai `president_director`.
- Policy Operasional, SDM, Keuangan, dan Legal kini mempunyai profil batas kewenangan yang berbeda.
- Draft Persetujuan baru divalidasi sebelum disimpan; kategori, level akhir, dan keberadaan rute wajib valid.
- Mobile mengambil `allowed_levels` dari preview backend, bukan lagi daftar level statis.
- UI admin baru tersedia untuk kategori, cakupan Koordinasi, mode resolusi, serta batas minimum/maksimum policy.
- Pengujian end-to-end Persetujuan Legal `f36f50f9-45a3-40c9-be52-eb92109a12ed` berhasil melewati Director → VP → President Director, memperoleh nomor `0001/PRS/DEP-IT-SW/VII/2026`, menghasilkan PDF final, dan mencapai status `published`.
- Bug query publikasi PDF yang tidak mengambil `body_html` telah diperbaiki dan job retry berhasil dipublikasikan.
