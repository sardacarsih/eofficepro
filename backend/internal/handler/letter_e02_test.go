package handler

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
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
	}, nil)
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

func TestRenderFinalLetterPDF(t *testing.T) {
	publishedAt := time.Date(2026, 7, 8, 10, 30, 0, 0, time.UTC)
	qrToken := "verify-token"
	data := draftPreviewData{
		ID:                   "letter-1",
		CompanyCode:          "KSK",
		CompanyName:          "PT Kalimantan Sawit Kusuma",
		LetterTypeCode:       "ND",
		LetterTypeName:       "Nota Dinas",
		LetterNumber:         "0001/ND/IS/VII/2026",
		Subject:              "Persetujuan Anggaran",
		Classification:       "biasa",
		Priority:             "normal",
		CreatorName:          "Budi",
		CreatorPositionTitle: "Dept Head",
		Version:              3,
		BodyPlain:            strings.Repeat("Isi final surat. ", 20),
		QRToken:              &qrToken,
		PublishedAt:          &publishedAt,
		CreatedAt:            publishedAt,
		UpdatedAt:            publishedAt,
	}

	pdf, err := renderFinalLetterPDF(data, map[string][]string{
		"to": {"Direktur - Information System"},
		"cc": {"HRGA"},
	}, "http://localhost:3000/verify/verify-token", nil)
	if err != nil {
		t.Fatalf("renderFinalLetterPDF() error = %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("renderFinalLetterPDF() did not return PDF bytes")
	}
	if len(pdf) < 1000 {
		t.Fatalf("renderFinalLetterPDF() length = %d, want substantial PDF", len(pdf))
	}
	if !bytes.Contains(pdf, []byte("/Subtype /Image")) {
		t.Fatalf("renderFinalLetterPDF() did not embed a QR image")
	}
}

func TestGenerateVerificationQRCode(t *testing.T) {
	const verifyURL = "http://localhost:3000/verify/verify-token"
	generated, err := generateVerificationQRCode(verifyURL, verificationQRCodeImageSize)
	if err != nil {
		t.Fatalf("generateVerificationQRCode(%q) error = %v", verifyURL, err)
	}
	if generated.Payload != verifyURL {
		t.Errorf("generateVerificationQRCode(%q).Payload = %q, want %q", verifyURL, generated.Payload, verifyURL)
	}

	img, err := png.Decode(bytes.NewReader(generated.PNG))
	if err != nil {
		t.Fatalf("png.Decode(generateVerificationQRCode(%q).PNG) error = %v", verifyURL, err)
	}
	if got := img.Bounds().Dx(); got < 400 || got > verificationQRCodeImageSize {
		t.Errorf("QR image width = %d, want between 400 and %d", got, verificationQRCodeImageSize)
	}
	if got := color.GrayModel.Convert(img.At(0, 0)).(color.Gray).Y; got != 255 {
		t.Errorf("QR image corner gray value = %d, want 255 for quiet zone", got)
	}
}

func TestGenerateVerificationQRCodeRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		targetSize int
	}{
		{name: "empty_payload", payload: " ", targetSize: verificationQRCodeImageSize},
		{name: "image_too_small", payload: "http://localhost:3000/verify/token", targetSize: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := generateVerificationQRCode(tt.payload, tt.targetSize); err == nil {
				t.Errorf("generateVerificationQRCode(%q, %d) error = nil, want error", tt.payload, tt.targetSize)
			}
		})
	}
}

func TestRenderFinalLetterPDFRequiresVerificationData(t *testing.T) {
	publishedAt := time.Date(2026, 7, 8, 10, 30, 0, 0, time.UTC)
	token := "verify-token"
	baseData := draftPreviewData{
		CompanyName:          "PT Kalimantan Sawit Kusuma",
		LetterTypeCode:       "ND",
		LetterTypeName:       "Nota Dinas",
		LetterNumber:         "0001/ND/IS/VII/2026",
		Subject:              "Persetujuan Anggaran",
		Classification:       "biasa",
		Priority:             "normal",
		CreatorName:          "Budi",
		CreatorPositionTitle: "Dept Head",
		Version:              3,
		BodyPlain:            "Isi final surat.",
		PublishedAt:          &publishedAt,
	}

	tests := []struct {
		name      string
		token     *string
		verifyURL string
	}{
		{name: "missing_url", token: &token},
		{name: "missing_token", verifyURL: "http://localhost:3000/verify/verify-token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := baseData
			data.QRToken = tt.token
			if _, err := renderFinalLetterPDF(data, map[string][]string{"to": {"Direktur"}}, tt.verifyURL, nil); err == nil {
				t.Errorf("renderFinalLetterPDF() error = nil, want error for %s", tt.name)
			}
		})
	}
}

