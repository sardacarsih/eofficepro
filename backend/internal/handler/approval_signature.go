package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
)

const (
	signatureMIMEType       = "image/png"
	maxSignatureImageSize   = 512 * 1024
	maxSignatureImageWidth  = 1600
	maxSignatureImageHeight = 800
)

type approvalSignatureImage struct {
	Data           []byte
	MIMEType       string
	SizeBytes      int
	ChecksumSHA256 string
}

type approvalPDFSignature struct {
	StepOrder     int
	PositionTitle string
	ActorName     string
	ActedAt       time.Time
	Image         []byte
	// OnBehalf true bila aksi dilakukan delegate "a.n." posisi delegator (E03-5).
	OnBehalf bool
}

var signatureObjectSegmentRE = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func validateApprovalSignatureImage(encoded string, mimeType string) (approvalSignatureImage, error) {
	mimeType = normalizeMIMEType(mimeType)
	if mimeType == "application/octet-stream" {
		mimeType = signatureMIMEType
	}
	if mimeType != signatureMIMEType {
		return approvalSignatureImage{}, errors.New("tanda tangan wajib berupa PNG")
	}

	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return approvalSignatureImage{}, errors.New("tanda tangan wajib diisi untuk setujui")
	}
	if comma := strings.Index(encoded, ","); strings.HasPrefix(encoded, "data:") && comma >= 0 {
		encoded = encoded[comma+1:]
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(encoded)
	}
	if err != nil {
		return approvalSignatureImage{}, errors.New("data tanda tangan tidak valid")
	}
	if len(data) == 0 || len(data) > maxSignatureImageSize {
		return approvalSignatureImage{}, errors.New("ukuran tanda tangan maksimal 512 KB")
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != "png" {
		return approvalSignatureImage{}, errors.New("tanda tangan wajib berupa PNG valid")
	}
	if config.Width <= 0 || config.Height <= 0 ||
		config.Width > maxSignatureImageWidth ||
		config.Height > maxSignatureImageHeight {
		return approvalSignatureImage{}, errors.New("dimensi tanda tangan maksimal 1600x800 piksel")
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return approvalSignatureImage{}, errors.New("gagal membaca gambar tanda tangan")
	}
	if imageLooksBlank(img) {
		return approvalSignatureImage{}, errors.New("tanda tangan tidak boleh kosong")
	}

	sum := sha256.Sum256(data)
	return approvalSignatureImage{
		Data:           data,
		MIMEType:       signatureMIMEType,
		SizeBytes:      len(data),
		ChecksumSHA256: hex.EncodeToString(sum[:]),
	}, nil
}

func imageLooksBlank(img image.Image) bool {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			if r < 0xf000 || g < 0xf000 || b < 0xf000 {
				return false
			}
		}
	}
	return true
}

func signatureObjectSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return randomHex(8)
	}
	value = signatureObjectSegmentRE.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return randomHex(8)
	}
	if len(value) > 80 {
		value = value[:80]
	}
	return value
}

func (h *Handler) putApprovalSignatureObject(ctx context.Context, letterID string, stepID string, clientActionID string, signature approvalSignatureImage) (string, error) {
	if h.Minio == nil {
		return "", errors.New("object storage belum tersedia untuk tanda tangan")
	}
	objectName := fmt.Sprintf(
		"letters/%s/signatures/%s-%s.png",
		signatureObjectSegment(letterID),
		signatureObjectSegment(stepID),
		signatureObjectSegment(clientActionID),
	)
	if _, err := h.Minio.PutObject(ctx, h.Bucket, objectName, bytes.NewReader(signature.Data), int64(len(signature.Data)), minio.PutObjectOptions{
		ContentType: signature.MIMEType,
	}); err != nil {
		return "", err
	}
	return objectName, nil
}

func (h *Handler) loadApprovalPDFSignatures(ctx context.Context, tx pgx.Tx, letterID string) ([]approvalPDFSignature, error) {
	rows, err := tx.Query(ctx, `
		SELECT s.step_order, p.title, u.full_name, aa.acted_at, aa.signature_image_key,
		       aa.on_behalf_delegation_id IS NOT NULL
		FROM approval_actions aa
		JOIN approval_steps s ON s.id = aa.approval_step_id
		JOIN positions p ON p.id = s.approver_position_id
		JOIN users u ON u.id = aa.acted_by_user_id
		WHERE s.letter_id = $1
		  AND s.approval_cycle = (
		    SELECT COALESCE(MAX(approval_cycle), 1)
		    FROM approval_steps
		    WHERE letter_id = $1
		  )
		  AND aa.action = 'approve'
		  AND aa.signature_image_key IS NOT NULL
		ORDER BY s.step_order`, letterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	signatures := []approvalPDFSignature{}
	for rows.Next() {
		var item approvalPDFSignature
		var storageKey string
		if err := rows.Scan(&item.StepOrder, &item.PositionTitle, &item.ActorName, &item.ActedAt, &storageKey, &item.OnBehalf); err != nil {
			return nil, err
		}
		imageData, err := h.loadSignatureImage(ctx, storageKey)
		if err != nil {
			return nil, err
		}
		item.Image = imageData
		signatures = append(signatures, item)
	}
	return signatures, rows.Err()
}

func (h *Handler) loadSignatureImage(ctx context.Context, storageKey string) ([]byte, error) {
	if h.Minio == nil {
		return nil, errors.New("object storage belum tersedia")
	}
	object, err := h.Minio.GetObject(ctx, h.Bucket, storageKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer object.Close()

	data := bytes.Buffer{}
	if _, err := data.ReadFrom(io.LimitReader(object, maxSignatureImageSize+1)); err != nil {
		return nil, err
	}
	if data.Len() <= 0 || data.Len() > maxSignatureImageSize {
		return nil, errors.New("ukuran objek tanda tangan tidak valid")
	}
	if _, err := validateApprovalSignatureImage(base64.StdEncoding.EncodeToString(data.Bytes()), signatureMIMEType); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}
