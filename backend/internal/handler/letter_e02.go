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
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jung-kurt/gofpdf"
	"github.com/minio/minio-go/v7"
	"golang.org/x/net/html"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

const (
	maxDraftAttachmentSize              = 25 * 1024 * 1024
	presignedURLTTL                     = 15 * time.Minute
	verificationQRCodeImageSize         = 512
	verificationQRCodeQuietZoneModules  = 4
	verificationQRCodePDFSizeMillimeter = 28.0
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
	CompanyLogoKey       string
	CompanyLogoMIMEType  string
	LetterTypeCode       string
	LetterTypeName       string
	LetterNumber         string
	Subject              string
	Classification       string
	Priority             string
	CreatorName          string
	CreatorPositionTitle string
	Version              int
	TemplateSnapshot     []byte
	BodyHTML             string
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
	contentType, err := validateDraftAttachmentContent(data, fileHeader.Filename, fileHeader.Header.Get("Content-Type"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sum := sha256.Sum256(data)
	checksum := hex.EncodeToString(sum[:])
	objectName := fmt.Sprintf("quarantine/letters/%s/attachments/%s-%s", letterID, randomHex(8), safeObjectFileName(fileHeader.Filename))

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
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending')
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
	if _, err := h.DB.Exec(ctx, `INSERT INTO attachment_scan_jobs (attachment_id) VALUES ($1)`, attachmentID); err != nil {
		_ = h.Minio.RemoveObject(ctx, h.Bucket, objectName, minio.RemoveObjectOptions{})
		_, _ = h.DB.Exec(ctx, `DELETE FROM letter_attachments WHERE id = $1`, attachmentID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membuat antrean pemindaian lampiran"})
		return
	}

	h.audit(ctx, "letter", &letterID, "upload_attachment", &userID, map[string]any{
		"attachment_id": attachmentID,
		"file_name":     fileHeader.Filename,
		"size_bytes":    len(data),
		"scan_status":   "pending",
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

	companyLogo, err := h.loadCompanyLogo(ctx, data.CompanyLogoKey, data.CompanyLogoMIMEType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat logo perusahaan"})
		return
	}

	pdfBytes, err := renderLetterPreviewPDF(data, recipients, companyLogo)
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
	var electronicSubmissionEnabled bool
	if err := tx.QueryRow(ctx, `
		SELECT electronic_submission_enabled
		FROM letter_types
		WHERE id = $1 AND is_active`, draft.LetterTypeID).Scan(&electronicSubmissionEnabled); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jenis surat tidak aktif atau tidak ditemukan"})
		return
	}
	if !electronicSubmissionEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "jenis surat ini belum diizinkan untuk pengajuan elektronik"})
		return
	}
	var unsafeAttachmentCount int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM letter_attachments
		WHERE letter_id = $1 AND scan_status <> 'clean'`, letterID).Scan(&unsafeAttachmentCount); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa status lampiran"})
		return
	}
	if unsafeAttachmentCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "semua lampiran harus berstatus clean sebelum surat diajukan"})
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

	policyPreview, err := h.resolvePolicyRoute(ctx, tx, draft.LetterTypeID, draft.CreatorPositionID,
		draft.ApprovalCategoryID, draft.RequestedFinalLevel, recipients)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	route := approvalRoute{SLAHours: 24, Steps: policyPreview.Steps}
	if err := validateApprovalRouteHasActiveHolders(ctx, tx, route); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	approvalCycle, err := insertApprovalRoute(ctx, tx, letterID, route)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan rute approval"})
		return
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
		    resolved_final_level = $4,
		    coordination_scope = NULLIF($5, ''),
		    approval_resolution_mode = $6,
		    updated_at = now()
		WHERE id = $1`, letterID, routeJSON, qrToken, policyPreview.FinalLevel,
		policyPreview.Scope, policyPreview.ResolutionMode); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengajukan surat"})
		return
	}
	if err := enqueueNotificationOutbox(ctx, tx, approverEmails); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengantrikan notifikasi approval"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan pengajuan"})
		return
	}

	h.audit(ctx, "letter", &letterID, "submit", &userID, map[string]any{
		"approval_steps": len(route.Steps),
		"approval_cycle": approvalCycle,
		"qr_token":       qrToken,
	}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{
		"id":             letterID,
		"status":         "in_approval",
		"approval_cycle": approvalCycle,
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
		// Downloads are served through authenticated handlers so a presigned URL
		// cannot be forwarded to an unauthorized recipient.
		_ = includeURL
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
		SELECT l.id::text, co.code, co.name,
		       COALESCE(co.letterhead_config #>> '{logo,storage_key}', ''),
		       COALESCE(co.letterhead_config #>> '{logo,mime_type}', ''),
		       lt.code, lt.name,
		       l.subject, l.classification, l.priority, u.full_name,
		       p.title, COALESCE(v.version, 0), l.template_snapshot, COALESCE(v.body_html, ''), COALESCE(v.body_plain, ''),
		       l.qr_token, l.created_at, l.updated_at
		FROM letters l
		JOIN companies co ON co.id = l.company_id
		JOIN letter_types lt ON lt.id = l.letter_type_id
		JOIN users u ON u.id = l.creator_user_id
		JOIN positions p ON p.id = l.creator_position_id
		LEFT JOIN LATERAL (
			SELECT version, body_html, body_plain
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
		&data.CompanyLogoKey,
		&data.CompanyLogoMIMEType,
		&data.LetterTypeCode,
		&data.LetterTypeName,
		&data.Subject,
		&data.Classification,
		&data.Priority,
		&data.CreatorName,
		&data.CreatorPositionTitle,
		&data.Version,
		&data.TemplateSnapshot,
		&data.BodyHTML,
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
		SELECT l.id::text, co.code, co.name,
		       COALESCE(co.letterhead_config #>> '{logo,storage_key}', ''),
		       COALESCE(co.letterhead_config #>> '{logo,mime_type}', ''),
		       lt.code, lt.name,
		       l.letter_number, l.subject, l.classification, l.priority,
		       u.full_name, p.title, COALESCE(v.version, 0), l.template_snapshot, COALESCE(v.body_html, ''), COALESCE(v.body_plain, ''),
		       l.qr_token, l.published_at, l.created_at, l.updated_at
		FROM letters l
		JOIN companies co ON co.id = l.company_id
		JOIN letter_types lt ON lt.id = l.letter_type_id
		JOIN users u ON u.id = l.creator_user_id
		JOIN positions p ON p.id = l.creator_position_id
		LEFT JOIN LATERAL (
			SELECT version, body_html, body_plain
			FROM letter_versions
			WHERE letter_id = l.id
			ORDER BY version DESC
			LIMIT 1
		) v ON true
		WHERE l.id = $1`, letterID).Scan(
		&data.ID,
		&data.CompanyCode,
		&data.CompanyName,
		&data.CompanyLogoKey,
		&data.CompanyLogoMIMEType,
		&data.LetterTypeCode,
		&data.LetterTypeName,
		&letterNumber,
		&data.Subject,
		&data.Classification,
		&data.Priority,
		&data.CreatorName,
		&data.CreatorPositionTitle,
		&data.Version,
		&data.TemplateSnapshot,
		&data.BodyHTML,
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
		return "", fmt.Errorf("gagal memuat data PDF final: %w", err)
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
	companyLogo, err := h.loadCompanyLogo(ctx, data.CompanyLogoKey, data.CompanyLogoMIMEType)
	if err != nil {
		return "", errors.New("gagal memuat logo perusahaan untuk PDF final")
	}
	signatures, err := h.loadApprovalPDFSignatures(ctx, tx, letterID)
	if err != nil {
		return "", errors.New("gagal memuat tanda tangan approval untuk PDF final")
	}
	pdfBytes, err := renderFinalLetterPDF(data, recipients, verifyURL, companyLogo, signatures)
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

// RegenerateFinalPDF replaces the stored final PDF for one published letter
// without changing its number, publication timestamp, or verification token.
func (h *Handler) RegenerateFinalPDF(ctx context.Context, letterID string) (string, error) {
	letterID = strings.TrimSpace(letterID)
	if letterID == "" {
		return "", errors.New("letter-id wajib diisi")
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("memulai regenerasi PDF final: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string
	var letterNumber *string
	var finalPDFKey *string
	var publishedAt *time.Time
	if err := tx.QueryRow(ctx, `
		SELECT status, letter_number, final_pdf_key, published_at
		FROM letters
		WHERE id = $1
		FOR UPDATE`, letterID).Scan(&status, &letterNumber, &finalPDFKey, &publishedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.New("surat tidak ditemukan")
		}
		return "", fmt.Errorf("memuat surat untuk regenerasi PDF final: %w", err)
	}
	if status != "published" {
		return "", errors.New("PDF final hanya dapat diregenerasi untuk surat published")
	}
	if letterNumber == nil || strings.TrimSpace(*letterNumber) == "" {
		return "", errors.New("surat published belum memiliki nomor")
	}
	if publishedAt == nil {
		return "", errors.New("surat published belum memiliki tanggal terbit")
	}
	if finalPDFKey == nil || strings.TrimSpace(*finalPDFKey) == "" {
		return "", errors.New("surat published belum memiliki objek PDF final")
	}

	objectName, err := h.renderAndStoreFinalPDF(ctx, tx, letterID, *letterNumber, *publishedAt)
	if err != nil {
		return "", err
	}
	if objectName != *finalPDFKey {
		return "", fmt.Errorf("key PDF final hasil regenerasi %q berbeda dari key tersimpan %q", objectName, *finalPDFKey)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("menyelesaikan regenerasi PDF final: %w", err)
	}

	h.audit(ctx, "letter", &letterID, "regenerate_final_pdf", nil, map[string]any{
		"final_pdf_key": objectName,
	}, "")
	return objectName, nil
}

func (h *Handler) loadCompanyLogo(ctx context.Context, storageKey string, expectedMIMEType string) ([]byte, error) {
	storageKey = strings.TrimSpace(storageKey)
	if storageKey == "" {
		return nil, nil
	}
	if h.Minio == nil {
		return nil, errors.New("object storage belum tersedia")
	}

	object, err := h.Minio.GetObject(ctx, h.Bucket, storageKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer object.Close()

	info, err := object.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size <= 0 || info.Size > maxCompanyLogoSize {
		return nil, errors.New("ukuran objek logo perusahaan tidak valid")
	}
	data, err := io.ReadAll(io.LimitReader(object, maxCompanyLogoSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxCompanyLogoSize {
		return nil, errors.New("ukuran objek logo perusahaan melebihi batas")
	}

	logo, _, err := validateCompanyLogo(data)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(expectedMIMEType) != "" {
		if expected := normalizeMIMEType(expectedMIMEType); expected != logo.MIMEType {
			return nil, errors.New("tipe objek logo perusahaan tidak sesuai konfigurasi")
		}
	}
	return data, nil
}

func renderLetterPreviewPDF(data draftPreviewData, recipients map[string][]string, companyLogo []byte) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle(data.Subject, true)
	pdf.SetAuthor("eOffice Pro", true)
	configureLetterPDFLayout(pdf, data.TemplateSnapshot)
	pdf.AddPage()

	if err := writeLetterHeader(pdf, data, companyLogo); err != nil {
		return nil, err
	}

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

	if err := writeLetterHTML(pdf, data.BodyHTML, data.BodyPlain); err != nil {
		return nil, err
	}
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

func renderFinalLetterPDF(data draftPreviewData, recipients map[string][]string, verifyURL string, companyLogo []byte, signatures []approvalPDFSignature) ([]byte, error) {
	verifyURL = strings.TrimSpace(verifyURL)
	if verifyURL == "" {
		return nil, errors.New("URL verifikasi wajib tersedia untuk PDF final")
	}
	if data.QRToken == nil || strings.TrimSpace(*data.QRToken) == "" {
		return nil, errors.New("token QR verifikasi wajib tersedia untuk PDF final")
	}

	qrCode, err := generateVerificationQRCode(verifyURL, verificationQRCodeImageSize)
	if err != nil {
		return nil, fmt.Errorf("membuat QR verifikasi: %w", err)
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle(data.Subject, true)
	pdf.SetAuthor("eOffice Pro", true)
	configureLetterPDFLayout(pdf, data.TemplateSnapshot)
	pdf.AddPage()

	if err := writeLetterHeader(pdf, data, companyLogo); err != nil {
		return nil, err
	}

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

	if err := writeLetterHTML(pdf, data.BodyHTML, data.BodyPlain); err != nil {
		return nil, err
	}
	pdf.Ln(10)

	if err := writeApprovalSignaturesBlock(pdf, signatures); err != nil {
		return nil, err
	}
	pdf.Ln(8)
	if err := writeVerificationQRCodeBlock(pdf, qrCode.PNG, verifyURL, data.Version); err != nil {
		return nil, err
	}

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// writeLetterHTML renders only the already-sanitized editor subset. It does
// not interpret CSS or arbitrary HTML, keeping PDF rendering deterministic.
func writeLetterHTML(pdf *gofpdf.Fpdf, bodyHTML, fallback string) error {
	bodyHTML = strings.TrimSpace(bodyHTML)
	if bodyHTML == "" {
		bodyHTML = "<p>" + strings.TrimSpace(fallback) + "</p>"
	}
	document, err := html.Parse(strings.NewReader(bodyHTML))
	if err != nil {
		return fmt.Errorf("membaca isi surat: %w", err)
	}
	body := findHTMLElement(document, "body")
	if body == nil {
		return errors.New("isi surat tidak dapat dirender")
	}
	if strings.TrimSpace(nodePlainText(body)) == "" {
		pdf.SetFont("Arial", "", 11)
		pdf.MultiCell(0, 6, "(isi surat kosong)", "", "L", false)
		return nil
	}
	for child := body.FirstChild; child != nil; child = child.NextSibling {
		if err := writeLetterHTMLNode(pdf, child); err != nil {
			return err
		}
	}
	return nil
}

func configureLetterPDFLayout(pdf *gofpdf.Fpdf, snapshot []byte) {
	const defaultMargin = 20.0
	top, right, bottom, left := 18.0, defaultMargin, 18.0, defaultMargin
	var config struct {
		Layout struct {
			Page struct {
				MarginMM struct {
					Top    float64 `json:"top"`
					Right  float64 `json:"right"`
					Bottom float64 `json:"bottom"`
					Left   float64 `json:"left"`
				} `json:"margin_mm"`
			} `json:"page"`
		} `json:"layout_config"`
	}
	if len(snapshot) > 0 && json.Unmarshal(snapshot, &config) == nil {
		margin := config.Layout.Page.MarginMM
		if margin.Top >= 8 && margin.Top <= 45 {
			top = margin.Top
		}
		if margin.Right >= 8 && margin.Right <= 45 {
			right = margin.Right
		}
		if margin.Bottom >= 8 && margin.Bottom <= 45 {
			bottom = margin.Bottom
		}
		if margin.Left >= 8 && margin.Left <= 45 {
			left = margin.Left
		}
	}
	pdf.SetMargins(left, top, right)
	pdf.SetAutoPageBreak(true, bottom)
}

func writeLetterHTMLNode(pdf *gofpdf.Fpdf, node *html.Node) error {
	if node.Type == html.TextNode {
		text := strings.TrimSpace(node.Data)
		if text != "" {
			pdf.SetFont("Arial", "", 11)
			pdf.MultiCell(0, 6, text, "", "L", false)
		}
		return nil
	}
	if node.Type != html.ElementNode {
		return nil
	}

	name := strings.ToLower(node.Data)
	switch name {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		size := 13.0
		if name == "h1" {
			size = 16
		} else if name == "h2" {
			size = 14
		}
		pdf.SetFont("Arial", "B", size)
		pdf.MultiCell(0, 7, nodePlainText(node), "", htmlAlignment(node), false)
		pdf.Ln(1)
		return nil
	case "p", "blockquote", "pre":
		style := ""
		if name == "blockquote" {
			style = "I"
		}
		pdf.SetFont("Arial", style, 11)
		pdf.MultiCell(0, 6, nodePlainText(node), "", htmlAlignment(node), false)
		pdf.Ln(1)
		return nil
	case "ul", "ol":
		index := 1
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type != html.ElementNode || strings.ToLower(child.Data) != "li" {
				continue
			}
			prefix := "• "
			if name == "ol" {
				prefix = fmt.Sprintf("%d. ", index)
				index++
			}
			pdf.SetFont("Arial", "", 11)
			pdf.MultiCell(0, 6, prefix+nodePlainText(child), "", "L", false)
		}
		pdf.Ln(1)
		return nil
	case "table":
		for row := node.FirstChild; row != nil; row = row.NextSibling {
			if err := writeLetterTableRows(pdf, row); err != nil {
				return err
			}
		}
		pdf.Ln(1)
		return nil
	case "br":
		pdf.Ln(6)
		return nil
	case "strong", "em", "u", "s", "code", "li", "th", "td", "tr", "thead", "tbody", "tfoot":
		// These elements are rendered by their parent block/table.
		return nil
	default:
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if err := writeLetterHTMLNode(pdf, child); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeLetterTableRows(pdf *gofpdf.Fpdf, node *html.Node) error {
	if node.Type == html.ElementNode && strings.ToLower(node.Data) == "tr" {
		cells := []string{}
		for cell := node.FirstChild; cell != nil; cell = cell.NextSibling {
			if cell.Type == html.ElementNode && (cell.Data == "td" || cell.Data == "th") {
				cells = append(cells, nodePlainText(cell))
			}
		}
		if len(cells) > 0 {
			pdf.SetFont("Arial", "", 10)
			pdf.MultiCell(0, 5, strings.Join(cells, " | "), "1", "L", false)
		}
		return nil
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if err := writeLetterTableRows(pdf, child); err != nil {
			return err
		}
	}
	return nil
}

func findHTMLElement(node *html.Node, name string) *html.Node {
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, name) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findHTMLElement(child, name); found != nil {
			return found
		}
	}
	return nil
}

func nodePlainText(node *html.Node) string {
	parts := []string{}
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			if text := strings.TrimSpace(current.Data); text != "" {
				parts = append(parts, text)
			}
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return strings.Join(parts, " ")
}

func htmlAlignment(node *html.Node) string {
	for _, attribute := range node.Attr {
		if attribute.Key == "style" {
			if strings.Contains(attribute.Val, "text-align:center") {
				return "C"
			}
			if strings.Contains(attribute.Val, "text-align:right") {
				return "R"
			}
			if strings.Contains(attribute.Val, "text-align:justify") {
				return "J"
			}
		}
	}
	return "L"
}

func writeApprovalSignaturesBlock(pdf *gofpdf.Fpdf, signatures []approvalPDFSignature) error {
	if len(signatures) == 0 {
		return nil
	}

	leftMargin, _, rightMargin, bottomMargin := pdf.GetMargins()
	pageWidth, pageHeight := pdf.GetPageSize()
	usableWidth := pageWidth - leftMargin - rightMargin
	const gutter = 8.0
	columnWidth := (usableWidth - gutter) / 2
	const blockHeight = 38.0

	if pdf.GetY()+blockHeight > pageHeight-bottomMargin {
		pdf.AddPage()
	}
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 6, "Tanda tangan approval", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	for i, signature := range signatures {
		if i%2 == 0 && pdf.GetY()+blockHeight > pageHeight-bottomMargin {
			pdf.AddPage()
		}

		rowY := pdf.GetY()
		colX := leftMargin
		if i%2 == 1 {
			colX = leftMargin + columnWidth + gutter
		}

		pdf.SetXY(colX, rowY)
		pdf.SetFont("Arial", "", 8)
		title := strings.TrimSpace(signature.PositionTitle)
		if title == "" {
			title = fmt.Sprintf("Approval step %d", signature.StepOrder)
		}
		pdf.MultiCell(columnWidth, 4, title, "", "C", false)

		imageY := rowY + 7
		if len(signature.Image) > 0 {
			imageName := fmt.Sprintf("approval-signature-%d-%d", signature.StepOrder, i)
			options := gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
			if info := pdf.RegisterImageOptionsReader(imageName, options, bytes.NewReader(signature.Image)); info == nil {
				if err := pdf.Error(); err != nil {
					return fmt.Errorf("mendaftarkan gambar tanda tangan: %w", err)
				}
				return errors.New("gambar tanda tangan tidak dapat didaftarkan")
			}
			drawWidth, drawHeight := fitSignatureImage(signature.Image, columnWidth-8, 16)
			pdf.ImageOptions(imageName, colX+(columnWidth-drawWidth)/2, imageY, drawWidth, drawHeight, false, options, 0, "")
			if err := pdf.Error(); err != nil {
				return fmt.Errorf("menulis gambar tanda tangan: %w", err)
			}
		}

		pdf.SetXY(colX, imageY+18)
		pdf.SetFont("Arial", "B", 8)
		actorName := strings.TrimSpace(signature.ActorName)
		if signature.OnBehalf {
			// Delegasi E03-5: delegate menandatangani "a.n." posisi delegator.
			actorName = "a.n. " + actorName
		}
		pdf.MultiCell(columnWidth, 4, actorName, "", "C", false)
		pdf.SetX(colX)
		pdf.SetFont("Arial", "I", 7)
		pdf.MultiCell(columnWidth, 4, signature.ActedAt.Format("02/01/2006 15:04 MST"), "", "C", false)

		if i%2 == 1 || i == len(signatures)-1 {
			pdf.SetY(rowY + blockHeight)
		}
	}
	return nil
}

func fitSignatureImage(data []byte, maxWidth float64, maxHeight float64) (float64, float64) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return maxWidth, maxHeight
	}
	width := maxWidth
	height := width * float64(config.Height) / float64(config.Width)
	if height > maxHeight {
		height = maxHeight
		width = height * float64(config.Width) / float64(config.Height)
	}
	return width, height
}

func writeLetterHeader(pdf *gofpdf.Fpdf, data draftPreviewData, companyLogo []byte) error {
	const (
		headerHeight  = 16.0
		logoBoxWidth  = 22.0
		logoBoxHeight = 16.0
		textX         = 45.0
		textWidth     = 120.0
	)

	startY := pdf.GetY()
	if len(companyLogo) > 0 {
		imageConfig, format, err := image.DecodeConfig(bytes.NewReader(companyLogo))
		if err != nil {
			return fmt.Errorf("membaca logo perusahaan untuk kop: %w", err)
		}

		imageType := ""
		switch format {
		case "png":
			imageType = "PNG"
		case "jpeg":
			imageType = "JPG"
		default:
			return errors.New("format logo perusahaan pada kop tidak didukung")
		}
		imageOptions := gofpdf.ImageOptions{ImageType: imageType, ReadDpi: true}
		const imageName = "company-letterhead-logo"
		if info := pdf.RegisterImageOptionsReader(imageName, imageOptions, bytes.NewReader(companyLogo)); info == nil {
			if err := pdf.Error(); err != nil {
				return fmt.Errorf("mendaftarkan logo perusahaan ke PDF: %w", err)
			}
			return errors.New("gagal mendaftarkan logo perusahaan ke PDF")
		}

		scale := min(logoBoxWidth/float64(imageConfig.Width), logoBoxHeight/float64(imageConfig.Height))
		drawWidth := float64(imageConfig.Width) * scale
		drawHeight := float64(imageConfig.Height) * scale
		logoX := 20 + (logoBoxWidth-drawWidth)/2
		logoY := startY + (logoBoxHeight-drawHeight)/2
		pdf.ImageOptions(imageName, logoX, logoY, drawWidth, drawHeight, false, imageOptions, 0, "")
		if err := pdf.Error(); err != nil {
			return fmt.Errorf("menempatkan logo perusahaan pada kop: %w", err)
		}

		fontSize := 14.0
		pdf.SetFont("Arial", "B", fontSize)
		for pdf.GetStringWidth(data.CompanyName) > textWidth-4 && fontSize > 10 {
			fontSize--
			pdf.SetFont("Arial", "B", fontSize)
		}
		pdf.SetXY(textX, startY+1)
		pdf.CellFormat(textWidth, 8, data.CompanyName, "", 1, "C", false, 0, "")
		pdf.SetXY(textX, startY+9)
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(textWidth, 5, fmt.Sprintf("%s - %s", data.LetterTypeCode, data.LetterTypeName), "", 1, "C", false, 0, "")
		pdf.SetY(startY + headerHeight)
	} else {
		pdf.SetFont("Arial", "B", 14)
		pdf.CellFormat(0, 8, data.CompanyName, "", 1, "C", false, 0, "")
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(0, 5, fmt.Sprintf("%s - %s", data.LetterTypeCode, data.LetterTypeName), "", 1, "C", false, 0, "")
		pdf.SetY(startY + headerHeight)
	}

	pdf.Line(20, pdf.GetY(), 190, pdf.GetY())
	pdf.Ln(6)
	return nil
}

type generatedVerificationQRCode struct {
	Payload string
	PNG     []byte
}

func generateVerificationQRCode(payload string, targetSize int) (generatedVerificationQRCode, error) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return generatedVerificationQRCode{}, errors.New("payload QR tidak boleh kosong")
	}

	code, err := qr.Encode(payload, qr.M, qr.Auto)
	if err != nil {
		return generatedVerificationQRCode{}, err
	}
	moduleCount := code.Bounds().Dx()
	modulePixels := targetSize / (moduleCount + 2*verificationQRCodeQuietZoneModules)
	if modulePixels < 1 {
		return generatedVerificationQRCode{}, errors.New("ukuran gambar QR terlalu kecil")
	}

	innerSize := moduleCount * modulePixels
	scaled, err := barcode.Scale(code, innerSize, innerSize)
	if err != nil {
		return generatedVerificationQRCode{}, err
	}

	quietZonePixels := verificationQRCodeQuietZoneModules * modulePixels
	canvasSize := innerSize + 2*quietZonePixels
	canvas := image.NewGray(image.Rect(0, 0, canvasSize, canvasSize))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(
		canvas,
		image.Rect(quietZonePixels, quietZonePixels, quietZonePixels+innerSize, quietZonePixels+innerSize),
		scaled,
		scaled.Bounds().Min,
		draw.Src,
	)

	var out bytes.Buffer
	if err := png.Encode(&out, canvas); err != nil {
		return generatedVerificationQRCode{}, err
	}
	return generatedVerificationQRCode{
		Payload: payload,
		PNG:     out.Bytes(),
	}, nil
}

func writeVerificationQRCodeBlock(pdf *gofpdf.Fpdf, qrPNG []byte, verifyURL string, version int) error {
	const blockHeight = verificationQRCodePDFSizeMillimeter + 12

	leftMargin, _, _, bottomMargin := pdf.GetMargins()
	_, pageHeight := pdf.GetPageSize()
	if pdf.GetY()+blockHeight > pageHeight-bottomMargin {
		pdf.AddPage()
	}

	imageOptions := gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
	const imageName = "verification-qr"
	if info := pdf.RegisterImageOptionsReader(imageName, imageOptions, bytes.NewReader(qrPNG)); info == nil {
		if err := pdf.Error(); err != nil {
			return fmt.Errorf("mendaftarkan gambar QR ke PDF: %w", err)
		}
		return errors.New("gagal mendaftarkan gambar QR ke PDF")
	}

	startY := pdf.GetY()
	pdf.ImageOptions(
		imageName,
		leftMargin,
		startY,
		verificationQRCodePDFSizeMillimeter,
		verificationQRCodePDFSizeMillimeter,
		false,
		imageOptions,
		0,
		verifyURL,
	)
	if err := pdf.Error(); err != nil {
		return fmt.Errorf("menempatkan gambar QR ke PDF: %w", err)
	}

	pdf.SetXY(leftMargin, startY+verificationQRCodePDFSizeMillimeter+1)
	pdf.SetFont("Arial", "B", 8)
	pdf.CellFormat(65, 4, "Pindai untuk verifikasi dokumen", "", 1, "L", false, 0, "")
	pdf.SetX(leftMargin)
	pdf.SetFont("Arial", "I", 8)
	pdf.CellFormat(80, 5, fmt.Sprintf("PDF final v%d diterbitkan oleh eOffice Pro.", version), "", 1, "L", false, 0, "")
	return nil
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