func TestRenderFinalLetterPDFWithLongBody(t *testing.T) {
	publishedAt := time.Date(2026, 7, 8, 10, 30, 0, 0, time.UTC)
	token := "verify-token"
	data := draftPreviewData{
		CompanyName:          "PT Kalimantan Sawit Kusuma",
		LetterTypeCode:       "ND",
		LetterTypeName:       "Nota Dinas",
		LetterNumber:         "0001/ND/IS/VII/2026",
		Subject:              "Dokumen Panjang",
		Classification:       "biasa",
		Priority:             "normal",
		CreatorName:          "Budi",
		CreatorPositionTitle: "Dept Head",
		Version:              3,
		BodyPlain:            strings.Repeat("Isi final surat yang panjang. ", 1000),
		QRToken:              &token,
		PublishedAt:          &publishedAt,
	}
	pdf, err := renderFinalLetterPDF(
		data,
		map[string][]string{"to": {"Direktur"}},
		"http://localhost:3000/verify/verify-token",
		nil,
	)
	if err != nil {
		t.Fatalf("renderFinalLetterPDF(long body) error = %v", err)
	}
	if got := bytes.Count(pdf, []byte("/Type /Page")); got < 2 {
		t.Errorf("renderFinalLetterPDF(long body) page markers = %d, want at least 2", got)
	}
	if !bytes.Contains(pdf, []byte("/Subtype /Image")) {
		t.Fatalf("renderFinalLetterPDF(long body) did not embed a QR image")
	}
}

func TestValidateCompanyLogo(t *testing.T) {
	tests := []struct {
		name          string
		data          []byte
		wantMIMEType  string
		wantExtension string
	}{
		{
			name:          "png",
			data:          makeTestLogo(t, "png", 256, 128),
			wantMIMEType:  "image/png",
			wantExtension: "png",
		},
		{
			name:          "jpeg",
			data:          makeTestLogo(t, "jpeg", 256, 128),
			wantMIMEType:  "image/jpeg",
			wantExtension: "jpg",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logo, extension, err := validateCompanyLogo(tt.data)
			if err != nil {
				t.Fatalf("validateCompanyLogo(%s) error = %v", tt.name, err)
			}
			if logo.MIMEType != tt.wantMIMEType {
				t.Errorf("validateCompanyLogo(%s).MIMEType = %q, want %q", tt.name, logo.MIMEType, tt.wantMIMEType)
			}
			if extension != tt.wantExtension {
				t.Errorf("validateCompanyLogo(%s) extension = %q, want %q", tt.name, extension, tt.wantExtension)
			}
			if logo.Width != 256 || logo.Height != 128 {
				t.Errorf("validateCompanyLogo(%s) dimensions = %dx%d, want 256x128", tt.name, logo.Width, logo.Height)
			}
			if len(logo.ChecksumSHA256) != 64 {
				t.Errorf("validateCompanyLogo(%s) checksum length = %d, want 64", tt.name, len(logo.ChecksumSHA256))
			}
		})
	}
}

func TestValidateCompanyLogoRejectsInvalidData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "not_image", data: []byte("not an image")},
		{name: "too_small", data: makeTestLogo(t, "png", 64, 64)},
		{name: "too_large", data: makeTestLogo(t, "png", 4097, 128)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := validateCompanyLogo(tt.data); err == nil {
				t.Errorf("validateCompanyLogo(%s) error = nil, want error", tt.name)
			}
		})
	}
}

func TestRenderPDFWithCompanyLogo(t *testing.T) {
	publishedAt := time.Date(2026, 7, 8, 10, 30, 0, 0, time.UTC)
	token := "verify-token"
	data := draftPreviewData{
		CompanyName:          "PT Kalimantan Sawit Kusuma",
		LetterTypeCode:       "ND",
		LetterTypeName:       "Nota Dinas",
		LetterNumber:         "0001/ND/IS/VII/2026",
		Subject:              "Dokumen Berlogo",
		Classification:       "biasa",
		Priority:             "normal",
		CreatorName:          "Budi",
		CreatorPositionTitle: "Dept Head",
		Version:              3,
		BodyPlain:            "Isi final surat.",
		QRToken:              &token,
		PublishedAt:          &publishedAt,
	}
	logo := makeTestLogo(t, "png", 256, 128)

	preview, err := renderLetterPreviewPDF(data, map[string][]string{"to": {"Direktur"}}, logo)
	if err != nil {
		t.Fatalf("renderLetterPreviewPDF(with logo) error = %v", err)
	}
	if got := bytes.Count(preview, []byte("/Subtype /Image")); got != 1 {
		t.Errorf("renderLetterPreviewPDF(with logo) image count = %d, want 1", got)
	}

	finalPDF, err := renderFinalLetterPDF(
		data,
		map[string][]string{"to": {"Direktur"}},
		"http://localhost:3000/verify/verify-token",
		logo,
	)
	if err != nil {
		t.Fatalf("renderFinalLetterPDF(with logo) error = %v", err)
	}
	if got := bytes.Count(finalPDF, []byte("/Subtype /Image")); got != 2 {
		t.Errorf("renderFinalLetterPDF(with logo) image count = %d, want 2 (logo and QR)", got)
	}
}

func makeTestLogo(t *testing.T, format string, width int, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 18, G: 86, B: 115, A: 255})
		}
	}

	var out bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&out, img)
	case "jpeg":
		err = jpeg.Encode(&out, img, &jpeg.Options{Quality: 90})
	default:
		t.Fatalf("makeTestLogo() format = %q, want png or jpeg", format)
	}
	if err != nil {
		t.Fatalf("makeTestLogo(%q, %d, %d) error = %v", format, width, height, err)
	}
	return out.Bytes()
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
