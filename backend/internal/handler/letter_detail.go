package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

type LetterDetail struct {
	ID                   string                 `json:"id"`
	CompanyCode          string                 `json:"company_code"`
	CompanyName          string                 `json:"company_name"`
	LetterTypeCode       string                 `json:"letter_type_code"`
	LetterTypeName       string                 `json:"letter_type_name"`
	LetterNumber         *string                `json:"letter_number"`
	Subject              string                 `json:"subject"`
	Classification       string                 `json:"classification"`
	Priority             string                 `json:"priority"`
	Status               string                 `json:"status"`
	CreatorName          string                 `json:"creator_name"`
	CreatorPositionTitle string                 `json:"creator_position_title"`
	OnBehalfOfTitle      *string                `json:"on_behalf_of_title"`
	Version              int                    `json:"version"`
	BodyHTML             string                 `json:"body_html"`
	BodyPlain            string                 `json:"body_plain"`
	QRToken              *string                `json:"qr_token"`
	VerifyURL            *string                `json:"verify_url"`
	FinalPDFURL          *string                `json:"final_pdf_url"`
	Recipients           []DraftRecipient       `json:"recipients"`
	Attachments          []LetterAttachment     `json:"attachments"`
	ApprovalSteps        []LetterApprovalStep   `json:"approval_steps"`
	ApprovalActions      []LetterApprovalAction `json:"approval_actions"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
	PublishedAt          *time.Time             `json:"published_at"`
}

type LetterApprovalStep struct {
	ID            string     `json:"id"`
	ApprovalCycle int        `json:"approval_cycle"`
	StepOrder     int        `json:"step_order"`
	FlowGroup     int        `json:"flow_group"`
	Status        string     `json:"status"`
	PositionTitle string     `json:"position_title"`
	PositionType  string     `json:"position_type"`
	SLADeadline   *time.Time `json:"sla_deadline"`
	DecidedAt     *time.Time `json:"decided_at"`
}

type LetterApprovalAction struct {
	ID               string    `json:"id"`
	StepID           string    `json:"step_id"`
	Action           string    `json:"action"`
	ActorName        string    `json:"actor_name"`
	Note             *string   `json:"note"`
	DeviceInfo       *string   `json:"device_info"`
	CreatedAt        time.Time `json:"created_at"`
	PositionTitle    string    `json:"position_title"`
	SignaturePresent bool      `json:"signature_present"`
}

func (h *Handler) GetLetterDetail(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	letterID := c.Param("id")
	ctx := c.Request.Context()

	allowed, err := h.userCanViewLetter(ctx, userID, letterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa akses surat"})
		return
	}
	if !allowed {
		c.JSON(http.StatusNotFound, gin.H{"error": "surat tidak ditemukan"})
		return
	}
	if err := h.markLetterRead(ctx, userID, letterID, c.ClientIP()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mencatat tanda baca surat"})
		return
	}

	var detail LetterDetail
	var finalPDFKey *string
	err = h.DB.QueryRow(ctx, `
		SELECT l.id::text, co.code, co.name, lt.code, lt.name, l.letter_number,
		       l.subject, l.classification, l.priority, l.status,
		       u.full_name, p.title, obp.title,
		       COALESCE(v.version, 0), COALESCE(v.body_html, ''), COALESCE(v.body_plain, ''),
		       l.qr_token, l.final_pdf_key, l.created_at, l.updated_at, l.published_at
		FROM letters l
		JOIN companies co ON co.id = l.company_id
		JOIN letter_types lt ON lt.id = l.letter_type_id
		JOIN users u ON u.id = l.creator_user_id
		JOIN positions p ON p.id = l.creator_position_id
		LEFT JOIN positions obp ON obp.id = l.on_behalf_of_position_id
		LEFT JOIN LATERAL (
			SELECT version, body_html, body_plain
			FROM letter_versions
			WHERE letter_id = l.id
			ORDER BY version DESC
			LIMIT 1
		) v ON true
		WHERE l.id = $1`, letterID).Scan(
		&detail.ID,
		&detail.CompanyCode,
		&detail.CompanyName,
		&detail.LetterTypeCode,
		&detail.LetterTypeName,
		&detail.LetterNumber,
		&detail.Subject,
		&detail.Classification,
		&detail.Priority,
		&detail.Status,
		&detail.CreatorName,
		&detail.CreatorPositionTitle,
		&detail.OnBehalfOfTitle,
		&detail.Version,
		&detail.BodyHTML,
		&detail.BodyPlain,
		&detail.QRToken,
		&finalPDFKey,
		&detail.CreatedAt,
		&detail.UpdatedAt,
		&detail.PublishedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "surat tidak ditemukan"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat detail surat"})
		return
	}
	if detail.QRToken != nil && *detail.QRToken != "" {
		verifyURL := h.verifyURL(*detail.QRToken)
		detail.VerifyURL = &verifyURL
	}
	_ = finalPDFKey // Download PDF selalu lewat endpoint terautentikasi.

	recipients, ok := h.loadLetterRecipients(c, letterID)
	if !ok {
		return
	}
	detail.Recipients = recipients

	attachments, ok := h.loadLetterAttachments(c, letterID, true)
	if !ok {
		return
	}
	detail.Attachments = attachments

	steps, ok := h.loadLetterApprovalSteps(c, letterID)
	if !ok {
		return
	}
	detail.ApprovalSteps = steps

	actions, ok := h.loadLetterApprovalActions(c, letterID)
	if !ok {
		return
	}
	detail.ApprovalActions = actions

	c.JSON(http.StatusOK, gin.H{"letter": detail})
}

func (h *Handler) userCanViewLetter(ctx context.Context, userID string, letterID string) (bool, error) {
	var allowed bool
	err := h.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM letters l
			WHERE l.id = $1 AND (
				l.creator_user_id = $2 OR EXISTS (
			SELECT 1
			FROM approval_steps s
			JOIN user_positions up ON up.position_id = s.approver_position_id
			WHERE s.letter_id = $1
			  AND up.user_id = $2
			  AND current_date >= up.valid_from
			  AND (up.valid_to IS NULL OR current_date < up.valid_to)
		) OR EXISTS (
			SELECT 1
			FROM letter_recipients lr
			WHERE lr.letter_id = l.id
			  AND `+publishedRecipientAccessSQL("$2")+`
		) OR EXISTS (
			SELECT 1
			FROM dispositions d
			LEFT JOIN disposition_recipients dr ON dr.disposition_id = d.id
			JOIN user_positions up
			  ON up.position_id IN (d.from_position_id, dr.position_id)
			WHERE d.letter_id = l.id
			  AND l.classification <> 'rahasia'
			  AND up.user_id = $2
			  AND current_date >= up.valid_from
				  AND (up.valid_to IS NULL OR current_date < up.valid_to)
				) OR `+auditLetterAccessSQL("$2", "l")+`
			)
		)`, letterID, userID).Scan(&allowed)
	return allowed, err
}

