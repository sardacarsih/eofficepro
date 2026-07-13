# Backend-specific Agent Rules

Aturan root `AGENTS.md` tetap berlaku.

- Pertahankan struktur route di `internal/server/router.go` dan enforcement di
  middleware/handler; jangan mengandalkan consumer untuk authorization.
- Gunakan `pgx` dan context request secara konsisten. Semua resource database,
  object storage, dan network harus memiliki lifecycle/error handling jelas.
- Perubahan schema selalu berupa pasangan migrasi baru di `migrations/` dan
  tetap kompatibel dengan runner di `cmd/migrate/`.
- Test handler harus mencakup authorization dan company scope ketika endpoint
  mengakses data tenant.
- Jangan menulis log yang memuat password, token, OTP, isi credential, atau
  dokumen rahasia.
- Jalankan `go test ./...` sebelum handoff; tambahkan test terfokus selama
  iterasi bila full suite lambat.

