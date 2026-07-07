# eOffice Pro — Mobile (Flutter)

Aplikasi Android untuk approver & penerima surat (PRD Epic E09). Fokus fitur:
inbox, detail surat + lampiran, approve/tolak/revisi, disposisi, push FCM,
dan **mode offline** (cache surat + antrian aksi idempotent via `client_action_id`).

## Setup (Flutter SDK belum terpasang di mesin ini)

1. Install Flutter SDK: https://docs.flutter.dev/get-started/install/windows
2. Dari folder ini jalankan:

   ```
   flutter create . --org com.kskgroup --project-name eoffice_mobile --platforms android
   flutter pub get
   flutter run
   ```

   `flutter create .` akan melengkapi folder `android/` di sekitar
   `pubspec.yaml` dan `lib/` yang sudah ada.

3. Base URL API diatur di `lib/main.dart` (sementara) — pindahkan ke
   `--dart-define=API_BASE_URL=...` saat pipeline build dibuat.

## Arsitektur yang direncanakan

- State management: Riverpod
- HTTP: dio + interceptor JWT refresh
- Offline queue: drift (SQLite) — setiap aksi approval membawa UUID unik,
  server idempotent sehingga retry aman (lihat docs/DATABASE-SCHEMA.md)
- Push: firebase_messaging (FCM)
