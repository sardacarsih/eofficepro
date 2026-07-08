package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jung-kurt/gofpdf"
	"github.com/minio/minio-go/v7"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

const (
	maxDraftAttachmentSize = 25 * 1024 * 1024
	presignedURLTTL        = 15 * time.Minute
)

var allowedAttachmentMIMETypes = map[string]bool{
	"application/pdf": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       true,
	"application/vnd.ms-excel": true,
	"text/csv":                 true,
	"image/png":                true,
	"image/jpeg":               true,
}

type LetterAttachment struct {
	ID             string    `json:"id"`
	FileName       string    `json:"file_name"`
	MIMEType       string    `json:"mime_type"`
	SizeBytes      int64     `json:"size_bytes"`
	StorageKey     string    `json:"storage_key"`
	ChecksumSHA256 string    `json:"checksum_sha256"`
	ScanStatus     string    `json:"scan_status"`
	DownloadURL    string    `json:"download_url,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type draftPreviewData struct {
	ID                   string
	CompanyCode          string
	CompanyName          string
	LetterTypeCode       string
	LetterTypeName       string
	LetterNumber         string
	Subject              string
	Classification       string
	Priority             string
	CreatorName          string
	CreatorPositionTitle string
	Version              int
	BodyPlain            string
	QRToken              *string
	PublishedAt          *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (h *Handler) ListDraftAttachments(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	letterID := c.Param("id")

	if ok, err := h.userOwnsEditableDraft(c.Request.Context(), userID, letterID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa draft"})
		return
	} else if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "draft surat tidak ditemukan"})
		return
	}

	attachments, ok := h.loadLetterAttachments(c, letterID, true)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"attachments": attachments})
}

func (h *Handler) UploadDraftAttachment(c *gin.Context) {
	if h.Minio == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "object storage belum tersedia"})
		return
	}

	userID := c.GetString(middleware.CtxUserID)
	letterID := c.Param("id")
	ctx := c.Request.Context()

	if ok, err := h.userOwnsEditableDraft(ctx, userID, letterID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa draft"})
		return
	} else if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "draft surat tidak ditemukan"})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxDraftAttachmentSize+1024*1024)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file lampiran wajib dikirim"})
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > maxDraftAttachmentSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ukuran lampiran maksimal 25 MB"})
		return
	}

	contentType := normalizeMIMEType(fileHeader.Header.Get("Content-Type"))
	if !allowedAttachmentMIMETypes[contentType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tipe file lampiran tidak diizinkan"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gagal membaca lampiran"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxDraftAttachmentSize+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gagal membaca lampiran"})
		return
	}
	if int64(len(data)) > maxDraftAttachmentSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ukuran lampiran maksimal 25 MB"})
		return
	}

	sum := sha256.Sum256(data)
	checksum := hex.EncodeToString(sum[:])
	objectName := fmt.Sprintf("letters/%s/attachments/%s-%s", letterID, randomHex(8), safeObjectFileName(fileHeader.Filename))

	if _, err := h.Minio.PutObject(ctx, h.Bucket, objectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	}); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "gagal mengunggah lampiran ke object storage"})
		return
	}

	var attachmentID string
	err = h.DB.QueryRow(ctx, `
		INSERT INTO letter_attachments
			(letter_id, file_name, mime_type, size_bytes, storage_key, checksum_sha256, uploaded_by, scan_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'clean')
		RETURNING id::text`,
		letterID,
		strings.TrimSpace(fileHeader.Filename),
		contentType,
		len(data),
		objectName,
		checksum,
		userID,
	).Scan(&attachmentID)
	if err != nil {
		_ = h.Minio.RemoveObject(ctx, h.Bucket, objectName, minio.RemoveObjectOptions{})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan metadata lampiran"})
		return
	}

	h.audit(ctx, "letter", &letterID, "upload_attachment", &userID, map[string]any{
		"attachment_id": attachmentID,
		"file_name":     fileHeader.Filename,
		"size_bytes":    len(data),
		"scan_status":   "clean",
	}, c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"id": attachmentID})
}

func (h *Handler) DeleteDraftAttachment(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	letterID := c.Param("id")
	attachmentID := c.Param("attachment_id")
	ctx := c.Request.Context()

	if ok, err := h.userOwnsEditableDraft(ctx, userID, letterID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa draft"})
		return
	} else if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "draft surat tidak ditemukan"})
		return
	}

	var objectName string
	err := h.DB.QueryRow(ctx, `
		DELETE FROM letter_attachments
		WHERE id = $1 AND letter_id = $2
		RETURNING storage_key`, attachmentID, letterID).Scan(&objectName)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "lampiran tidak ditemukan"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menghapus lampiran"})
		return
	}
	if h.Minio != nil {
		_ = h.Minio.RemoveObject(ctx, h.Bucket, objectName, minio.RemoveObjectOptions{})
	}

	h.audit(ctx, "letter", &letterID, "delete_attachment", &userID, map[string]any{
		"attachment_id": attachmentID,
	}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": attachmentID})
}

func (h *Handler) PreviewDraftLetter(c *gin.Context) {
	if h.Minio == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "object storage belum tersedia"})
		return
	}

	userID := c.GetString(middleware.CtxUserID)
	letterID := c.Param("id")
	ctx := c.Request.Context()

	data, err := h.loadDraftPreviewData(ctx, userID, letterID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "draft surat tidak ditemukan"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat draft"})
		return
	}

	recipients, err := h.loadRecipientLabels(ctx, letterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat penerima"})
		return
	}

	pdfBytes, err := renderLetterPreviewPDF(data, recipients)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membuat preview PDF"})
		return
	}

	objectName := fmt.Sprintf("letters/%s/previews/preview-v%d-%d.pdf", letterID, data.Version, time.Now().Unix())
	if _, err := h.Minio.PutObject(ctx, h.Bucket, objectName, bytes.NewReader(pdfBytes), int64(len(pdfBytes)), minio.PutObjectOptions{
		ContentType: "application/pdf",
	}); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "gagal menyimpan preview PDF"})
		return
	}

	previewURL, err := h.presignedGetURL(ctx, objectName)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "gagal membuat URL preview"})
		return
	}

	h.audit(ctx, "letter", &letterID, "preview_pdf", &userID, map[string]any{
		"storage_key": objectName,
		"version":     data.Version,
	}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{
		"storage_key": objectName,
		"preview_url": previewURL,
		"expires_in":  int(presignedURLTTL.Seconds()),
	})
}

func (h *Handler) SubmitDraftLetter(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	letterID := c.Param("id")
	ctx := c.Request.Context()

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi"})
		return
	}
	defer tx.Rollback(ctx)

	draft, err := lockDraftForSubmit(ctx, tx, letterID, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "draft surat tidak ditemukan"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca draft"})
		return
	}
	if draft.Status != "draft" && draft.Status != "revision" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "surat sudah tidak dapat diajukan"})
		return
	}
	if strings.TrimSpace(draft.BodyPlain) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "isi surat wajib diisi sebelum diajukan"})
		return
	}

	recipients, err := loadDraftRecipientRequests(ctx, tx, letterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat penerima draft"})
		return
	}
	if err := normalizeDraftRecipients(recipients); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.validateDraftRecipientTargets(ctx, tx, draft.CreatorPositionID, recipients); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	route, err := h.resolveApprovalRoute(ctx, tx, draft.LetterTypeID, draft.CreatorPositionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateApprovalRouteHasActiveHolders(ctx, tx, route); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := tx.Exec(ctx, `DELETE FROM approval_steps WHERE letter_id = $1`, letterID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyiapkan rute approval"})
		return
	}
	for _, step := range route.Steps {
		status := "pending"
		if step.StepOrder == 1 {
			status = "waiting"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO approval_steps
				(letter_id, step_order, approver_position_id, flow_group, status, sla_deadline)
			VALUES ($1, $2, $3, $4, $5, now() + make_interval(hours => $6::int))`,
			letterID, step.StepOrder, step.PositionID, step.FlowGroup, status, route.SLAHours); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan rute approval"})
			return
		}
	}

	approverEmails, err := notifyWaitingApprovers(ctx, tx, letterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengirim notifikasi approval"})
		return
	}

	qrToken, err := h.uniqueQRToken(ctx, tx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membuat token QR"})
		return
	}
	routeJSON, _ := json.Marshal(route.Steps)

	if _, err := tx.Exec(ctx, `
		UPDATE letters
		SET status = 'in_approval',
		    current_step_order = 1,
		    route_snapshot = $2,
		    qr_token = $3,
		    updated_at = now()
		WHERE id = $1`, letterID, routeJSON, qrToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengajukan surat"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan pengajuan"})
		return
	}

	h.audit(ctx, "letter", &letterID, "submit", &userID, map[string]any{
		"approval_steps": len(route.Steps),
		"qr_token":       qrToken,
	}, c.ClientIP())
	h.sendNotificationEmails(approverEmails)
	c.JSON(http.StatusOK, gin.H{
		"id":             letterID,
		"status":         "in_approval",
		"qr_token":       qrToken,
		"verify_url":     h.verifyURL(qrToken),
		"approval_steps": route.Steps,
	})
}

