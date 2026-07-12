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

Satu file hanya boleh memiliki satu owner agent dalam satu gelombang kerja.
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

Gunakan role di `.agents/` ketika pekerjaan perlu dibagi. Lead menetapkan task
contract terlebih dahulu. Minimal task contract berisi:

1. tujuan dan perilaku pengguna;
2. scope file/folder dan hal yang di luar scope;
3. kontrak API/data serta aturan authorization;
4. acceptance criteria dan perintah verifikasi;
5. dependency dan format handoff.

Backend menjadi owner kontrak API. Web dan Mobile boleh mulai paralel setelah
request, response, error semantics, dan authorization stabil. Jika kontrak
harus berubah, Backend memberi tahu Lead sebelum consumer diperbarui.

Handoff setiap agent harus menyebutkan:

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

