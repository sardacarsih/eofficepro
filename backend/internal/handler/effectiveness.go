package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

type effectivenessResponse struct {
	From               string `json:"from"`
	To                 string `json:"to"`
	ActiveUsers        int64  `json:"active_users"`
	RegisteredUsers    int64  `json:"registered_users"`
	LettersCreated     int64  `json:"letters_created"`
	LettersPublished   int64  `json:"letters_published"`
	PendingApprovals   int64  `json:"pending_approvals"`
	OverdueApprovals   int64  `json:"overdue_approvals"`
	ApprovalActions    int64  `json:"approval_actions"`
	ReadNotifications  int64  `json:"read_notifications"`
	TotalNotifications int64  `json:"total_notifications"`
}

func (h *Handler) ManagementEffectiveness(c *gin.Context) {
	to := time.Now().UTC()
	from := to.AddDate(0, 0, -30)
	layout := "2006-01-02"
	if v := c.Query("from"); v != "" {
		if parsed, err := time.Parse(layout, v); err == nil {
			from = parsed
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from tidak valid"})
			return
		}
	}
	if v := c.Query("to"); v != "" {
		if parsed, err := time.Parse(layout, v); err == nil {
			to = parsed.Add(24 * time.Hour)
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "to tidak valid"})
			return
		}
	}
	if !from.Before(to) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rentang tanggal tidak valid"})
		return
	}
	ctx := c.Request.Context()
	var result effectivenessResponse
	err := h.DB.QueryRow(ctx, `
      SELECT
        (SELECT count(DISTINCT actor_user_id) FROM audit_logs WHERE actor_user_id IS NOT NULL AND created_at >= $1 AND created_at < $2),
        (SELECT count(*) FROM users WHERE status = 'active'),
        (SELECT count(*) FROM letters WHERE created_at >= $1 AND created_at < $2),
        (SELECT count(*) FROM letters WHERE status = 'published' AND published_at >= $1 AND published_at < $2),
        (SELECT count(*) FROM approval_steps s JOIN letters l ON l.id=s.letter_id WHERE s.status='waiting' AND l.status='in_approval'),
        (SELECT count(*) FROM approval_steps s JOIN letters l ON l.id=s.letter_id WHERE s.status='waiting' AND l.status='in_approval' AND l.updated_at < now() - interval '48 hours'),
        (SELECT count(*) FROM approval_actions WHERE acted_at >= $1 AND acted_at < $2),
        (SELECT count(*) FROM notifications WHERE read_at IS NOT NULL AND read_at >= $1 AND read_at < $2),
        (SELECT count(*) FROM notifications WHERE created_at >= $1 AND created_at < $2)`, from, to).Scan(&result.ActiveUsers, &result.RegisteredUsers, &result.LettersCreated, &result.LettersPublished, &result.PendingApprovals, &result.OverdueApprovals, &result.ApprovalActions, &result.ReadNotifications, &result.TotalNotifications)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat efektivitas aplikasi"})
		return
	}
	result.From = from.Format(layout)
	result.To = to.Add(-24 * time.Hour).Format(layout)
	c.JSON(http.StatusOK, result)
}