func (h *Handler) loadLetterRecipients(c *gin.Context, letterID string) ([]DraftRecipient, bool) {
	rows, err := h.DB.Query(c.Request.Context(), `
		SELECT lr.recipient_type,
		       CASE WHEN lr.position_id IS NOT NULL THEN 'position' ELSE 'org_unit' END AS target_type,
		       COALESCE(lr.position_id::text, lr.org_unit_id::text) AS target_id,
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat penerima surat"})
		return nil, false
	}
	defer rows.Close()

	recipients := []DraftRecipient{}
	for rows.Next() {
		var recipient DraftRecipient
		if err := rows.Scan(&recipient.Type, &recipient.TargetType, &recipient.TargetID, &recipient.Label); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca penerima surat"})
			return nil, false
		}
		recipients = append(recipients, recipient)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar penerima surat"})
		return nil, false
	}
	return recipients, true
}

func (h *Handler) loadLetterApprovalSteps(c *gin.Context, letterID string) ([]LetterApprovalStep, bool) {
	rows, err := h.DB.Query(c.Request.Context(), `
		SELECT s.id::text, s.approval_cycle, s.step_order, s.flow_group, s.status,
		       p.title, p.position_type, s.sla_deadline, s.decided_at
		FROM approval_steps s
		JOIN positions p ON p.id = s.approver_position_id
		WHERE s.letter_id = $1
		ORDER BY s.approval_cycle, s.step_order`, letterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat timeline approval"})
		return nil, false
	}
	defer rows.Close()

	steps := []LetterApprovalStep{}
	for rows.Next() {
		var step LetterApprovalStep
		if err := rows.Scan(
			&step.ID,
			&step.ApprovalCycle,
			&step.StepOrder,
			&step.FlowGroup,
			&step.Status,
			&step.PositionTitle,
			&step.PositionType,
			&step.SLADeadline,
			&step.DecidedAt,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca timeline approval"})
			return nil, false
		}
		steps = append(steps, step)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar timeline approval"})
		return nil, false
	}
	return steps, true
}

func (h *Handler) loadLetterApprovalActions(c *gin.Context, letterID string) ([]LetterApprovalAction, bool) {
	rows, err := h.DB.Query(c.Request.Context(), `
		SELECT aa.id::text, aa.approval_step_id::text, aa.action,
		       u.full_name, aa.note, aa.device_info, aa.acted_at, p.title,
		       aa.signature_image_key IS NOT NULL
		FROM approval_actions aa
		JOIN approval_steps s ON s.id = aa.approval_step_id
		JOIN users u ON u.id = aa.acted_by_user_id
		JOIN positions p ON p.id = s.approver_position_id
		WHERE s.letter_id = $1
		ORDER BY aa.acted_at`, letterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat aksi approval"})
		return nil, false
	}
	defer rows.Close()

	actions := []LetterApprovalAction{}
	for rows.Next() {
		var action LetterApprovalAction
		if err := rows.Scan(
			&action.ID,
			&action.StepID,
			&action.Action,
			&action.ActorName,
			&action.Note,
			&action.DeviceInfo,
			&action.CreatedAt,
			&action.PositionTitle,
			&action.SignaturePresent,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca aksi approval"})
			return nil, false
		}
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar aksi approval"})
		return nil, false
	}
	return actions, true
}
