// Runner migrasi database: go run ./cmd/migrate up|down
// Memakai file SQL ter-embed dari folder migrations/.
package main

import (
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/joho/godotenv"

	"github.com/kskgroup/eofficepro/internal/config"
	"github.com/kskgroup/eofficepro/migrations"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	src, err := iofs.New(migrations.Files, ".")
	if err != nil {
		log.Fatalf("migrations source: %v", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("migrate init: %v", err)
	}
	defer m.Close()

	dir := "up"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	switch dir {
	case "up":
		err = m.Up()
	case "down":
		err = m.Steps(-1) // turun satu versi, bukan drop semua
	default:
		log.Fatalf("argumen tidak dikenal: %s (pakai: up | down)", dir)
	}

	if err != nil && err != migrate.ErrNoChange {
		log.Fatalf("migrate %s: %v", dir, err)
	}
	log.Printf("migrate %s: selesai", dir)
}
