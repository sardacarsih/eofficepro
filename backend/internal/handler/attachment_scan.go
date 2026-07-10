package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
)

const attachmentScanInterval = 10 * time.Second

// validateDraftAttachmentContent rejects files whose bytes do not match their
// declared extension. Browser supplied MIME types are not a security boundary.
func validateDraftAttachmentContent(data []byte, fileName, declaredMIME string) (string, error) {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(fileName)))
	declared := normalizeMIMEType(declaredMIME)
	if len(data) == 0 {
		return "", errors.New("lampiran kosong")
	}
	if declared != "" && !allowedAttachmentMIMETypes[declared] {
		return "", errors.New("tipe file lampiran tidak diizinkan")
	}

	switch ext {
	case ".pdf":
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			return "", errors.New("isi lampiran bukan PDF yang valid")
		}
		return "application/pdf", nil
	case ".png":
		if !bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) {
			return "", errors.New("isi lampiran bukan PNG yang valid")
		}
		return "image/png", nil
	case ".jpg", ".jpeg":
		if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 || data[len(data)-2] != 0xff || data[len(data)-1] != 0xd9 {
			return "", errors.New("isi lampiran bukan JPEG yang valid")
		}
		return "image/jpeg", nil
	case ".docx", ".xlsx":
		kind, err := officeDocumentKind(data)
		if err != nil {
			return "", err
		}
		if ext == ".docx" && kind != "docx" || ext == ".xlsx" && kind != "xlsx" {
			return "", errors.New("ekstensi lampiran tidak sesuai dengan dokumen Office")
		}
		if kind == "docx" {
			return "application/vnd.openxmlformats-officedocument.wordprocessingml.document", nil
		}
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
	case ".xls":
		if len(data) < 8 || !bytes.Equal(data[:8], []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}) {
			return "", errors.New("isi lampiran bukan XLS yang valid")
		}
		return "application/vnd.ms-excel", nil
	case ".csv":
		if bytes.IndexByte(data, 0) >= 0 {
			return "", errors.New("CSV tidak boleh memuat byte biner")
		}
		return "text/csv", nil
	default:
		return "", errors.New("ekstensi lampiran tidak diizinkan")
	}
}

func officeDocumentKind(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", errors.New("isi lampiran bukan dokumen Office yang valid")
	}
	var hasContentTypes, hasWord, hasXL bool
	for _, file := range reader.File {
		if file.Name == "[Content_Types].xml" {
			hasContentTypes = true
		}
		hasWord = hasWord || strings.HasPrefix(file.Name, "word/")
		hasXL = hasXL || strings.HasPrefix(file.Name, "xl/")
	}
	if !hasContentTypes || hasWord == hasXL {
		return "", errors.New("struktur dokumen Office tidak valid")
	}
	if hasWord {
		return "docx", nil
	}
	return "xlsx", nil
}

