package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

func (h *Handler) DownloadLetterAttachment(c *gin.Context) {
	letterID := c.Param("id")
	attachmentID := c.Param("attachment_id")
	userID := c.GetString(middleware.CtxUserID)
	ctx := c.Request.Context()
	if h.Minio == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "object storage belum tersedia"})
		return
	}
	if ok, err := h.userCanDownloadLetter(ctx, userID, letterID); err != nil || !ok {
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa akses unduh"})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "lampiran tidak ditemukan"})
		}
		return
	}
	var storageKey, fileName, mimeType, scanStatus string
	err := h.DB.QueryRow(ctx, `
		SELECT storage_key, file_name, mime_type, scan_status
		FROM letter_attachments
		WHERE id = $1 AND letter_id = $2`, attachmentID, letterID).Scan(&storageKey, &fileName, &mimeType, &scanStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "lampiran tidak ditemukan"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca lampiran"})
		return
	}
	if scanStatus != "clean" {
		c.JSON(http.StatusConflict, gin.H{"error": "lampiran belum tersedia karena belum lolos pemindaian keamanan"})
		return
	}
	h.streamObject(c, storageKey, fileName, mimeType)
	h.audit(ctx, "letter", &letterID, "download_attachment", &userID, map[string]any{"attachment_id": attachmentID}, c.ClientIP())
}

func (h *Handler) DownloadFinalLetterPDF(c *gin.Context) {
	letterID := c.Param("id")
	userID := c.GetString(middleware.CtxUserID)
	ctx := c.Request.Context()
	if h.Minio == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "object storage belum tersedia"})
		return
	}
	if ok, err := h.userCanDownloadLetter(ctx, userID, letterID); err != nil || !ok {
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa akses unduh"})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "PDF final tidak ditemukan"})
		}
		return
	}
	var key, number string
	err := h.DB.QueryRow(ctx, `
		SELECT COALESCE(final_pdf_key, ''), COALESCE(letter_number, '')
		FROM letters WHERE id = $1 AND status = 'published'`, letterID).Scan(&key, &number)
	if errors.Is(err, pgx.ErrNoRows) || key == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "PDF final belum tersedia"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca PDF final"})
		return
	}
	fileName := "surat-" + safeObjectFileName(number)
	if !strings.HasSuffix(strings.ToLower(fileName), ".pdf") {
		fileName += ".pdf"
	}
	h.streamObject(c, key, fileName, "application/pdf")
	h.audit(ctx, "letter", &letterID, "download_final_pdf", &userID, nil, c.ClientIP())
}

func (h *Handler) userCanDownloadLetter(ctx context.Context, userID, letterID string) (bool, error) {
	var allowed bool
	err := h.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM letters l
			WHERE l.id = $1 AND (
				l.classification <> 'rahasia' AND (`+downloadBaseAccessSQL("$2")+`) OR
				l.classification = 'rahasia' AND (
					l.creator_user_id = $2 OR EXISTS (
						SELECT 1 FROM approval_steps s
						JOIN user_positions up ON up.position_id = s.approver_position_id
						WHERE s.letter_id = l.id AND up.user_id = $2
						AND current_date >= up.valid_from AND (up.valid_to IS NULL OR current_date < up.valid_to)
					) OR EXISTS (
						SELECT 1 FROM letter_recipients lr
						WHERE lr.letter_id = l.id AND lr.recipient_type = 'to'
						AND `+publishedRecipientAccessSQL("$2")+`
					)
				)
			)
		)`, letterID, userID).Scan(&allowed)
	return allowed, err
}

func downloadBaseAccessSQL(userArg string) string {
	return `l.creator_user_id = ` + userArg + ` OR EXISTS (
		SELECT 1 FROM approval_steps s JOIN user_positions up ON up.position_id = s.approver_position_id
		WHERE s.letter_id = l.id AND up.user_id = ` + userArg + `
		AND current_date >= up.valid_from AND (up.valid_to IS NULL OR current_date < up.valid_to)
	) OR EXISTS (
		SELECT 1 FROM letter_recipients lr
		WHERE lr.letter_id = l.id AND ` + publishedRecipientAccessSQL(userArg) + `
	) OR ` + auditLetterAccessSQL(userArg, "l")
}

func (h *Handler) streamObject(c *gin.Context, key, fileName, contentType string) {
	object, err := h.Minio.GetObject(c.Request.Context(), h.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "gagal membuka object storage"})
		return
	}
	defer object.Close()
	info, err := object.Stat()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file tidak ditemukan di object storage"})
		return
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", mimeDisposition(fileName))
	if info.Size >= 0 {
		c.Header("Content-Length", strconv.FormatInt(info.Size, 10))
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, object)
}

func mimeDisposition(fileName string) string {
	return fmt.Sprintf("attachment; filename=%q", strings.ReplaceAll(fileName, "\"", ""))
}