func (h *Handler) VerifyLetter(c *gin.Context) {
	token := strings.TrimSpace(c.Param("token"))
	if len(token) < 16 || len(token) > 64 {
		c.JSON(http.StatusNotFound, gin.H{"error": "token verifikasi tidak ditemukan"})
		return
	}

	var publishedAt *time.Time
	var letterNumber *string
	var result struct {
		ID             string     `json:"id"`
		CompanyCode    string     `json:"company_code"`
		CompanyName    string     `json:"company_name"`
		LetterTypeCode string     `json:"letter_type_code"`
		LetterTypeName string     `json:"letter_type_name"`
		LetterNumber   *string    `json:"letter_number"`
		Subject        string     `json:"subject"`
		Classification string     `json:"classification"`
		Status         string     `json:"status"`
		PublishedAt    *time.Time `json:"published_at"`
		CreatedAt      time.Time  `json:"created_at"`
	}
	err := h.DB.QueryRow(c.Request.Context(), `
		SELECT l.id::text, co.code, co.name, lt.code, lt.name, l.letter_number,
		       l.subject, l.classification, l.status, l.published_at, l.created_at
		FROM letters l
		JOIN companies co ON co.id = l.company_id
		JOIN letter_types lt ON lt.id = l.letter_type_id
		WHERE l.qr_token = $1`, token).Scan(
		&result.ID,
		&result.CompanyCode,
		&result.CompanyName,
		&result.LetterTypeCode,
		&result.LetterTypeName,
		&letterNumber,
		&result.Subject,
		&result.Classification,
		&result.Status,
		&publishedAt,
		&result.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "token verifikasi tidak ditemukan"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memverifikasi surat"})
		return
	}
	result.LetterNumber = letterNumber
	result.PublishedAt = publishedAt

	c.JSON(http.StatusOK, gin.H{
		"valid":  true,
		"letter": result,
	})
}

