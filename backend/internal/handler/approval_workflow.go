package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

type draftSubmitSnapshot struct {
	ID                string
	LetterTypeID      string
	CreatorPositionID string
	Status            string
	BodyPlain         string
}

type approvalRoute struct {
	SLAHours int
	Steps    []approvalRouteStep
}

type approvalRouteStep struct {
	StepOrder    int    `json:"step_order"`
	FlowGroup    int    `json:"flow_group"`
	PositionID   string `json:"position_id"`
	PositionType string `json:"position_type"`
	Title        string `json:"title"`
}

type approvalPosition struct {
	ID           string
	Title        string
	PositionType string
	ReportsTo    *string
}

type approvalActionRequest struct {
	Action         string `json:"action"`
	Note           string `json:"note"`
	ClientActionID string `json:"client_action_id"`
	DeviceInfo     string `json:"device_info"`
}

func lockDraftForSubmit(ctx context.Context, tx pgx.Tx, letterID string, userID string) (draftSubmitSnapshot, error) {
	var draft draftSubmitSnapshot
	err := tx.QueryRow(ctx, `
		SELECT l.id::text, l.letter_type_id::text, l.creator_position_id::text,
		       l.status, COALESCE(v.body_plain, '')
		FROM letters l
		LEFT JOIN LATERAL (
			SELECT body_plain
			FROM letter_versions
			WHERE letter_id = l.id
			ORDER BY version DESC
			LIMIT 1
		) v ON true
		WHERE l.id = $1 AND l.creator_user_id = $2
		FOR UPDATE OF l`, letterID, userID).Scan(
		&draft.ID,
		&draft.LetterTypeID,
		&draft.CreatorPositionID,
		&draft.Status,
		&draft.BodyPlain,
	)
	return draft, err
}

