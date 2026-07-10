package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

type IncomingLetter struct {
	ID                   string     `json:"id"`
	RecipientType        string     `json:"recipient_type"`
	CompanyCode          string     `json:"company_code"`
	LetterTypeCode       string     `json:"letter_type_code"`
	LetterNumber         *string    `json:"letter_number"`
	Subject              string     `json:"subject"`
	Classification       string     `json:"classification"`
	Priority             string     `json:"priority"`
	CreatorName          string     `json:"creator_name"`
	CreatorPositionTitle string     `json:"creator_position_title"`
	BodyPlain            string     `json:"body_plain"`
	AttachmentCount      int        `json:"attachment_count"`
	IsRead               bool       `json:"is_read"`
	FirstReadAt          *time.Time `json:"first_read_at"`
	LastReadAt           *time.Time `json:"last_read_at"`
	DeliveredAt          *time.Time `json:"delivered_at"`
	PublishedAt          *time.Time `json:"published_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

func (h *Handler) ListIncomingLetters(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	box := strings.ToLower(strings.TrimSpace(c.Query("box")))
	if box != "" && box != "to" && box != "cc" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "box harus to atau cc"})
		return
	}

	page, pageSize, offset, ok := parsePagination(c.Query("page"), c.Query("page_size"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page atau page_size tidak valid"})
		return
	}

	filterSQL := ""
	args := []any{userID}
	if box != "" {
		args = append(args, box)
		filterSQL = "AND lr.recipient_type = $2"
	}

	inboxCTE := `
		SELECT DISTINCT ON (l.id, lr.recipient_type)
		       l.id::text AS id, lr.recipient_type, co.code, lt.code, l.letter_number,
		       l.subject, l.classification, l.priority,
		       u.full_name, cp.title, COALESCE(v.body_plain, ''),
		       COALESCE(a.attachment_count, 0),
		       rr.first_read_at IS NOT NULL AS is_read,
		       rr.first_read_at, rr.last_read_at,
		       lr.delivered_at, l.published_at, l.updated_at
		FROM letter_recipients lr
		JOIN letters l ON l.id = lr.letter_id
		JOIN companies co ON co.id = l.company_id
		JOIN letter_types lt ON lt.id = l.letter_type_id
		JOIN users u ON u.id = l.creator_user_id
		JOIN positions cp ON cp.id = l.creator_position_id
		LEFT JOIN read_receipts rr ON rr.letter_id = l.id AND rr.user_id = $1
		LEFT JOIN LATERAL (
			SELECT body_plain
			FROM letter_versions
			WHERE letter_id = l.id
			ORDER BY version DESC
			LIMIT 1
		) v ON true
		LEFT JOIN LATERAL (
			SELECT count(*) AS attachment_count
			FROM letter_attachments
			WHERE letter_id = l.id
		) a ON true
		WHERE l.status = 'published'
		  ` + filterSQL + `
		  AND ` + publishedRecipientAccessSQL("$1") + `
		ORDER BY l.id, lr.recipient_type, l.published_at DESC NULLS LAST, l.updated_at DESC`

	ctx := c.Request.Context()
	var total int64
	if err := h.DB.QueryRow(ctx, `SELECT count(*) FROM (`+inboxCTE+`) inbox`, args...).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menghitung surat masuk"})
		return
	}

	limitArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := h.DB.Query(ctx, `
		SELECT *
		FROM (`+inboxCTE+`) inbox
		ORDER BY published_at DESC NULLS LAST, updated_at DESC
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		limitArgs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat surat masuk"})
		return
	}
	defer rows.Close()

	letters := []IncomingLetter{}
	for rows.Next() {
		var item IncomingLetter
		if err := rows.Scan(
			&item.ID,
			&item.RecipientType,
			&item.CompanyCode,
			&item.LetterTypeCode,
			&item.LetterNumber,
			&item.Subject,
			&item.Classification,
			&item.Priority,
			&item.CreatorName,
			&item.CreatorPositionTitle,
			&item.BodyPlain,
			&item.AttachmentCount,
			&item.IsRead,
			&item.FirstReadAt,
			&item.LastReadAt,
			&item.DeliveredAt,
			&item.PublishedAt,
			&item.UpdatedAt,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca surat masuk"})
			return
		}
		letters = append(letters, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar surat masuk"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": letters, "meta": newPageMeta(page, pageSize, total)})
}

func (h *Handler) userCanReceivePublishedLetter(ctx context.Context, userID string, letterID string) (bool, error) {
	var allowed bool
	err := h.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM letter_recipients lr
			JOIN letters l ON l.id = lr.letter_id
			WHERE lr.letter_id = $1
			  AND l.status = 'published'
			  AND `+publishedRecipientAccessSQL("$2")+`
		)`, letterID, userID).Scan(&allowed)
	return allowed, err
}

func (h *Handler) userIsLetterRecipient(ctx context.Context, userID string, letterID string) (bool, error) {
	var allowed bool
	err := h.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM letter_recipients lr
			JOIN letters l ON l.id = lr.letter_id
			WHERE lr.letter_id = $1
			  AND `+publishedRecipientAccessSQL("$2")+`
		)`, letterID, userID).Scan(&allowed)
	return allowed, err
}

