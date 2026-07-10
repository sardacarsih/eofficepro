package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestValidateDraftAttachmentContent(t *testing.T) {
	validPNG := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00}
	tests := []struct {
		name     string
		data     []byte
		fileName string
		declared string
		wantMIME string
		wantErr  bool
	}{
		{"pdf", []byte("%PDF-1.7\n"), "memo.pdf", "application/pdf", "application/pdf", false},
		{"png", validPNG, "gambar.png", "image/png", "image/png", false},
		{"spoofed_pdf", []byte("not-a-pdf"), "memo.pdf", "application/pdf", "", true},
		{"bad_extension", validPNG, "gambar.exe", "image/png", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateDraftAttachmentContent(tt.data, tt.fileName, tt.declared)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateDraftAttachmentContent() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.wantMIME {
				t.Errorf("MIME = %q, want %q", got, tt.wantMIME)
			}
		})
	}
}

func TestScanClamAVIntegration(t *testing.T) {
	address := os.Getenv("CLAMAV_ADDRESS")
	if address == "" {
		address = "localhost:3310"
	}
	result, err := scanClamAV(context.Background(), address, strings.NewReader("eOffice attachment scan"))
	if err != nil {
		t.Skipf("ClamAV tidak tersedia untuk integration test: %v", err)
	}
	if result != "clean" {
		t.Fatalf("scanClamAV(clean) = %q, want clean", result)
	}

	result, err = scanClamAV(context.Background(), address, strings.NewReader("X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*"))
	if err != nil {
		t.Fatal(err)
	}
	if result != "infected" {
		t.Fatalf("scanClamAV(EICAR) = %q, want infected", result)
	}
}

func TestOfficeDocumentKind(t *testing.T) {
	var document bytes.Buffer
	writer := zip.NewWriter(&document)
	for _, name := range []string{"[Content_Types].xml", "word/document.xml"} {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte("test")); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	kind, err := officeDocumentKind(document.Bytes())
	if err != nil {
		t.Fatalf("officeDocumentKind() error = %v", err)
	}
	if kind != "docx" {
		t.Errorf("officeDocumentKind() = %q, want docx", kind)
	}
}
