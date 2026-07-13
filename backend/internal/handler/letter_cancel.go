package handler

// Pembatalan surat oleh pembuat sebelum approval final (E03-7, PRD P0-3).
// Transisi atomik: lock surat -> validasi status -> skip step pending/waiting ->
// status cancelled + kolom jejak -> notifikasi (in-app + outbox) -> commit -> audit.
// Surat yang dibatalkan tidak pernah menerima nomor (letter_number tetap NULL).

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

const maxCancelReasonRunes = 1000

type cancelLetterRequest struct {
	Reason string `json:"reason"`
}

func (h *Handler) CancelLetter(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	letterID := c.Param("id")
	ctx := c.Request.Context()

	var req cancelLetterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data pembatalan tidak valid: " + err.Error()})
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "alasan pembatalan wajib diisi"})
		return
	}
	if len([]rune(req.Reason)) > maxCancelReasonRunes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "alasan pembatalan maksimal 1000 karakter"})
		return
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi"})
		return
	}
	defer tx.Rollback(ctx)

	// Lock surat milik pembuat; ActApprovalStep memakai urutan lock yang sama
	// (letters dulu, baru approval_steps) sehingga race cancel vs approve
	// bebas deadlock dan tepat satu yang menang.
	var previousStatus string
	err = tx.QueryRow(ctx, `
		SELECT status
		FROM letters
		WHERE id = $1 AND creator_user_id = $2
		FOR UPDATE`, letterID, userID).Scan(&previousStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "surat tidak ditemukan"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca surat"})
		return
	}

	switch previousStatus {
	case "draft", "revision", "in_approval":
		// dapat dibatalkan
	case "cancelled":
		c.JSON(http.StatusConflict, gin.H{"error": "surat sudah dibatalkan"})
		return
	default:
		c.JSON(http.StatusConflict, gin.H{"error": "surat sudah disetujui final"})
		return
	}

	if _, err := tx.Exec(ctx, `
		UPDATE approval_steps
		SET status = 'skipped'
		WHERE letter_id = $1 AND status IN ('pending','waiting')`, letterID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menutup sisa approval"})
		return
	}

	var cancelledAt time.Time
	var cancelledByName string
	err = tx.QueryRow(ctx, `
		UPDATE letters l
		SET status = 'cancelled',
		    current_step_order = NULL,
		    cancelled_at = now(),
		    cancelled_by_user_id = $2,
		    cancel_reason = $3,
		    updated_at = now()
		FROM users u
		WHERE l.id = $1 AND u.id = $2
		RETURNING l.cancelled_at, u.full_name`, letterID, userID, req.Reason).Scan(&cancelledAt, &cancelledByName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membatalkan surat"})
		return
	}

	pendingEmails, err := notifyLetterCancelledByCreator(ctx, tx, letterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengirim notifikasi pembatalan"})
		return
	}
	if err := enqueueNotificationOutbox(ctx, tx, pendingEmails); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengantrikan notifikasi pembatalan"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan pembatalan surat"})
		return
	}

	h.audit(ctx, "letter", &letterID, "letter_cancelled", &userID, map[string]any{
		"reason":          req.Reason,
		"previous_status": previousStatus,
	}, c.ClientIP())

	c.JSON(http.StatusOK, gin.H{"letter": gin.H{
		"id":                letterID,
		"status":            "cancelled",
		"cancelled_at":      cancelledAt,
		"cancelled_by_name": cancelledByName,
		"cancel_reason":     req.Reason,
	}})
}

// notifyLetterCancelledByCreator memberi tahu approver cycle berjalan bahwa
// surat dibatalkan oleh pembuat: pemegang aktif posisi step yang sudah
// approve atau yang step-nya di-skip, plus delegate aktif (E03-5) posisi
// tersebut. Jangan reuse notifyApprovalResult — copy "cancelled"-nya
// berkonteks penolakan approver, bukan pembatalan oleh pembuat.
func notifyLetterCancelledByCreator(ctx context.Context, tx pgx.Tx, letterID string) ([]notificationEmail, error) {
	rows, err := tx.Query(ctx, `
		WITH cycle_steps AS (
			SELECT s.approver_position_id
			FROM approval_steps s
			WHERE s.letter_id = $1
			  AND s.approval_cycle = (
				SELECT COALESCE(MAX(approval_cycle), 0)
				FROM approval_steps
				WHERE letter_id = $1
			  )
			  AND s.status IN ('approved','skipped')
		),
		targets AS (
			SELECT DISTINCT combined.user_id
			FROM (
				SELECT up.user_id
				FROM cycle_steps cs
				JOIN user_positions up ON up.position_id = cs.approver_position_id
				JOIN users u ON u.id = up.user_id
				WHERE current_date >= up.valid_from
				  AND (up.valid_to IS NULL OR current_date < up.valid_to)
				  AND u.status = 'active'
				UNION
				SELECT dg.delegate_user_id
				FROM cycle_steps cs
				JOIN delegations dg ON dg.delegator_position_id = cs.approver_position_id
				JOIN users u ON u.id = dg.delegate_user_id
				WHERE now() >= dg.valid_from AND now() < dg.valid_to
				  AND dg.revoked_at IS NULL
				  AND u.status = 'active'
			) combined
		),
		inserted AS (
			INSERT INTO notifications (user_id, event_type, letter_id, title, body)
			SELECT t.user_id, 'letter_cancelled', l.id,
			       'Surat dibatalkan: ' || l.subject,
			       'Surat ini dibatalkan oleh pembuatnya (' || cu.full_name || '). Alasan: ' || l.cancel_reason
			FROM targets t
			CROSS JOIN letters l
			JOIN users cu ON cu.id = l.creator_user_id
			WHERE l.id = $1
			RETURNING user_id, event_type, letter_id, title, body
		)
		SELECT u.email, i.event_type, i.letter_id::text, i.title, i.body
		FROM inserted i
		JOIN users u ON u.id = i.user_id`, letterID)
	if err != nil {
		return nil, err
	}
	return collectNotificationEmails(rows)
}