// RunAttachmentScanWorker processes quarantine objects. It leaves an object
// pending when ClamAV is unavailable, preventing accidental distribution.
func (h *Handler) RunAttachmentScanWorker(ctx context.Context) {
	ticker := time.NewTicker(attachmentScanInterval)
	defer ticker.Stop()
	for {
		if err := h.scanNextAttachment(ctx); err != nil && ctx.Err() == nil {
			// A retryable failure is persisted by scanNextAttachment.
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (h *Handler) scanNextAttachment(ctx context.Context) error {
	if h.Minio == nil {
		return nil
	}
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var jobID, attachmentID, storageKey string
	err = tx.QueryRow(ctx, `
		SELECT j.id::text, a.id::text, a.storage_key
		FROM attachment_scan_jobs j
		JOIN letter_attachments a ON a.id = j.attachment_id
		WHERE j.status IN ('pending', 'failed')
		ORDER BY j.created_at
		FOR UPDATE SKIP LOCKED
		LIMIT 1`).Scan(&jobID, &attachmentID, &storageKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE attachment_scan_jobs SET status = 'processing', attempts = attempts + 1, updated_at = now() WHERE id = $1`, jobID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	object, err := h.Minio.GetObject(ctx, h.Bucket, storageKey, minio.GetObjectOptions{})
	if err != nil {
		return h.failAttachmentScan(ctx, jobID, attachmentID, fmt.Errorf("membaca objek karantina: %w", err))
	}
	defer object.Close()
	result, err := scanClamAV(ctx, h.Cfg.ClamAVAddress, object)
	if err != nil {
		return h.failAttachmentScan(ctx, jobID, attachmentID, err)
	}
	if result != "clean" {
		_ = h.Minio.RemoveObject(ctx, h.Bucket, storageKey, minio.RemoveObjectOptions{})
		_, err := h.DB.Exec(ctx, `
			WITH affected AS (
				UPDATE letter_attachments SET scan_status = 'infected' WHERE id = $1
				RETURNING letter_id
			), notified AS (
				INSERT INTO notifications (user_id, event_type, letter_id, title, body)
				SELECT l.creator_user_id, 'attachment_infected', l.id,
				       'Lampiran ditolak', 'Lampiran terdeteksi berbahaya dan telah dihapus.'
				FROM affected a JOIN letters l ON l.id = a.letter_id
			)
			UPDATE attachment_scan_jobs
			SET status = 'completed', last_error = 'malware detected', updated_at = now()
			WHERE id = $2`, attachmentID, jobID)
		return err
	}

	cleanKey := strings.TrimPrefix(storageKey, "quarantine/")
	_, err = h.Minio.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: h.Bucket, Object: cleanKey},
		minio.CopySrcOptions{Bucket: h.Bucket, Object: storageKey})
	if err != nil {
		return h.failAttachmentScan(ctx, jobID, attachmentID, fmt.Errorf("memindahkan lampiran bersih: %w", err))
	}
	if err := h.Minio.RemoveObject(ctx, h.Bucket, storageKey, minio.RemoveObjectOptions{}); err != nil {
		return h.failAttachmentScan(ctx, jobID, attachmentID, fmt.Errorf("menghapus karantina: %w", err))
	}
	_, err = h.DB.Exec(ctx, `
		UPDATE letter_attachments SET scan_status = 'clean', storage_key = $1 WHERE id = $2;
		UPDATE attachment_scan_jobs SET status = 'completed', last_error = NULL, updated_at = now() WHERE id = $3`, cleanKey, attachmentID, jobID)
	return err
}

func (h *Handler) failAttachmentScan(ctx context.Context, jobID, attachmentID string, scanErr error) error {
	_, err := h.DB.Exec(ctx, `
		UPDATE letter_attachments SET scan_status = 'failed' WHERE id = $1;
		UPDATE attachment_scan_jobs SET status = 'failed', last_error = $2, updated_at = now() WHERE id = $3`, attachmentID, scanErr.Error(), jobID)
	return err
}

func scanClamAV(ctx context.Context, address string, source io.Reader) (string, error) {
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return "", fmt.Errorf("menghubungi ClamAV: %w", err)
	}
	defer connection.Close()
	if _, err := connection.Write([]byte("zINSTREAM\x00")); err != nil {
		return "", err
	}
	buffer := make([]byte, 32*1024)
	for {
		count, readErr := source.Read(buffer)
		if count > 0 {
			var length [4]byte
			binary.BigEndian.PutUint32(length[:], uint32(count))
			if _, err := connection.Write(length[:]); err != nil {
				return "", err
			}
			if _, err := connection.Write(buffer[:count]); err != nil {
				return "", err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}
	if _, err := connection.Write([]byte{0, 0, 0, 0}); err != nil {
		return "", err
	}
	response, err := io.ReadAll(io.LimitReader(connection, 4096))
	if err != nil {
		return "", err
	}
	if strings.Contains(string(response), " OK") {
		return "clean", nil
	}
	if strings.Contains(string(response), " FOUND") {
		return "infected", nil
	}
	return "", fmt.Errorf("ClamAV mengembalikan respons tidak valid: %s", strings.TrimSpace(string(response)))
}