func (h *Handler) userOwnsEditableDraft(ctx context.Context, userID string, letterID string) (bool, error) {
	var exists bool
	err := h.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM letters
			WHERE id = $1
			  AND creator_user_id = $2
			  AND status IN ('draft', 'revision')
		)`, letterID, userID).Scan(&exists)
	return exists, err
}

func (h *Handler) loadLetterAttachments(c *gin.Context, letterID string, includeURL bool) ([]LetterAttachment, bool) {
	rows, err := h.DB.Query(c.Request.Context(), `
		SELECT id::text, file_name, mime_type, size_bytes, storage_key,
		       checksum_sha256, scan_status, created_at
		FROM letter_attachments
		WHERE letter_id = $1
		ORDER BY created_at DESC`, letterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat lampiran"})
		return nil, false
	}
	defer rows.Close()

	attachments := []LetterAttachment{}
	for rows.Next() {
		var item LetterAttachment
		if err := rows.Scan(
			&item.ID,
			&item.FileName,
			&item.MIMEType,
			&item.SizeBytes,
			&item.StorageKey,
			&item.ChecksumSHA256,
			&item.ScanStatus,
			&item.CreatedAt,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca lampiran"})
			return nil, false
		}
		if includeURL && h.Minio != nil {
			item.DownloadURL, _ = h.presignedGetURL(c.Request.Context(), item.StorageKey)
		}
		attachments = append(attachments, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar lampiran"})
		return nil, false
	}
	return attachments, true
}

func (h *Handler) loadDraftPreviewData(ctx context.Context, userID string, letterID string, editableOnly bool) (draftPreviewData, error) {
	var data draftPreviewData
	statusSQL := ""
	if editableOnly {
		statusSQL = "AND l.status IN ('draft', 'revision')"
	}
	err := h.DB.QueryRow(ctx, `
		SELECT l.id::text, co.code, co.name, lt.code, lt.name,
		       l.subject, l.classification, l.priority, u.full_name,
		       p.title, COALESCE(v.version, 0), COALESCE(v.body_plain, ''),
		       l.qr_token, l.created_at, l.updated_at
		FROM letters l
		JOIN companies co ON co.id = l.company_id
		JOIN letter_types lt ON lt.id = l.letter_type_id
		JOIN users u ON u.id = l.creator_user_id
		JOIN positions p ON p.id = l.creator_position_id
		LEFT JOIN LATERAL (
			SELECT version, body_plain
			FROM letter_versions
			WHERE letter_id = l.id
			ORDER BY version DESC
			LIMIT 1
		) v ON true
		WHERE l.id = $1
		  AND l.creator_user_id = $2
		`+statusSQL,
		letterID,
		userID,
	).Scan(
		&data.ID,
		&data.CompanyCode,
		&data.CompanyName,
		&data.LetterTypeCode,
		&data.LetterTypeName,
		&data.Subject,
		&data.Classification,
		&data.Priority,
		&data.CreatorName,
		&data.CreatorPositionTitle,
		&data.Version,
		&data.BodyPlain,
		&data.QRToken,
		&data.CreatedAt,
		&data.UpdatedAt,
	)
	return data, err
}

func (h *Handler) loadRecipientLabels(ctx context.Context, letterID string) (map[string][]string, error) {
	return loadRecipientLabels(ctx, h.DB, letterID)
}

type recipientLabelQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func loadRecipientLabels(ctx context.Context, q recipientLabelQuerier, letterID string) (map[string][]string, error) {
	rows, err := q.Query(ctx, `
		SELECT lr.recipient_type,
		       CASE
		         WHEN lr.position_id IS NOT NULL THEN p.title || ' - ' || pou.name
		         ELSE ou.name || ' (' || ou.code || ')'
		       END AS label
		FROM letter_recipients lr
		LEFT JOIN positions p ON p.id = lr.position_id
		LEFT JOIN org_units pou ON pou.id = p.org_unit_id
		LEFT JOIN org_units ou ON ou.id = lr.org_unit_id
		WHERE lr.letter_id = $1
		ORDER BY lr.recipient_type DESC, label`, letterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	recipients := map[string][]string{"to": {}, "cc": {}}
	for rows.Next() {
		var recipientType, label string
		if err := rows.Scan(&recipientType, &label); err != nil {
			return nil, err
		}
		recipients[recipientType] = append(recipients[recipientType], label)
	}
	return recipients, rows.Err()
}

func loadFinalLetterPDFData(ctx context.Context, tx pgx.Tx, letterID string) (draftPreviewData, error) {
	var data draftPreviewData
	var letterNumber *string
	err := tx.QueryRow(ctx, `
		SELECT l.id::text, co.code, co.name, lt.code, lt.name,
		       l.letter_number, l.subject, l.classification, l.priority,
		       u.full_name, p.title, COALESCE(v.version, 0), COALESCE(v.body_plain, ''),
		       l.qr_token, l.published_at, l.created_at, l.updated_at
		FROM letters l
		JOIN companies co ON co.id = l.company_id
		JOIN letter_types lt ON lt.id = l.letter_type_id
		JOIN users u ON u.id = l.creator_user_id
		JOIN positions p ON p.id = l.creator_position_id
		LEFT JOIN LATERAL (
			SELECT version, body_plain
			FROM letter_versions
			WHERE letter_id = l.id
			ORDER BY version DESC
			LIMIT 1
		) v ON true
		WHERE l.id = $1`, letterID).Scan(
		&data.ID,
		&data.CompanyCode,
		&data.CompanyName,
		&data.LetterTypeCode,
		&data.LetterTypeName,
		&letterNumber,
		&data.Subject,
		&data.Classification,
		&data.Priority,
		&data.CreatorName,
		&data.CreatorPositionTitle,
		&data.Version,
		&data.BodyPlain,
		&data.QRToken,
		&data.PublishedAt,
		&data.CreatedAt,
		&data.UpdatedAt,
	)
	if letterNumber != nil {
		data.LetterNumber = *letterNumber
	}
	return data, err
}

func (h *Handler) renderAndStoreFinalPDF(ctx context.Context, tx pgx.Tx, letterID string, letterNumber string, publishedAt time.Time) (string, error) {
	if h.Minio == nil {
		return "", errors.New("object storage belum tersedia untuk PDF final")
	}

	data, err := loadFinalLetterPDFData(ctx, tx, letterID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errors.New("surat tidak ditemukan untuk PDF final")
	}
	if err != nil {
		return "", errors.New("gagal memuat data PDF final")
	}
	data.LetterNumber = letterNumber
	data.PublishedAt = &publishedAt

	recipients, err := loadRecipientLabels(ctx, tx, letterID)
	if err != nil {
		return "", errors.New("gagal memuat penerima PDF final")
	}

	verifyURL := ""
	if data.QRToken != nil && *data.QRToken != "" {
		verifyURL = h.verifyURL(*data.QRToken)
	}
	pdfBytes, err := renderFinalLetterPDF(data, recipients, verifyURL)
	if err != nil {
		return "", errors.New("gagal membuat PDF final")
	}

	objectName := fmt.Sprintf("letters/%s/final/final-%d.pdf", letterID, publishedAt.Unix())
	if _, err := h.Minio.PutObject(ctx, h.Bucket, objectName, bytes.NewReader(pdfBytes), int64(len(pdfBytes)), minio.PutObjectOptions{
		ContentType: "application/pdf",
	}); err != nil {
		return "", errors.New("gagal menyimpan PDF final ke object storage")
	}
	return objectName, nil
}

func renderLetterPreviewPDF(data draftPreviewData, recipients map[string][]string) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle(data.Subject, true)
	pdf.SetAuthor("eOffice Pro", true)
	pdf.SetMargins(20, 18, 20)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(0, 8, data.CompanyName, "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(0, 5, fmt.Sprintf("%s - %s", data.LetterTypeCode, data.LetterTypeName), "", 1, "C", false, 0, "")
	pdf.Ln(3)
	pdf.Line(20, pdf.GetY(), 190, pdf.GetY())
	pdf.Ln(6)

	pdf.SetFont("Arial", "", 10)
	writePDFKV(pdf, "Perihal", data.Subject)
	writePDFKV(pdf, "Klasifikasi", strings.Title(data.Classification))
	writePDFKV(pdf, "Prioritas", strings.Title(data.Priority))
	writePDFKV(pdf, "Pembuat", data.CreatorName+" - "+data.CreatorPositionTitle)
	writePDFKV(pdf, "Kepada", strings.Join(recipients["to"], "; "))
	if len(recipients["cc"]) > 0 {
		writePDFKV(pdf, "Tembusan", strings.Join(recipients["cc"], "; "))
	}
	pdf.Ln(3)

	pdf.SetFont("Arial", "", 11)
	body := strings.TrimSpace(data.BodyPlain)
	if body == "" {
		body = "(isi surat kosong)"
	}
	pdf.MultiCell(0, 6, body, "", "L", false)
	pdf.Ln(8)

	pdf.SetFont("Arial", "I", 8)
	if data.QRToken != nil && *data.QRToken != "" {
		pdf.MultiCell(0, 5, "Token QR verifikasi: "+*data.QRToken, "", "L", false)
	} else {
		pdf.MultiCell(0, 5, "QR verifikasi akan diterbitkan saat draft diajukan.", "", "L", false)
	}
	pdf.MultiCell(0, 5, fmt.Sprintf("Preview v%d dibuat oleh eOffice Pro pada %s", data.Version, time.Now().Format(time.RFC3339)), "", "L", false)

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func renderFinalLetterPDF(data draftPreviewData, recipients map[string][]string, verifyURL string) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle(data.Subject, true)
	pdf.SetAuthor("eOffice Pro", true)
	pdf.SetMargins(20, 18, 20)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(0, 8, data.CompanyName, "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(0, 5, fmt.Sprintf("%s - %s", data.LetterTypeCode, data.LetterTypeName), "", 1, "C", false, 0, "")
	pdf.Ln(3)
	pdf.Line(20, pdf.GetY(), 190, pdf.GetY())
	pdf.Ln(6)

	publishedAt := "-"
	if data.PublishedAt != nil {
		publishedAt = data.PublishedAt.Format("02/01/2006 15:04 MST")
	}

	pdf.SetFont("Arial", "", 10)
	writePDFKV(pdf, "Nomor", data.LetterNumber)
	writePDFKV(pdf, "Tanggal", publishedAt)
	writePDFKV(pdf, "Perihal", data.Subject)
	writePDFKV(pdf, "Klasifikasi", strings.Title(data.Classification))
	writePDFKV(pdf, "Prioritas", strings.Title(data.Priority))
	writePDFKV(pdf, "Pembuat", data.CreatorName+" - "+data.CreatorPositionTitle)
	writePDFKV(pdf, "Kepada", strings.Join(recipients["to"], "; "))
	if len(recipients["cc"]) > 0 {
		writePDFKV(pdf, "Tembusan", strings.Join(recipients["cc"], "; "))
	}
	pdf.Ln(4)

	pdf.SetFont("Arial", "", 11)
	body := strings.TrimSpace(data.BodyPlain)
	if body == "" {
		body = "(isi surat kosong)"
	}
	pdf.MultiCell(0, 6, body, "", "L", false)
	pdf.Ln(10)

	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, data.CreatorPositionTitle+",", "", 1, "R", false, 0, "")
	pdf.Ln(14)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 6, data.CreatorName, "", 1, "R", false, 0, "")

	pdf.Ln(8)
	pdf.SetFont("Arial", "I", 8)
	if verifyURL != "" {
		pdf.MultiCell(0, 5, "Verifikasi dokumen: "+verifyURL, "", "L", false)
	}
	if data.QRToken != nil && *data.QRToken != "" {
		pdf.MultiCell(0, 5, "Token QR verifikasi: "+*data.QRToken, "", "L", false)
	}
	pdf.MultiCell(0, 5, fmt.Sprintf("PDF final v%d diterbitkan oleh eOffice Pro.", data.Version), "", "L", false)

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writePDFKV(pdf *gofpdf.Fpdf, key string, value string) {
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(32, 6, key, "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 6, ": "+value, "", "L", false)
}

func (h *Handler) presignedGetURL(ctx context.Context, objectName string) (string, error) {
	reqParams := make(url.Values)
	u, err := h.Minio.PresignedGetObject(ctx, h.Bucket, objectName, presignedURLTTL, reqParams)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (h *Handler) verifyURL(token string) string {
	base := strings.TrimRight(h.Cfg.WebBaseURL, "/")
	return base + "/verify/" + token
}

func normalizeMIMEType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "application/octet-stream"
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return strings.ToLower(value)
	}
	return strings.ToLower(mediaType)
}

func safeObjectFileName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == "" {
		return "lampiran"
	}
	unsafe := regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	name = unsafe.ReplaceAllString(name, "-")
	name = strings.Trim(name, ".-")
	if name == "" {
		return "lampiran"
	}
	if len(name) > 120 {
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		if len(ext) > 20 {
			ext = ""
		}
		if len(base) > 120-len(ext) {
			base = base[:120-len(ext)]
		}
		name = base + ext
	}
	return name
}

func randomHex(bytesLen int) string {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
