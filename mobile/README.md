# eOffice Pro — Mobile (Flutter)

Aplikasi Android untuk pembuat, approver, dan penerima surat (PRD Epic E09).
Fokus fitur: dashboard ringkas, Tulis Surat berbasis template, pengelolaan
draft dan lampiran, preview PDF, pengajuan approval, inbox approval, surat
masuk, detail surat, approve/tolak/revisi, disposisi, dan pencarian arsip.
Implementasi saat ini **online-only**: aksi yang gagal karena jaringan tidak
disimpan offline dan pengguna diminta mencoba ulang.

Target device utama: Samsung Tab S10 FE 5G, 10.9", 8GB RAM, Android. Layout
utama dioptimalkan untuk landscape tablet dengan panel daftar + detail; portrait
tetap didukung sebagai tampilan satu panel.

## Setup

1. Install Flutter SDK: https://docs.flutter.dev/get-started/install/windows
2. Dari folder ini jalankan:

   ```
   flutter create . --org com.fkkg --project-name eoffice_mobile --platforms android
   flutter pub get
   flutter run
   ```

   `flutter create .` akan melengkapi folder `android/` di sekitar
   `pubspec.yaml` dan `lib/` yang sudah ada.

3. Jalankan dengan base URL API:

   ```
   flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8080/api/v1
   ```

  Untuk Samsung Tab fisik, ganti `10.0.2.2` dengan URL/IP server API yang
  dapat dijangkau lewat Wi-Fi 6 atau 5G/VPN internal.

## Firebase Cloud Messaging

FCM bersifat opt-in agar build dev tetap berjalan tanpa konfigurasi Firebase.

1. Tambahkan aplikasi Android `com.fkkg.eoffice` di Firebase Console.
2. Simpan `google-services.json` dari Firebase ke `android/app/` secara lokal
   atau gunakan konfigurasi FlutterFire yang setara. Jangan commit file ini.
3. Jalankan aplikasi dengan:

   ```powershell
   flutter run -d emulator-5554 `
     --dart-define=API_BASE_URL=http://10.0.2.2:8080/api/v1 `
     --dart-define=FIREBASE_MESSAGING_ENABLED=true
   ```

4. Di backend, aktifkan env `FIREBASE_CLOUD_MESSAGING_ENABLED=true` dan isi
   `FIREBASE_PROJECT_ID` plus salah satu dari `FIREBASE_CREDENTIALS_FILE` atau
   `FIREBASE_CREDENTIALS_JSON`.

Untuk tablet emulator development, dari root repo bisa langsung pakai:

```powershell
.\run_tablet_emulator.bat
```

Launcher ini menunggu AVD `Pixel_Tablet_API_35`, memastikan jaringan emulator
tervalidasi, memastikan API dev host `http://10.0.2.2:8080/api/v1` reachable,
memastikan Google Play Services tersedia untuk FCM, lalu menjalankan app dengan
`FIREBASE_MESSAGING_ENABLED=true`.

## Arsitektur

- State management: Riverpod
- Navigasi: GoRouter dengan auth guard
- HTTP: Dio + interceptor JWT refresh
- Token: flutter_secure_storage
- Pemilihan lampiran: file_picker dengan upload multipart dari file cache
- Draft composer: autosave 30 detik setelah data wajib lengkap
- Isi surat: editor rich-text native Android (PlatformView + EditText) dengan
  tebal/miring/garis bawah/daftar; HTML template di luar subset yang dapat
  diedit tetap dikunci read-only dan dikirim apa adanya
- Lampiran/PDF: dibuka dari URL online tanpa cache permanen
- Aksi approval tetap membawa `client_action_id` untuk idempotency server,
  tetapi tidak ada offline queue.
- Cleartext HTTP hanya diizinkan pada debug/profile build untuk pengujian
  internal. Release build sebaiknya memakai HTTPS.
