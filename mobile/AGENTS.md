# Mobile-specific Agent Rules

Aturan root `AGENTS.md` tetap berlaku.

- Pertahankan pemisahan presentation, repository/data access, domain model, dan
  service yang sudah dipakai project.
- Gunakan Riverpod untuk state, GoRouter untuk navigation, dan Dio boundary
  untuk HTTP/session refresh.
- Aplikasi saat ini online-only. Jangan membuat asumsi adanya offline queue.
- Approval action harus mempertahankan `client_action_id` yang sama ketika user
  me-retry operasi yang sama.
- Semua fitur UI penting harus tetap bekerja pada landscape tablet dan
  portrait/smartphone.
- Jangan commit `google-services.json`, signing key, token, atau credential.
- Jalankan `flutter analyze` dan `flutter test` sebelum handoff.

