# Mobile Agent

## Ownership

- `mobile/lib/`
- `mobile/test/`
- konfigurasi Flutter/Android ketika secara eksplisit termasuk scope

## Mission

Bangun pengalaman Flutter Android yang aman dan responsif untuk smartphone dan
tablet, dengan session handling dan retry semantics yang konsisten terhadap
backend.

## Working rules

- Ikuti arsitektur Riverpod, GoRouter, Dio, repository, dan secure storage yang
  sudah ada.
- Jangan mengklaim offline support: implementasi saat ini online-only kecuali
  task secara eksplisit menambah offline queue.
- Semua aksi approval menghasilkan dan mengirim `client_action_id` yang stabil
  selama retry operasi yang sama.
- Tangani expired session, timeout, retry, duplicate response, dan error API
  tanpa membuat state UI palsu.
- Pertahankan layout landscape tablet dan portrait/smartphone.
- Jangan menyimpan attachment atau credential sensitif secara permanen tanpa
  kebutuhan dan threat assessment yang jelas.
- Jaga parsing model backward-compatible jika field response bersifat opsional.

## Verification

- `flutter analyze`
- `flutter test`
- uji widget untuk state penting dan model/repository test untuk kontrak baru.

## Handoff

Tulis pada bagian `## Handoff — Mobile` di file task (`docs/tasks/`).
Laporkan screen/provider/repository/model yang berubah, perilaku retry dan
session, hasil analyze/test, perangkat/layout yang diverifikasi, serta dependency
backend yang belum tersedia.

