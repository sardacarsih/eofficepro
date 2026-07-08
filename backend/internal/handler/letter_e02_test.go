package handler

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestNormalizeMIMEType(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "with_params", in: "Application/PDF; charset=binary", want: "application/pdf"},
		{name: "empty", in: "", want: "application/octet-stream"},
		{name: "invalid_kept_lowercase", in: "IMAGE/PNG", want: "image/png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeMIMEType(tt.in); got != tt.want {
				t.Fatalf("normalizeMIMEType(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSafeObjectFileName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain", in: "memo.pdf", want: "memo.pdf"},
		{name: "path_and_spaces", in: `..\folder\memo final!.pdf`, want: "memo-final-.pdf"},
		{name: "empty", in: "", want: "lampiran"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := safeObjectFileName(tt.in); got != tt.want {
				t.Fatalf("safeObjectFileName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRenderLetterPreviewPDF(t *testing.T) {
	data := draftPreviewData{
		ID:                   "letter-1",
		CompanyCode:          "KSK",
		CompanyName:          "PT Kalimantan Sawit Kusuma",
		LetterTypeCode:       "ND",
		LetterTypeName:       "Nota Dinas",
		Subject:              "Permintaan Persetujuan",
		Classification:       "biasa",
		Priority:             "normal",
		CreatorName:          "Budi",
		CreatorPositionTitle: "Dept Head",
		Version:              2,
		BodyPlain:            strings.Repeat("Isi surat. ", 20),
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	pdf, err := renderLetterPreviewPDF(data, map[string][]string{
		"to": {"Direktur - Information System"},
		"cc": {"HRGA"},
	})
	if err != nil {
		t.Fatalf("renderLetterPreviewPDF() error = %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("renderLetterPreviewPDF() did not return PDF bytes")
	}
	if len(pdf) < 1000 {
		t.Fatalf("renderLetterPreviewPDF() length = %d, want substantial PDF", len(pdf))
	}
}

func TestRandomHexLength(t *testing.T) {
	got := randomHex(24)
	if len(got) != 48 {
		t.Fatalf("randomHex(24) length = %d, want 48", len(got))
	}
	if got == randomHex(24) {
		t.Fatalf("randomHex returned duplicate consecutive values")
	}
}
