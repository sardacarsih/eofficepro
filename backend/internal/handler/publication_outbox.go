package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
)

const publicationOutboxInterval = 5 * time.Second

// RunPublicationOutboxWorker finalizes approved letters outside the approver's
// request. A storage outage leaves the numbered letter in approved state and
// retries publication without replaying approval or numbering.
func (h *Handler) RunPublicationOutboxWorker(ctx context.Context) {
	ticker := time.NewTicker(publicationOutboxInterval)
	defer ticker.Stop()
	for {
		for h.processNextPublication(ctx) {
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (h *Handler) processNextPublication(ctx context.Context) bool {
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return false
	}
	defer tx.Rollback(ctx)
	var jobID, letterID string
	err = tx.QueryRow(ctx, `
		SELECT id::text, letter_id::text
		FROM letter_publication_jobs
		WHERE (status IN ('pending', 'retry') AND available_at <= now())
		   OR (status = 'processing' AND updated_at < now() - interval '5 minutes')
		ORDER BY created_at
		FOR UPDATE SKIP LOCKED
		LIMIT 1`).Scan(&jobID, &letterID)
	if err != nil {
		return false
	}
	if _, err := tx.Exec(ctx, `
		UPDATE letter_publication_jobs
		SET status = 'processing', attempts = attempts + 1, updated_at = now()
		WHERE id = $1`, jobID); err != nil {
		return false
	}
	if err := tx.Commit(ctx); err != nil {
		return false
	}

	if err := h.publishApprovedLetter(ctx, jobID, letterID); err != nil {
		_, _ = h.DB.Exec(ctx, `
			UPDATE letter_publication_jobs
			SET status = 'retry', available_at = now() + make_interval(secs => LEAST(3600, 30 * attempts)),
				last_error = $2, updated_at = now()
			WHERE id = $1`, jobID, err.Error())
		return true
	}
	return true
}

func (h *Handler) publishApprovedLetter(ctx context.Context, jobID, letterID string) error {
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var letterNumber string
	var status string
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(letter_number, ''), status
		FROM letters
		WHERE id = $1
		FOR UPDATE`, letterID).Scan(&letterNumber, &status)
	if err != nil {
		return err
	}
	if status == "published" {
		_, err := tx.Exec(ctx, `UPDATE letter_publication_jobs SET status = 'published', published_at = COALESCE(published_at, now()), updated_at = now() WHERE id = $1`, jobID)
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if status != "approved" || letterNumber == "" {
		return fmt.Errorf("surat tidak siap dipublikasikan")
	}
	publishedAt := time.Now()
	finalPDFKey, err := h.renderAndStoreFinalPDF(ctx, tx, letterID, letterNumber, publishedAt)
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup && h.Minio != nil {
			_ = h.Minio.RemoveObject(context.Background(), h.Bucket, finalPDFKey, minio.RemoveObjectOptions{})
		}
	}()
	if _, err := tx.Exec(ctx, `
		UPDATE letters
		SET status = 'published', final_pdf_key = $2, published_at = $3, updated_at = now()
		WHERE id = $1`, letterID, finalPDFKey, publishedAt); err != nil {
		return err
	}
	incoming, err := distributePublishedLetter(ctx, tx, letterID, publishedAt)
	if err != nil {
		return err
	}
	result, err := notifyApprovalResult(ctx, tx, letterID, "published")
	if err != nil {
		return err
	}
	if err := enqueueNotificationOutbox(ctx, tx, append(incoming, result...)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE letter_publication_jobs
		SET status = 'published', published_at = $2, last_error = NULL, updated_at = now()
		WHERE id = $1`, jobID, publishedAt); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	cleanup = false
	h.audit(ctx, "letter", &letterID, "publish", nil, map[string]any{"final_pdf_key": finalPDFKey}, "")
	return nil
}