func (h *Handler) markLetterRead(ctx context.Context, userID string, letterID string, ip string) error {
	isRecipient, err := h.userCanReceivePublishedLetter(ctx, userID, letterID)
	if err != nil || !isRecipient {
		return err
	}

	var alreadyRead bool
	if err := h.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM read_receipts
			WHERE letter_id = $1 AND user_id = $2
		)`, letterID, userID).Scan(&alreadyRead); err != nil {
		return err
	}

	if _, err := h.DB.Exec(ctx, `
		INSERT INTO read_receipts (letter_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (letter_id, user_id)
		DO UPDATE SET last_read_at = now()`, letterID, userID); err != nil {
		return err
	}
	if !alreadyRead {
		h.audit(ctx, "letter", &letterID, "read", &userID, nil, ip)
	}
	return nil
}

func distributePublishedLetter(ctx context.Context, tx pgx.Tx, letterID string, publishedAt time.Time) ([]notificationEmail, error) {
	if _, err := tx.Exec(ctx, `
		UPDATE letter_recipients lr
		SET delivered_at = COALESCE(lr.delivered_at, $2),
		    resolved_user_id = COALESCE(lr.resolved_user_id, (
			    SELECT up.user_id
			    FROM user_positions up
			    JOIN users u ON u.id = up.user_id
			    WHERE up.position_id = lr.position_id
			      AND current_date >= up.valid_from
			      AND (up.valid_to IS NULL OR current_date < up.valid_to)
			      AND u.status = 'active'
			    ORDER BY up.valid_from DESC, up.id
			    LIMIT 1
		    ))
		WHERE lr.letter_id = $1
		  AND lr.position_id IS NOT NULL`, letterID, publishedAt); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE letter_recipients
		SET delivered_at = COALESCE(delivered_at, $2)
		WHERE letter_id = $1
		  AND org_unit_id IS NOT NULL`, letterID, publishedAt); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
		WITH RECURSIVE unit_targets AS (
			SELECT lr.id AS recipient_id, lr.recipient_type, lr.org_unit_id AS unit_id
			FROM letter_recipients lr
			WHERE lr.letter_id = $1
			  AND lr.org_unit_id IS NOT NULL
			UNION ALL
			SELECT ut.recipient_id, ut.recipient_type, child.id
			FROM unit_targets ut
			JOIN org_units child ON child.parent_id = ut.unit_id
			WHERE child.is_active
		),
		target_users AS (
			SELECT DISTINCT up.user_id, lr.recipient_type
			FROM letter_recipients lr
			JOIN user_positions up ON up.position_id = lr.position_id
			JOIN users u ON u.id = up.user_id
			WHERE lr.letter_id = $1
			  AND lr.position_id IS NOT NULL
			  AND current_date >= up.valid_from
			  AND (up.valid_to IS NULL OR current_date < up.valid_to)
			  AND u.status = 'active'
			UNION
			SELECT DISTINCT up.user_id, ut.recipient_type
			FROM unit_targets ut
			JOIN positions p ON p.org_unit_id = ut.unit_id
			JOIN user_positions up ON up.position_id = p.id
			JOIN users u ON u.id = up.user_id
			WHERE current_date >= up.valid_from
			  AND (up.valid_to IS NULL OR current_date < up.valid_to)
			  AND p.is_active
			  AND u.status = 'active'
		),
		letter_data AS (
			SELECT subject, classification
			FROM letters
			WHERE id = $1
		),
		inserted AS (
			INSERT INTO notifications (user_id, event_type, letter_id, title, body)
			SELECT tu.user_id,
			       'letter_incoming',
			       $1,
			       'Surat masuk: ' || ld.subject,
			       CASE WHEN tu.recipient_type = 'cc'
			            THEN 'Anda menerima tembusan surat terbit.'
			            ELSE 'Anda menerima surat terbit untuk ditindaklanjuti.'
			       END
			FROM target_users tu
			CROSS JOIN letter_data ld
			WHERE ld.classification <> 'rahasia' OR tu.recipient_type = 'to'
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

func recipientAccessSQL(userParam string) string {
	return `(
		lr.resolved_user_id = ` + userParam + `
		OR EXISTS (
			SELECT 1
			FROM user_positions up
			WHERE up.user_id = ` + userParam + `
			  AND up.position_id = lr.position_id
			  AND current_date >= up.valid_from
			  AND (up.valid_to IS NULL OR current_date < up.valid_to)
		)
		OR EXISTS (
			WITH RECURSIVE ancestors AS (
				SELECT p.org_unit_id AS id, ou.parent_id
				FROM user_positions up
				JOIN positions p ON p.id = up.position_id
				JOIN org_units ou ON ou.id = p.org_unit_id
				WHERE up.user_id = ` + userParam + `
				  AND current_date >= up.valid_from
				  AND (up.valid_to IS NULL OR current_date < up.valid_to)
				  AND p.is_active
				  AND ou.is_active
				UNION ALL
				SELECT parent.id, parent.parent_id
				FROM org_units parent
				JOIN ancestors child ON child.parent_id = parent.id
				WHERE parent.is_active
			)
			SELECT 1
			FROM ancestors
			WHERE ancestors.id = lr.org_unit_id
		)
	)`
}

// publishedRecipientAccessSQL applies the additional classification rule for
// published letters: confidential letters are only available to primary (To)
// recipients. Workflow approvers use their separate access path.
func publishedRecipientAccessSQL(userParam string) string {
	return `(
		` + recipientAccessSQL(userParam) + `
		AND (l.classification <> 'rahasia' OR lr.recipient_type = 'to')
	)`
}
