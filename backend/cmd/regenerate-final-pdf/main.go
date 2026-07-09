// Command regenerate-final-pdf replaces the stored final PDF for one
// published letter while preserving its publication data.
package main

import (
	"context"
	"flag"
	"log"
	"strings"

	"github.com/joho/godotenv"

	"github.com/kskgroup/eofficepro/internal/config"
	"github.com/kskgroup/eofficepro/internal/handler"
	"github.com/kskgroup/eofficepro/internal/store"
)

func main() {
	var letterID string
	flag.StringVar(&letterID, "letter-id", "", "ID surat published yang PDF finalnya akan diregenerasi")
	flag.Parse()

	letterID = strings.TrimSpace(letterID)
	if letterID == "" {
		log.Fatal("argumen -letter-id wajib diisi")
	}

	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	st, err := store.New(ctx, cfg)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	h := handler.New(st.DB, st.Redis, st.Minio, st.Bucket, cfg)
	objectName, err := h.RegenerateFinalPDF(ctx, letterID)
	if err != nil {
		log.Fatalf("regenerasi PDF final surat %s: %v", letterID, err)
	}
	log.Printf("PDF final surat %s berhasil diregenerasi pada %s", letterID, objectName)
}
