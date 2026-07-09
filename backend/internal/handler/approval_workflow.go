package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"

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

func insertApprovalRoute(ctx context.Context, tx pgx.Tx, letterID string, route approvalRoute) (int, error) {
	var approvalCycle int
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(approval_cycle), 0) + 1
		FROM approval_steps
		WHERE letter_id = $1`, letterID).Scan(&approvalCycle)
	if err != nil {
		return 0, fmt.Errorf("determine approval cycle: %w", err)
	}

	for _, step := range route.Steps {
		status := "pending"
		if step.StepOrder == 1 {
			status = "waiting"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO approval_steps
				(letter_id, approval_cycle, step_order, approver_position_id, flow_group, status, sla_deadline)
			VALUES ($1, $2, $3, $4, $5, $6, now() + make_interval(hours => $7::int))`,
			letterID,
			approvalCycle,
			step.StepOrder,
			step.PositionID,
			step.FlowGroup,
			status,
			route.SLAHours,
		); err != nil {
			return 0, fmt.Errorf("insert approval step %d: %w", step.StepOrder, err)
		}
	}

	return approvalCycle, nil
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

		if position.PositionType == "sub_dept_head" {
			hasActiveHolder, err := approvalPositionHasActiveHolder(ctx, tx, position.ID)
			if err != nil {
				return approvalRoute{}, errors.New("gagal memvalidasi pemegang Sub Department Head")
			}
			if !hasActiveHolder {
				nextID = position.ReportsTo
				continue
			}
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

func approvalPositionHasActiveHolder(ctx context.Context, tx pgx.Tx, positionID string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM user_positions assignment
			JOIN users holder ON holder.id = assignment.user_id
			WHERE assignment.position_id = $1
			  AND current_date >= assignment.valid_from
			  AND (assignment.valid_to IS NULL OR current_date < assignment.valid_to)
			  AND holder.status = 'active'
		)`, positionID).Scan(&exists)
	return exists, err
}

type numberingContext struct {
	CompanyID    string
	CompanyCode  string
	LetterTypeID string
	LetterType   string
	OrgUnitID    string
	OrgUnitCode  string
	CreatedAt    time.Time
}

type numberingFormatConfig struct {
	ID          string
	Pattern     string
	ResetPeriod string
}

func finalizeApprovedLetter(ctx context.Context, tx pgx.Tx, letterID string) (string, error) {
	numbering, err := loadNumberingContext(ctx, tx, letterID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errors.New("surat tidak ditemukan untuk finalisasi")
	}
	if err != nil {
		return "", errors.New("gagal membaca konteks penomoran surat")
	}

	format, err := ensureNumberingFormat(ctx, tx, numbering)
	if err != nil {
		return "", err
	}

	seq, err := nextNumberingSequence(ctx, tx, format.ID, numberingScopeKey(numbering, format.ResetPeriod))
	if err != nil {
		return "", err
	}
	return renderLetterNumber(format.Pattern, numbering, seq), nil
}

func loadNumberingContext(ctx context.Context, tx pgx.Tx, letterID string) (numberingContext, error) {
	var numbering numberingContext
	err := tx.QueryRow(ctx, `
		SELECT l.company_id::text, co.code,
		       l.letter_type_id::text, lt.code,
		       ou.id::text, ou.code,
		       l.created_at
		FROM letters l
		JOIN companies co ON co.id = l.company_id
		JOIN letter_types lt ON lt.id = l.letter_type_id
		JOIN positions p ON p.id = l.creator_position_id
		JOIN org_units ou ON ou.id = p.org_unit_id
		WHERE l.id = $1
		FOR UPDATE OF l`, letterID).Scan(
		&numbering.CompanyID,
		&numbering.CompanyCode,
		&numbering.LetterTypeID,
		&numbering.LetterType,
		&numbering.OrgUnitID,
		&numbering.OrgUnitCode,
		&numbering.CreatedAt,
	)
	return numbering, err
}

func ensureNumberingFormat(ctx context.Context, tx pgx.Tx, numbering numberingContext) (numberingFormatConfig, error) {
	var format numberingFormatConfig
	err := tx.QueryRow(ctx, `
		SELECT id::text, pattern, reset_period
		FROM numbering_formats
		WHERE company_id = $1
		  AND is_active
		  AND (letter_type_id IS NULL OR letter_type_id = $2)
		  AND (org_unit_id IS NULL OR org_unit_id = $3)
		ORDER BY
		  CASE WHEN letter_type_id = $2 THEN 0 ELSE 1 END,
		  CASE WHEN org_unit_id = $3 THEN 0 ELSE 1 END,
		  id
		LIMIT 1`, numbering.CompanyID, numbering.LetterTypeID, numbering.OrgUnitID).Scan(
		&format.ID,
		&format.Pattern,
		&format.ResetPeriod,
	)
	if err == nil {
		return format, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return numberingFormatConfig{}, errors.New("gagal membaca format penomoran")
	}

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "numbering_format:"+numbering.CompanyID); err != nil {
		return numberingFormatConfig{}, errors.New("gagal mengunci format penomoran")
	}

	err = tx.QueryRow(ctx, `
		SELECT id::text, pattern, reset_period
		FROM numbering_formats
		WHERE company_id = $1
		  AND letter_type_id IS NULL
		  AND org_unit_id IS NULL
		  AND is_active
		ORDER BY id
		LIMIT 1`, numbering.CompanyID).Scan(&format.ID, &format.Pattern, &format.ResetPeriod)
	if err == nil {
		return format, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return numberingFormatConfig{}, errors.New("gagal membaca format penomoran default")
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO numbering_formats (company_id, pattern, reset_period)
		VALUES ($1, '{seq:4}/{type}/{unit}/{roman_month}/{year}', 'yearly')
		RETURNING id::text, pattern, reset_period`, numbering.CompanyID).Scan(
		&format.ID,
		&format.Pattern,
		&format.ResetPeriod,
	)
	if err != nil {
		return numberingFormatConfig{}, errors.New("gagal membuat format penomoran default")
	}
	return format, nil
}