func loadDraftRecipientRequests(ctx context.Context, tx pgx.Tx, letterID string) ([]draftRecipientRequest, error) {
	rows, err := tx.Query(ctx, `
		SELECT recipient_type,
		       CASE WHEN position_id IS NOT NULL THEN 'position' ELSE 'org_unit' END,
		       COALESCE(position_id::text, org_unit_id::text)
		FROM letter_recipients
		WHERE letter_id = $1
		ORDER BY recipient_type`, letterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	recipients := []draftRecipientRequest{}
	for rows.Next() {
		var recipient draftRecipientRequest
		if err := rows.Scan(&recipient.Type, &recipient.TargetType, &recipient.TargetID); err != nil {
			return nil, err
		}
		recipients = append(recipients, recipient)
	}
	return recipients, rows.Err()
}

func (h *Handler) resolveApprovalRoute(ctx context.Context, tx pgx.Tx, letterTypeID string, creatorPositionID string) (approvalRoute, error) {
	var (
		finalLevel sql.NullString
		flowMode   sql.NullString
		slaHours   int
	)
	creator, err := loadApprovalPosition(ctx, tx, creatorPositionID)
	if err != nil {
		return approvalRoute{}, errors.New("jabatan pembuat tidak ditemukan")
	}

	err = tx.QueryRow(ctx, `
		SELECT am.final_level, am.flow_mode, lt.default_sla_hours
		FROM letter_types lt
		LEFT JOIN LATERAL (
			SELECT final_level, flow_mode
			FROM approval_matrices
			WHERE letter_type_id = lt.id
			  AND is_active
			  AND (originator_level IS NULL OR originator_level = $2)
			ORDER BY CASE WHEN originator_level = $2 THEN 0 ELSE 1 END
			LIMIT 1
		) am ON true
		WHERE lt.id = $1 AND lt.is_active`, letterTypeID, creator.PositionType).Scan(&finalLevel, &flowMode, &slaHours)
	if errors.Is(err, pgx.ErrNoRows) {
		return approvalRoute{}, errors.New("jenis surat tidak aktif")
	}
	if err != nil {
		return approvalRoute{}, errors.New("gagal membaca matrix approval")
	}
	finalLevelValue := finalLevel.String
	if !finalLevel.Valid || finalLevelValue == "" {
		finalLevelValue = "director"
	}
	flowModeValue := flowMode.String
	if !flowMode.Valid || flowModeValue == "" {
		flowModeValue = "serial"
	}
	if slaHours <= 0 {
		slaHours = 24
	}

	steps := []approvalRouteStep{}
	nextID := creator.ReportsTo
	for nextID != nil {
		position, err := loadApprovalPosition(ctx, tx, *nextID)
		if errors.Is(err, pgx.ErrNoRows) {
			break
		}
		if err != nil {
			return approvalRoute{}, errors.New("gagal membaca rantai atasan")
		}

		stepOrder := len(steps) + 1
		steps = append(steps, approvalRouteStep{
			StepOrder:    stepOrder,
			FlowGroup:    stepOrder,
			PositionID:   position.ID,
			PositionType: position.PositionType,
			Title:        position.Title,
		})
		if position.PositionType == finalLevelValue {
			break
		}
		nextID = position.ReportsTo
	}

	if len(steps) == 0 {
		return approvalRoute{}, errors.New("rantai approval belum tersedia untuk jabatan pembuat ini")
	}
	if flowModeValue == "parallel" {
		for i := range steps {
			steps[i].FlowGroup = 1
		}
	}
	return approvalRoute{SLAHours: slaHours, Steps: steps}, nil
}

func loadApprovalPosition(ctx context.Context, tx pgx.Tx, positionID string) (approvalPosition, error) {
	var position approvalPosition
	err := tx.QueryRow(ctx, `
		SELECT id::text, title, position_type, reports_to::text
		FROM positions
		WHERE id = $1 AND is_active`, positionID).Scan(
		&position.ID,
		&position.Title,
		&position.PositionType,
		&position.ReportsTo,
	)
	return position, err
}

func (h *Handler) uniqueQRToken(ctx context.Context, tx pgx.Tx) (string, error) {
	for i := 0; i < 5; i++ {
		token := randomHex(24)
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM letters WHERE qr_token = $1)`, token).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return token, nil
		}
	}
	return "", errors.New("gagal membuat token unik")
}

func (h *Handler) ListApprovalInbox(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	rows, err := h.DB.Query(c.Request.Context(), `
		SELECT s.id::text, s.letter_id::text, s.step_order, s.status,
		       l.subject, l.priority, l.classification, lt.code, co.code,
		       p.title, l.updated_at
		FROM approval_steps s
		JOIN letters l ON l.id = s.letter_id
		JOIN letter_types lt ON lt.id = l.letter_type_id
		JOIN companies co ON co.id = l.company_id
		JOIN positions p ON p.id = s.approver_position_id
		JOIN user_positions up ON up.position_id = s.approver_position_id
		WHERE up.user_id = $1
		  AND current_date >= up.valid_from
		  AND (up.valid_to IS NULL OR current_date <= up.valid_to)
		  AND s.status = 'waiting'
		  AND l.status = 'in_approval'
		ORDER BY l.priority DESC, l.updated_at ASC`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat inbox approval"})
		return
	}
	defer rows.Close()

	type approvalInboxItem struct {
		StepID         string    `json:"step_id"`
		LetterID       string    `json:"letter_id"`
		StepOrder      int       `json:"step_order"`
		Status         string    `json:"status"`
		Subject        string    `json:"subject"`
		Priority       string    `json:"priority"`
		Classification string    `json:"classification"`
		LetterTypeCode string    `json:"letter_type_code"`
		CompanyCode    string    `json:"company_code"`
		PositionTitle  string    `json:"position_title"`
		UpdatedAt      time.Time `json:"updated_at"`
	}
	items := []approvalInboxItem{}
	for rows.Next() {
		var item approvalInboxItem
		if err := rows.Scan(
			&item.StepID,
			&item.LetterID,
			&item.StepOrder,
			&item.Status,
			&item.Subject,
			&item.Priority,
			&item.Classification,
			&item.LetterTypeCode,
			&item.CompanyCode,
			&item.PositionTitle,
			&item.UpdatedAt,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca inbox approval"})
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar approval"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"approvals": items})
}

func (h *Handler) ActApprovalStep(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	stepID := c.Param("id")
	var req approvalActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data aksi approval tidak valid: " + err.Error()})
		return
	}
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	req.Note = strings.TrimSpace(req.Note)
	if req.Action != "approve" && req.Action != "reject" && req.Action != "request_revision" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "aksi approval tidak valid"})
		return
	}
	if req.Action != "approve" && req.Note == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "alasan wajib diisi untuk tolak atau minta revisi"})
		return
	}

	ctx := c.Request.Context()
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi"})
		return
	}
	defer tx.Rollback(ctx)

	var letterID string
	var stepOrder int
	err = tx.QueryRow(ctx, `
		SELECT s.letter_id::text, s.step_order
		FROM approval_steps s
		JOIN user_positions up ON up.position_id = s.approver_position_id
		JOIN letters l ON l.id = s.letter_id
		WHERE s.id = $1
		  AND up.user_id = $2
		  AND current_date >= up.valid_from
		  AND (up.valid_to IS NULL OR current_date <= up.valid_to)
		  AND s.status = 'waiting'
		  AND l.status = 'in_approval'
		FOR UPDATE OF s`, stepID, userID).Scan(&letterID, &stepOrder)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "approval step tidak ditemukan atau bukan giliran pengguna ini"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca approval step"})
		return
	}

	clientActionID := strings.TrimSpace(req.ClientActionID)
	var clientActionArg any
	if clientActionID != "" {
		clientActionArg = clientActionID
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO approval_actions
			(approval_step_id, action, acted_by_user_id, note, client_action_id, device_info, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		stepID, req.Action, userID, nullableString(req.Note), clientActionArg, nullableString(req.DeviceInfo), c.ClientIP()); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "aksi approval gagal dicatat atau duplikat"})
		return
	}

	nextStatus := "approved"
	if req.Action == "reject" {
		nextStatus = "rejected"
	}
	if _, err := tx.Exec(ctx, `UPDATE approval_steps SET status = $2 WHERE id = $1`, stepID, nextStatus); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memperbarui approval step"})
		return
	}

	letterStatus := "in_approval"
	if req.Action == "approve" {
		nextStepID, nextOrder, err := promoteNextApprovalStep(ctx, tx, letterID, stepOrder)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memindahkan step approval"})
			return
		}
		if nextStepID == "" {
			letterStatus = "approved"
			if _, err := tx.Exec(ctx, `
				UPDATE letters
				SET status = 'approved', current_step_order = NULL, updated_at = now()
				WHERE id = $1`, letterID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyelesaikan approval"})
				return
			}
		} else if _, err := tx.Exec(ctx, `
			UPDATE letters
			SET current_step_order = $2, updated_at = now()
			WHERE id = $1`, letterID, nextOrder); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memperbarui surat"})
			return
		}
	} else {
		if _, err := tx.Exec(ctx, `
			UPDATE approval_steps
			SET status = 'skipped'
			WHERE letter_id = $1 AND status IN ('pending','waiting')`, letterID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menutup sisa approval"})
			return
		}
		letterStatus = "cancelled"
		if req.Action == "request_revision" {
			letterStatus = "revision"
		}
		if _, err := tx.Exec(ctx, `
			UPDATE letters
			SET status = $2, current_step_order = NULL, updated_at = now()
			WHERE id = $1`, letterID, letterStatus); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memperbarui status surat"})
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan aksi approval"})
		return
	}

	h.audit(ctx, "letter", &letterID, "approval_"+req.Action, &userID, map[string]any{
		"step_id": stepID,
		"status":  letterStatus,
	}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"letter_id": letterID, "status": letterStatus})
}

func promoteNextApprovalStep(ctx context.Context, tx pgx.Tx, letterID string, currentStepOrder int) (string, int, error) {
	var nextStepID string
	var nextOrder int
	err := tx.QueryRow(ctx, `
		SELECT id::text, step_order
		FROM approval_steps
		WHERE letter_id = $1
		  AND step_order > $2
		  AND status = 'pending'
		ORDER BY step_order
		LIMIT 1`, letterID, currentStepOrder).Scan(&nextStepID, &nextOrder)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, err
	}
	_, err = tx.Exec(ctx, `UPDATE approval_steps SET status = 'waiting' WHERE id = $1`, nextStepID)
	if err != nil {
		return "", 0, err
	}
	return nextStepID, nextOrder, nil
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
