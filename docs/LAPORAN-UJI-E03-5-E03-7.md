# Laporan Uji E03-5 (Delegasi Wewenang "a.n.") dan E03-7 (Pembatalan Surat oleh Pembuat)

**Tanggal pengujian:** 13–14 Juli 2026

**Environment:** database development lokal (Postgres 16, port 5433), schema version 25 (migrasi `0025_delegation_lifecycle_and_cancel_trace` terpasang; siklus `up → down → up` terverifikasi: versi 24 → 25 → 24 → 25)

**Metode:** test integrasi Go terhadap handler dengan DB nyata (`EOFFICE_INTEGRATION_DB_URL`), probe adversarial tambahan, review kode defensif, dan regression gate tiga stack.

## Ringkasan

Seluruh acceptance criteria kedua ticket terverifikasi. Satu bug nyata ditemukan
dan diperbaiki selama gelombang ini: race cancel vs approve final semula
deadlock (SQLSTATE 40P01) karena `FOR UPDATE OF s, l` satu statement tidak
menjamin urutan lock; diperbaiki dengan urutan lock deterministik
letters → approval_steps pada `ActApprovalStep` (konsisten dengan
`CancelLetter`). Setelah perbaikan, race lulus stabil (`-count=5`, tepat satu
pemenang di tiap iterasi, surat batal tanpa nomor).

## Matriks skenario — E03-5 Delegasi

| Skenario | Test | Hasil |
|---|---|---|
| Create/list/revoke + notifikasi + audit | `TestDelegationCreateListRevoke_Integration` | PASS |
| Validasi 400 (reason kosong, format tanggal, from≥to, to lampau, self-delegation) | `TestDelegationCreateValidationAndAuthorization_Integration` | PASS |
| Overlap konkuren → tepat satu 201 + satu 409 (SQLSTATE 23P01) | `TestDelegationOverlapConcurrent_Integration` (x5) | PASS |
| Boundary waktu: scheduled/expired tidak bisa bertindak; aktif mengisi `on_behalf_delegation_id` | `TestDelegationActOnBehalfAndBoundaries_Integration` | PASS |
| Kapasitas langsung menang (`on_behalf_delegation_id` NULL); revoke seketika mencabut view/act | `TestDelegationDirectCapacityWinsAndRevokeImmediate_Integration` | PASS |
| `notifyWaitingApprovers` + SLA reminder/eskalasi menjangkau delegate | `TestDelegationNotificationsAndSLAReachDelegate_Integration` | PASS |
| Kandidat delegate se-company, tanpa pemegang posisi delegator | `TestDelegationDelegateOptions_Integration` | PASS |
| Isolasi tenant: admin company lain tidak melihat/mencabut (404)/membuat (403) delegasi company lain | `TestQADelegationCrossCompanyAdminIsolation_Integration` | PASS |
| Delegasi expired: act → 404, inbox kosong | `TestQADelegateExpiredCannotActAndInboxEmpty_Integration` | PASS |
| Idempotency `client_action_id` konkuren via delegate: tepat satu action | `TestQAActConcurrentDuplicateClientActionID_Integration` | PASS |
| Delegasi revoked tidak memblokir delegasi baru rentang sama | `TestQARevokedDelegationDoesNotBlockNewOverlap_Integration` | PASS |

## Matriks skenario — E03-7 Pembatalan

| Skenario | Test | Hasil |
|---|---|---|
| Happy path in_approval: step skipped, actions utuh, jejak terisi, notifikasi + audit | `TestCancelLetterInApproval_Integration` | PASS |
| Per status draft/revision; 400 reason kosong/kepanjangan; 404 non-pemilik & lintas company; 409 approved/published/cancelled; cancel ulang 409 | `TestCancelLetterPerStatusAndValidation_Integration` | PASS |
| Race cancel vs approve final: tepat satu menang; batal → tanpa nomor; kalah → 409 | `TestCancelLetterRaceWithApproveFinal_Integration` (x5, 15 race) | PASS (setelah fix urutan lock) |
| Delegate/approver bukan pembuat tidak bisa cancel (404); surat batal memblokir act delegate | `TestQACancelledLetterBlocksDelegateActAndDelegateCannotCancel_Integration` | PASS |

## Review kode defensif

- Seluruh query baru parameterized (`$n`); tidak ada konkatenasi input user ke
  SQL. Fragmen dinamis (`activeDelegationExistsSQL`, kondisi visibility list)
  hanya menyusun konstanta SQL, bukan input.
- Fragmen "delegasi aktif" didefinisikan sekali
  (`activeDelegationExistsSQLTemplate`, `delegation.go`) dan dipakai konsisten
  di inbox/act/view; cabang notifikasi/SLA/cancel memakai predikat identik
  (`now() >= valid_from AND now() < valid_to AND revoked_at IS NULL`).
- Tidak ada `UPDATE`/`DELETE` terhadap `audit_logs` di kode aplikasi
  (satu-satunya DELETE ada di cleanup fixture test pre-existing).
- Kontrak lintas stack dicek silang: JSON tag handler backend =
  `web/src/lib/api.ts` = model mobile `letter_models.dart`
  (`is_delegated`, `delegated_from_title`, `on_behalf_of`,
  `on_behalf_of_position_title`, `cancelled_at`, `cancelled_by_name`,
  `cancel_reason`, `can_cancel`) — tidak ada mismatch.
- Prefix `"a.n. "` blok tanda tangan PDF diverifikasi via kode
  (`approval_signature.go` flag `OnBehalf` → `letter_e02.go`
  `writeApprovalSignaturesBlock`); render PDF end-to-end belum diuji manual.

## Regression gate

| Stack | Perintah | Hasil |
|---|---|---|
| Backend | `go test ./...` (dengan integration DB) | Hanya 11 kegagalan pre-existing keluarga company-scope (`user_position_test.go`, `secretary_flow_test.go`, `TestApprovalMatrixCRUD` — "tidak punya akses ke perusahaan ini"); terkonfirmasi gagal identik pada tree bersih tanpa perubahan gelombang ini |
| Web | `npm run lint && npm run build` | Hijau (route `/delegations` ter-generate) |
| Mobile | `flutter analyze && flutter test` | Hijau — No issues, 112 test lulus |

## Temuan

| # | Severity | Temuan | Status |
|---|---|---|---|
| 1 | High | Deadlock 40P01 race cancel vs approve final (`FOR UPDATE OF s, l` satu statement) | **Diperbaiki** di gelombang ini: urutan lock letters → step pada `ActApprovalStep` (`approval_workflow.go`), diverifikasi race test x5 |
| 2 | Low | Path id non-UUID pada `DELETE /delegations/:id` dan `POST /letters/view/:id/cancel` mengembalikan 500 generik, bukan 404 | Dicatat — konsisten dengan pola endpoint existing (cast uuid gagal → error DB); kandidat perbaikan lintas endpoint terpisah, bukan blocker |
| 3 | Info | Satu run suite penuh menampilkan 1 kegagalan ekstra transien (di luar 11 pre-existing) yang tidak terreproduksi pada dua run penuh berikutnya | Dipantau; kemungkinan interferensi fixture saat run beruntun |

## Risiko tersisa

- Uji manual E2E browser/perangkat (buat delegasi via UI web, delegate approve
  dari Android, PDF final menampilkan "a.n.") belum dijalankan — perlu backend
  hidup + APK; disarankan sebelum rilis.
- 11 kegagalan pre-existing company-scope perlu ticket tersendiri (di luar
  scope E03-5/E03-7).