func numberingScopeKey(numbering numberingContext, resetPeriod string) string {
	if resetPeriod == "monthly" {
		return strings.Join([]string{numbering.CompanyCode, numbering.OrgUnitCode, numbering.LetterType, numbering.CreatedAt.Format("2006-01")}, "|")
	}
	return strings.Join([]string{numbering.CompanyCode, numbering.OrgUnitCode, numbering.LetterType, numbering.CreatedAt.Format("2006")}, "|")
}

func nextNumberingSequence(ctx context.Context, tx pgx.Tx, formatID string, scopeKey string) (int, error) {
	if _, err := tx.Exec(ctx, `
		INSERT INTO numbering_counters (format_id, scope_key, current_value)
		VALUES ($1, $2, 0)
		ON CONFLICT (format_id, scope_key) DO NOTHING`, formatID, scopeKey); err != nil {
		return 0, errors.New("gagal menyiapkan counter penomoran")
	}

	var current int
	if err := tx.QueryRow(ctx, `
		SELECT current_value
		FROM numbering_counters
		WHERE format_id = $1 AND scope_key = $2
		FOR UPDATE`, formatID, scopeKey).Scan(&current); err != nil {
		return 0, errors.New("gagal mengunci counter penomoran")
	}

	next := current + 1
	if _, err := tx.Exec(ctx, `
		UPDATE numbering_counters
		SET current_value = $3
		WHERE format_id = $1 AND scope_key = $2`, formatID, scopeKey, next); err != nil {
		return 0, errors.New("gagal memperbarui counter penomoran")
	}
	return next, nil
}

func renderLetterNumber(pattern string, numbering numberingContext, seq int) string {
	replacements := map[string]string{
		"company":     numbering.CompanyCode,
		"type":        numbering.LetterType,
		"unit":        numbering.OrgUnitCode,
		"year":        numbering.CreatedAt.Format("2006"),
		"month":       numbering.CreatedAt.Format("01"),
		"roman_month": romanMonth(int(numbering.CreatedAt.Month())),
	}

	result := pattern
	for key, value := range replacements {
		result = strings.ReplaceAll(result, "{"+key+"}", value)
	}

	seqPattern := regexp.MustCompile(`\{seq(?::([0-9]+))?\}`)
	return seqPattern.ReplaceAllStringFunc(result, func(match string) string {
		parts := seqPattern.FindStringSubmatch(match)
		width := 0
		if len(parts) > 1 && parts[1] != "" {
			if parsed, err := strconv.Atoi(parts[1]); err == nil {
				width = parsed
			}
		}
		if width > 0 {
			return fmt.Sprintf("%0*d", width, seq)
		}
		return strconv.Itoa(seq)
	})
}

func romanMonth(month int) string {
	months := []string{"", "I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX", "X", "XI", "XII"}
	if month < 1 || month > 12 {
		return ""
	}
	return months[month]
}

