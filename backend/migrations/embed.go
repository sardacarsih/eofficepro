// Package migrations meng-embed file SQL agar runner migrasi
// dapat dijalankan tanpa CLI eksternal: go run ./cmd/migrate up
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS
