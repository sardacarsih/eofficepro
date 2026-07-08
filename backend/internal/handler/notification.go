package handler

// Notifikasi in-app (E08-1/E08-2). Event yang didukung:
//   approval_waiting — giliran approve tiba, untuk pemegang jabatan step waiting
//   approval_result  — hasil akhir approval (terbit/tolak/revisi) untuk pembuat surat
//   letter_incoming  — surat terbit untuk penerima (di-insert distributePublishedLetter)

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

const (
	notificationDefaultLimit = 15
	notificationMaxLimit     = 50
)

// notifyWaitingApprovers memberi tahu seluruh pemegang aktif jabatan approver
// pada step berstatus waiting milik surat ini. Dipanggil di dalam transaksi
// submit/promote sehingga hanya step yang baru menunggu yang ternotifikasi.
func notifyWaitingApprovers(ctx context.Context, tx pgx.Tx, letterID string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO notifications (user_id, event_type, letter_id, title, body)
		SELECT DISTINCT up.user_id,
		       'approval_waiting',
		       l.id,
		       'Menunggu approval: ' || l.subject,
		       'Surat dari ' || cu.full_name || ' menunggu persetujuan Anda sebagai ' || p.title || '.'
		FROM approval_steps s
		JOIN letters l ON l.id = s.letter_id
		JOIN users cu ON cu.id = l.creator_user_id
		JOIN positions p ON p.id = s.approver_position_id
		JOIN user_positions up ON up.position_id = s.approver_position_id
		JOIN users u ON u.id = up.user_id
		WHERE s.letter_id = $1
		  AND s.status = 'waiting'
		  AND current_date >= up.valid_from
		  AND (up.valid_to IS NULL OR current_date < up.valid_to)
		  AND u.status = 'active'`, letterID)
	return err
}

// notifyApprovalResult memberi tahu pembuat surat hasil akhir approval.
func notifyApprovalResult(ctx context.Context, tx pgx.Tx, letterID string, letterStatus string) error {
	var title, body string
	switch letterStatus {
	case "published":
		title = "Surat disetujui & terbit"
		body = "Surat Anda telah disetujui seluruh approver dan terbit dengan nomor resmi."
	case "revision":
		title = "Surat perlu revisi"
		body = "Approver meminta revisi. Buka surat untuk membaca catatan, perbaiki, lalu ajukan ulang."
	case "cancelled":
		title = "Surat ditolak"
		body = "Approver menolak surat Anda. Buka surat untuk membaca alasan penolakan."
	default:
		return nil
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO notifications (user_id, event_type, letter_id, title, body)
		SELECT l.creator_user_id, 'approval_result', l.id, $2 || ': ' || l.subject, $3
		FROM letters l
		WHERE l.id = $1`, letterID, title, body)
	return err
}

type notificationItem struct {
	ID        string     `json:"id"`
	EventType string     `json:"event_type"`
	LetterID  *string    `json:"letter_id"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func (h *Handler) ListNotifications(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	ctx := c.Request.Context()

	limit := notificationDefaultLimit
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit tidak valid"})
			return
		}
		limit = min(parsed, notificationMaxLimit)
	}

	rows, err := h.DB.Query(ctx, `
		SELECT id::text, event_type, letter_id::text, title, body, read_at, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat notifikasi"})
		return
	}
	defer rows.Close()

	items := []notificationItem{}
	for rows.Next() {
		var item notificationItem
		if err := rows.Scan(
			&item.ID,
			&item.EventType,
			&item.LetterID,
			&item.Title,
			&item.Body,
			&item.ReadAt,
			&item.CreatedAt,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca notifikasi"})
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar notifikasi"})
		return
	}

	var unread int
	if err := h.DB.QueryRow(ctx, `
		SELECT count(*) FROM notifications
		WHERE user_id = $1 AND read_at IS NULL`, userID).Scan(&unread); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menghitung notifikasi belum dibaca"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"notifications": items, "unread_count": unread})
}

func (h *Handler) MarkNotificationRead(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	notificationID := c.Param("id")

	tag, err := h.DB.Exec(c.Request.Context(), `
		UPDATE notifications
		SET read_at = now()
		WHERE id = $1 AND user_id = $2 AND read_at IS NULL`, notificationID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menandai notifikasi"})
		return
	}
	if tag.RowsAffected() == 0 {
		// Sudah dibaca atau bukan milik pengguna — idempoten, tetap 200.
		c.JSON(http.StatusOK, gin.H{"id": notificationID, "updated": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": notificationID, "updated": true})
}

func (h *Handler) MarkAllNotificationsRead(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)

	tag, err := h.DB.Exec(c.Request.Context(), `
		UPDATE notifications
		SET read_at = now()
		WHERE user_id = $1 AND read_at IS NULL`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menandai notifikasi"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"marked_read": tag.RowsAffected()})
}
