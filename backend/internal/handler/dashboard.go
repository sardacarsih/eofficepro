package handler

// Ringkasan dashboard per pengguna: KPI, trend surat masuk 30 hari,
// aktivitas terakhir (dari notifikasi), dan daftar approval menunggu.

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

type dashboardStats struct {
	InboxUnread      int `json:"inbox_unread"`
	SentThisMonth    int `json:"sent_this_month"`
	PendingApprovals int `json:"pending_approvals"`
	ArchivedTotal    int `json:"archived_total"`
}

type dashboardTrendPoint struct {
	Date  string `json:"date"`
	Total int    `json:"total"`
}

type dashboardActivity struct {
	ID        string    `json:"id"`
	EventType string    `json:"event_type"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

type dashboardPendingApproval struct {
	StepID      string    `json:"step_id"`
	Subject     string    `json:"subject"`
	CreatorName string    `json:"creator_name"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (h *Handler) DashboardSummary(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	ctx := c.Request.Context()

	var stats dashboardStats
	err := h.DB.QueryRow(ctx, `
		WITH my_incoming AS (
			SELECT DISTINCT l.id, l.published_at
			FROM letter_recipients lr
			JOIN letters l ON l.id = lr.letter_id
			WHERE l.status = 'published'
			  AND `+recipientAccessSQL("$1")+`
		)
		SELECT
			(SELECT count(*) FROM my_incoming mi
			 WHERE NOT EXISTS (
				SELECT 1 FROM read_receipts rr
				WHERE rr.letter_id = mi.id AND rr.user_id = $1
			 )),
			(SELECT count(*) FROM letters
			 WHERE creator_user_id = $1
			   AND status = 'published'
			   AND published_at >= date_trunc('month', now())),
			(SELECT count(*)
			 FROM approval_steps s
			 JOIN letters l ON l.id = s.letter_id
			 JOIN user_positions up ON up.position_id = s.approver_position_id
			 WHERE up.user_id = $1
			   AND current_date >= up.valid_from
			   AND (up.valid_to IS NULL OR current_date < up.valid_to)
			   AND s.status = 'waiting'
			   AND l.status = 'in_approval'),
			(SELECT count(*) FROM (
				SELECT id FROM my_incoming
				UNION
				SELECT id FROM letters
				WHERE creator_user_id = $1 AND status = 'published'
			) archived)`, userID).Scan(
		&stats.InboxUnread,
		&stats.SentThisMonth,
		&stats.PendingApprovals,
		&stats.ArchivedTotal,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat statistik dashboard"})
		return
	}

	trend := []dashboardTrendPoint{}
	rows, err := h.DB.Query(ctx, `
		WITH my_incoming AS (
			SELECT DISTINCT l.id, l.published_at
			FROM letter_recipients lr
			JOIN letters l ON l.id = lr.letter_id
			WHERE l.status = 'published'
			  AND l.published_at >= current_date - interval '29 days'
			  AND `+recipientAccessSQL("$1")+`
		)
		SELECT d.day::date::text, count(mi.id)
		FROM generate_series(current_date - interval '29 days', current_date, interval '1 day') AS d(day)
		LEFT JOIN my_incoming mi ON mi.published_at::date = d.day::date
		GROUP BY d.day
		ORDER BY d.day`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat trend surat masuk"})
		return
	}
	defer rows.Close()
	for rows.Next() {
		var point dashboardTrendPoint
		if err := rows.Scan(&point.Date, &point.Total); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca trend surat masuk"})
			return
		}
		trend = append(trend, point)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca trend surat masuk"})
		return
	}

	activities := []dashboardActivity{}
	activityRows, err := h.DB.Query(ctx, `
		SELECT id::text, event_type, title, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 5`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat aktivitas"})
		return
	}
	defer activityRows.Close()
	for activityRows.Next() {
		var activity dashboardActivity
		if err := activityRows.Scan(&activity.ID, &activity.EventType, &activity.Title, &activity.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca aktivitas"})
			return
		}
		activities = append(activities, activity)
	}
	if err := activityRows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar aktivitas"})
		return
	}

	pending := []dashboardPendingApproval{}
	pendingRows, err := h.DB.Query(ctx, `
		SELECT s.id::text, l.subject, u.full_name, l.updated_at
		FROM approval_steps s
		JOIN letters l ON l.id = s.letter_id
		JOIN users u ON u.id = l.creator_user_id
		JOIN user_positions up ON up.position_id = s.approver_position_id
		WHERE up.user_id = $1
		  AND current_date >= up.valid_from
		  AND (up.valid_to IS NULL OR current_date < up.valid_to)
		  AND s.status = 'waiting'
		  AND l.status = 'in_approval'
		ORDER BY l.priority DESC, l.updated_at ASC
		LIMIT 5`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat approval menunggu"})
		return
	}
	defer pendingRows.Close()
	for pendingRows.Next() {
		var item dashboardPendingApproval
		if err := pendingRows.Scan(&item.StepID, &item.Subject, &item.CreatorName, &item.UpdatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca approval menunggu"})
			return
		}
		pending = append(pending, item)
	}
	if err := pendingRows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar approval menunggu"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stats":             stats,
		"incoming_trend":    trend,
		"recent_activities": activities,
		"pending_approvals": pending,
	})
}
