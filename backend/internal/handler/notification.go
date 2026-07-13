package handler

// Notifikasi in-app (E08-1/E08-2). Event yang didukung:
//   approval_waiting — giliran approve tiba, untuk pemegang jabatan step waiting
//   approval_result  — hasil akhir approval (terbit/tolak/revisi) untuk pembuat surat
//   letter_incoming  — surat terbit untuk penerima (di-insert distributePublishedLetter)

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/kskgroup/eofficepro/internal/middleware"
	"github.com/kskgroup/eofficepro/internal/push"
)

const (
	notificationDefaultLimit = 15
	notificationMaxLimit     = 50
)

// notificationEmail adalah salinan notifikasi yang baru dibuat untuk dikirim
// juga sebagai email (E08-3) setelah transaksi commit.
type notificationEmail struct {
	Email     string
	EventType string
	LetterID  string
	Title     string
	Body      string
}

func collectNotificationEmails(rows pgx.Rows) ([]notificationEmail, error) {
	defer rows.Close()
	items := []notificationEmail{}
	for rows.Next() {
		var item notificationEmail
		if err := rows.Scan(&item.Email, &item.EventType, &item.LetterID, &item.Title, &item.Body); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// sendNotificationEmails mengirim email notifikasi (deep link ke web) secara
// asinkron — kegagalan SMTP tidak menggagalkan aksi utamanya.
func (h *Handler) sendNotificationEmails(items []notificationEmail) {
	if len(items) == 0 {
		return
	}
	h.sendNotificationPushes(items)
	go func() {
		for _, item := range items {
			body := item.Body + "\n\nBuka di eOffice Pro:\n" + h.notificationLink(item)
			if err := h.Mailer.Send(item.Email, item.Title, body); err != nil {
				log.Printf("email notifikasi gagal ke %s: %v", item.Email, err)
			}
		}
	}()
}

func (h *Handler) sendNotificationPushes(items []notificationEmail) {
	if h.Push == nil || !h.Push.Enabled() || len(items) == 0 {
		return
	}
	go func() {
		ctx := context.Background()
		for _, item := range items {
			tokens, err := h.pushTokensForEmail(ctx, item.Email)
			if err != nil {
				log.Printf("query FCM tokens failed for %s: %v", item.Email, err)
				continue
			}
			if len(tokens) == 0 {
				log.Printf("push dilewati untuk %s: tidak ada device token terdaftar", item.Email)
				continue
			}
			classification, err := h.pushNotificationClassification(ctx, item.LetterID)
			if err != nil {
				log.Printf("query notification classification failed for %s: %v", item.LetterID, err)
			}
			invalidTokens := h.Push.SendToTokens(ctx, tokens, buildPushMessage(item, classification))
			if len(invalidTokens) > 0 {
				if err := h.deletePushTokens(ctx, invalidTokens); err != nil {
					log.Printf("delete invalid FCM tokens failed: %v", err)
				}
			}
		}
	}()
}

func buildPushMessage(item notificationEmail, classification string) push.Message {
	message := push.Message{
		Title:         item.Title,
		Body:          truncatePushBody(item.Body),
		LetterID:      item.LetterID,
		Event:         item.EventType,
		TargetSection: notificationTargetSection(item.EventType),
	}
	if classification == "rahasia" {
		message.Title = "Notifikasi eOffice Pro"
		message.Body = "Ada pembaruan surat rahasia. Buka eOffice Pro untuk melihat detail."
	}
	return message
}

func truncatePushBody(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= 180 {
		return value
	}
	return string(runes[:177]) + "..."
}

func notificationTargetSection(eventType string) string {
	switch eventType {
	case "approval_waiting", "sla_reminder", "sla_escalation":
		return "approvals"
	case "disposition_assigned", "disposition_updated":
		return "dispositions"
	case "letter_incoming", "approval_result", "letter_cancelled":
		return "inbox"
	default:
		return "dashboard"
	}
}

// notificationLink membangun deep link web untuk email/push notifikasi.
func (h *Handler) notificationLink(item notificationEmail) string {
	switch item.EventType {
	case "approval_waiting":
		return h.Cfg.WebBaseURL + "/approvals"
	case "delegation_created", "delegation_revoked":
		return h.Cfg.WebBaseURL + "/delegations"
	default:
		return h.Cfg.WebBaseURL + "/letters/" + item.LetterID
	}
}

func (h *Handler) pushNotificationClassification(ctx context.Context, letterID string) (string, error) {
	if strings.TrimSpace(letterID) == "" {
		return "", nil
	}
	var classification string
	err := h.DB.QueryRow(ctx, `
		SELECT classification
		FROM letters
		WHERE id = $1`, letterID).Scan(&classification)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return classification, err
}

func (h *Handler) pushTokensForEmail(ctx context.Context, email string) ([]string, error) {
	rows, err := h.DB.Query(ctx, `
		SELECT pt.token
		FROM user_push_tokens pt
		JOIN users u ON u.id = pt.user_id
		WHERE u.email = $1
		  AND u.status = 'active'`, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tokens := []string{}
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

func (h *Handler) deletePushTokens(ctx context.Context, tokens []string) error {
	_, err := h.DB.Exec(ctx, `
		DELETE FROM user_push_tokens
		WHERE token = ANY($1::text[])`, tokens)
	return err
}

type pushTokenRequest struct {
	Token      string `json:"token"`
	Platform   string `json:"platform"`
	DeviceInfo string `json:"device_info"`
	AppVersion string `json:"app_version"`
	DeviceID   string `json:"device_id"`
}

func (h *Handler) RegisterPushToken(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	var req pushTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data token push tidak valid"})
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	req.Platform = strings.ToLower(strings.TrimSpace(req.Platform))
	req.DeviceInfo = strings.TrimSpace(req.DeviceInfo)
	req.AppVersion = strings.TrimSpace(req.AppVersion)
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if req.Platform == "" {
		req.Platform = "android"
	}
	if req.Token == "" || len(req.Token) > 4096 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token push tidak valid"})
		return
	}
	if req.Platform != "android" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform push tidak didukung"})
		return
	}
	if len(req.DeviceInfo) > 255 {
		req.DeviceInfo = req.DeviceInfo[:255]
	}
	if len(req.AppVersion) > 50 {
		req.AppVersion = req.AppVersion[:50]
	}
	if len(req.DeviceID) > 150 {
		req.DeviceID = req.DeviceID[:150]
	}

	if _, err := h.DB.Exec(c.Request.Context(), `
		INSERT INTO user_push_tokens (user_id, token, platform, device_info, app_version, device_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (token) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    platform = EXCLUDED.platform,
		    device_info = EXCLUDED.device_info,
		    app_version = EXCLUDED.app_version,
		    device_id = EXCLUDED.device_id,
		    last_seen_at = now()`,
		userID,
		req.Token,
		req.Platform,
		nullableString(req.DeviceInfo),
		nullableString(req.AppVersion),
		nullableString(req.DeviceID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan token push"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"registered": true})
}

func (h *Handler) UnregisterPushToken(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	var req pushTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data token push tidak valid"})
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token push tidak valid"})
		return
	}
	tag, err := h.DB.Exec(c.Request.Context(), `
		DELETE FROM user_push_tokens
		WHERE user_id = $1 AND token = $2`, userID, req.Token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menghapus token push"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"removed": tag.RowsAffected() > 0})
}

// notifyWaitingApprovers memberi tahu seluruh pemegang aktif jabatan approver
// pada step berstatus waiting milik surat ini. Dipanggil di dalam transaksi
// submit/promote sehingga hanya step yang baru menunggu yang ternotifikasi.
// Mengembalikan data email untuk dikirim setelah commit.
func notifyWaitingApprovers(ctx context.Context, tx pgx.Tx, letterID string) ([]notificationEmail, error) {
	// Target = pemegang aktif posisi step waiting UNION delegate aktif (E03-5);
	// baris identik ter-dedup oleh UNION + DISTINCT (satu notifikasi per user).
	rows, err := tx.Query(ctx, `
		WITH targets AS (
			SELECT DISTINCT combined.user_id, combined.letter_id, combined.subject,
			       combined.creator_name, combined.position_title
			FROM (
				SELECT up.user_id, l.id AS letter_id, l.subject,
				       cu.full_name AS creator_name, p.title AS position_title
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
				  AND u.status = 'active'
				UNION
				SELECT dg.delegate_user_id, l.id, l.subject, cu.full_name, p.title
				FROM approval_steps s
				JOIN letters l ON l.id = s.letter_id
				JOIN users cu ON cu.id = l.creator_user_id
				JOIN positions p ON p.id = s.approver_position_id
				JOIN delegations dg ON dg.delegator_position_id = s.approver_position_id
				JOIN users u ON u.id = dg.delegate_user_id
				WHERE s.letter_id = $1
				  AND s.status = 'waiting'
				  AND now() >= dg.valid_from AND now() < dg.valid_to
				  AND dg.revoked_at IS NULL
				  AND u.status = 'active'
			) combined
		),
		inserted AS (
			INSERT INTO notifications (user_id, event_type, letter_id, title, body)
			SELECT t.user_id,
			       'approval_waiting',
			       t.letter_id,
			       'Menunggu approval: ' || t.subject,
			       'Surat dari ' || t.creator_name || ' menunggu persetujuan Anda sebagai ' || t.position_title || '.'
			FROM targets t
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

// notifyApprovalResult memberi tahu pembuat surat hasil akhir approval.
// Mengembalikan data email untuk dikirim setelah commit.
func notifyApprovalResult(ctx context.Context, tx pgx.Tx, letterID string, letterStatus string) ([]notificationEmail, error) {
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
		return nil, nil
	}

	rows, err := tx.Query(ctx, `
		WITH inserted AS (
			INSERT INTO notifications (user_id, event_type, letter_id, title, body)
			SELECT l.creator_user_id, 'approval_result', l.id, $2 || ': ' || l.subject, $3
			FROM letters l
			WHERE l.id = $1
			RETURNING user_id, event_type, letter_id, title, body
		)
		SELECT u.email, i.event_type, i.letter_id::text, i.title, i.body
		FROM inserted i
		JOIN users u ON u.id = i.user_id`, letterID, title, body)
	if err != nil {
		return nil, err
	}
	return collectNotificationEmails(rows)
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

	page, pageSize, offset, ok := parsePagination(c.Query("page"), c.Query("page_size"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page atau page_size tidak valid"})
		return
	}
	if c.Query("page_size") == "" {
		if raw := c.Query("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "limit tidak valid"})
				return
			}
			pageSize = min(parsed, notificationMaxLimit)
		} else {
			pageSize = notificationDefaultLimit
		}
		offset = (page - 1) * pageSize
	}

	var total int64
	if err := h.DB.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE user_id = $1`, userID).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menghitung notifikasi"})
		return
	}

	rows, err := h.DB.Query(ctx, `
		SELECT id::text, event_type, letter_id::text, title, body, read_at, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, userID, pageSize, offset)
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

	c.JSON(http.StatusOK, gin.H{
		"data": items, "unread_count": unread,
		"meta": newPageMeta(page, pageSize, total),
	})
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