func validateApprovalRouteHasActiveHolders(ctx context.Context, tx pgx.Tx, route approvalRoute) error {
	for _, step := range route.Steps {
		var exists bool
		err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM user_positions up
				JOIN users u ON u.id = up.user_id
				WHERE up.position_id = $1
				  AND current_date >= up.valid_from
				  AND (up.valid_to IS NULL OR current_date < up.valid_to)
				  AND u.status = 'active'
			)`, step.PositionID).Scan(&exists)
		if err != nil {
			return errors.New("gagal memvalidasi pemegang jabatan approver")
		}
		if !exists {
			return errors.New("jabatan approver belum memiliki pemegang aktif: " + step.Title)
		}
	}
	return nil
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
	ctx := c.Request.Context()

	page, pageSize, offset, ok := parsePagination(c.Query("page"), c.Query("page_size"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page atau page_size tidak valid"})
		return
	}

	inboxSQL := `
		FROM approval_steps s
		JOIN letters l ON l.id = s.letter_id
		JOIN letter_types lt ON lt.id = l.letter_type_id
		JOIN companies co ON co.id = l.company_id
		JOIN positions p ON p.id = s.approver_position_id
		JOIN users u ON u.id = l.creator_user_id
		JOIN positions cp ON cp.id = l.creator_position_id
		JOIN user_positions up ON up.position_id = s.approver_position_id
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
		WHERE up.user_id = $1
		  AND current_date >= up.valid_from
		  AND (up.valid_to IS NULL OR current_date < up.valid_to)
		  AND s.status = 'waiting'
		  AND l.status = 'in_approval'`

	var total int64
	if err := h.DB.QueryRow(ctx, `SELECT count(*) `+inboxSQL, userID).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menghitung inbox approval"})
		return
	}

	rows, err := h.DB.Query(ctx, `
		SELECT s.id::text, s.letter_id::text, s.step_order, s.status,
		       l.subject, l.priority, l.classification, lt.code, co.code,
		       p.title, u.full_name, cp.title, COALESCE(v.body_plain, ''),
		       COALESCE(a.attachment_count, 0), l.updated_at `+
		inboxSQL+`
		ORDER BY l.priority DESC, l.updated_at ASC
		LIMIT $2 OFFSET $3`, userID, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat inbox approval"})
		return
	}
	defer rows.Close()

	type approvalInboxItem struct {
		StepID          string    `json:"step_id"`
		LetterID        string    `json:"letter_id"`
		StepOrder       int       `json:"step_order"`
		Status          string    `json:"status"`
		Subject         string    `json:"subject"`
		Priority        string    `json:"priority"`
		Classification  string    `json:"classification"`
		LetterTypeCode  string    `json:"letter_type_code"`
		CompanyCode     string    `json:"company_code"`
		PositionTitle   string    `json:"position_title"`
		CreatorName     string    `json:"creator_name"`
		CreatorPosition string    `json:"creator_position"`
		BodyPlain       string    `json:"body_plain"`
		AttachmentCount int       `json:"attachment_count"`
		UpdatedAt       time.Time `json:"updated_at"`
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
			&item.CreatorName,
			&item.CreatorPosition,
			&item.BodyPlain,
			&item.AttachmentCount,
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
	c.JSON(http.StatusOK, gin.H{"data": items, "meta": newPageMeta(page, pageSize, total)})
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
		  AND (up.valid_to IS NULL OR current_date < up.valid_to)
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
	if _, err := tx.Exec(ctx, `UPDATE approval_steps SET status = $2, decided_at = now() WHERE id = $1`, stepID, nextStatus); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memperbarui approval step"})
		return
	}

	letterStatus := "in_approval"
	var finalPDFKey string
	var pendingEmails []notificationEmail
	if req.Action == "approve" {
		nextStepID, nextOrder, err := promoteNextApprovalStep(ctx, tx, letterID, stepOrder)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memindahkan step approval"})
			return
		}
		if nextStepID == "" {
			letterNumber, err := finalizeApprovedLetter(ctx, tx, letterID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			publishedAt := time.Now()
			finalPDFKey, err = h.renderAndStoreFinalPDF(ctx, tx, letterID, letterNumber, publishedAt)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			letterStatus = "published"
			if _, err := tx.Exec(ctx, `
				UPDATE letters
				SET status = 'published',
				    letter_number = $2,
				    final_pdf_key = $3,
				    current_step_order = NULL,
				    published_at = $4,
				    updated_at = now()
				WHERE id = $1`, letterID, letterNumber, finalPDFKey, publishedAt); err != nil {
				_ = h.Minio.RemoveObject(ctx, h.Bucket, finalPDFKey, minio.RemoveObjectOptions{})
				c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyelesaikan approval"})
				return
			}
			incomingEmails, err := distributePublishedLetter(ctx, tx, letterID, publishedAt)
			if err != nil {
				_ = h.Minio.RemoveObject(ctx, h.Bucket, finalPDFKey, minio.RemoveObjectOptions{})
				c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mendistribusikan surat"})
				return
			}
			resultEmails, err := notifyApprovalResult(ctx, tx, letterID, letterStatus)
			if err != nil {
				_ = h.Minio.RemoveObject(ctx, h.Bucket, finalPDFKey, minio.RemoveObjectOptions{})
				c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengirim notifikasi hasil approval"})
				return
			}
			pendingEmails = append(incomingEmails, resultEmails...)
		} else {
			if _, err := tx.Exec(ctx, `
				UPDATE letters
				SET current_step_order = $2, updated_at = now()
				WHERE id = $1`, letterID, nextOrder); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memperbarui surat"})
				return
			}
			pendingEmails, err = notifyWaitingApprovers(ctx, tx, letterID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengirim notifikasi approval"})
				return
			}
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
		pendingEmails, err = notifyApprovalResult(ctx, tx, letterID, letterStatus)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengirim notifikasi hasil approval"})
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
	h.sendNotificationEmails(pendingEmails)
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
