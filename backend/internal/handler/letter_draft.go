package handler

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

type DraftLetter struct {
	ID                   string           `json:"id"`
	CompanyID            string           `json:"company_id"`
	CompanyCode          string           `json:"company_code"`
	CompanyName          string           `json:"company_name"`
	LetterTypeID         string           `json:"letter_type_id"`
	LetterTypeCode       string           `json:"letter_type_code"`
	LetterTypeName       string           `json:"letter_type_name"`
	LetterNumber         *string          `json:"letter_number"`
	Subject              string           `json:"subject"`
	Classification       string           `json:"classification"`
	Priority             string           `json:"priority"`
	Status               string           `json:"status"`
	CreatorPositionID    string           `json:"creator_position_id"`
	CreatorPositionTitle string           `json:"creator_position_title"`
	OnBehalfOfPositionID *string          `json:"on_behalf_of_position_id"`
	OnBehalfOfTitle      *string          `json:"on_behalf_of_title"`
	Version              int              `json:"version"`
	BodyHTML             string           `json:"body_html"`
	BodyPlain            string           `json:"body_plain"`
	Recipients           []DraftRecipient `json:"recipients"`
	CreatedAt            time.Time        `json:"created_at"`
	UpdatedAt            time.Time        `json:"updated_at"`
}

type DraftRecipient struct {
	Type       string `json:"type"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Label      string `json:"label"`
}

type draftLetterRequest struct {
	CompanyID            string                  `json:"company_id"`
	LetterTypeID         string                  `json:"letter_type_id"`
	CreatorPositionID    string                  `json:"creator_position_id"`
	OnBehalfOfPositionID *string                 `json:"on_behalf_of_position_id"`
	Subject              string                  `json:"subject"`
	Classification       string                  `json:"classification"`
	Priority             string                  `json:"priority"`
	BodyHTML             string                  `json:"body_html"`
	Recipients           []draftRecipientRequest `json:"recipients"`
}

type draftRecipientRequest struct {
	Type       string `json:"type"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
}

func (h *Handler) ListDraftLetters(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	rows, err := h.DB.Query(c.Request.Context(), draftLetterSelect(`
		WHERE l.creator_user_id = $1
		  AND l.status IN ('draft', 'revision')
		ORDER BY l.updated_at DESC`), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat draft surat"})
		return
	}
	defer rows.Close()

	letters, ok := scanDraftLetters(c, rows)
	if !ok {
		return
	}
	if !h.attachDraftRecipients(c, letters) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"letters": letters})
}

func (h *Handler) ListMyLetters(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	ctx := c.Request.Context()

	page, pageSize, offset, ok := parsePagination(c.Query("page"), c.Query("page_size"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page atau page_size tidak valid"})
		return
	}

	var total int64
	if err := h.DB.QueryRow(ctx, `SELECT count(*) FROM letters l WHERE l.creator_user_id = $1`, userID).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menghitung surat saya"})
		return
	}

	rows, err := h.DB.Query(ctx, draftLetterSelect(`
		WHERE l.creator_user_id = $1
		ORDER BY l.updated_at DESC
		LIMIT $2 OFFSET $3`), userID, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat surat saya"})
		return
	}
	defer rows.Close()

	letters, scanOK := scanDraftLetters(c, rows)
	if !scanOK {
		return
	}
	if !h.attachDraftRecipients(c, letters) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": letters, "meta": newPageMeta(page, pageSize, total)})
}

func (h *Handler) GetDraftLetter(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	id := c.Param("id")

	rows, err := h.DB.Query(c.Request.Context(), draftLetterSelect(`
		WHERE l.id = $1
		  AND l.creator_user_id = $2
		  AND l.status IN ('draft', 'revision')`), id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat draft surat"})
		return
	}
	defer rows.Close()

	letters, ok := scanDraftLetters(c, rows)
	if !ok {
		return
	}
	if len(letters) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "draft surat tidak ditemukan"})
		return
	}
	if !h.attachDraftRecipients(c, letters) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"letter": letters[0]})
}

func (h *Handler) CreateDraftLetter(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	var req draftLetterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data draft tidak valid: " + err.Error()})
		return
	}
	if err := normalizeDraftLetterRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	if ok, err := h.userCanUsePosition(ctx, userID, req.CreatorPositionID); err != nil || !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jabatan pembuat tidak valid untuk pengguna ini"})
		return
	}
	if req.Classification == "" {
		classification, err := h.defaultLetterClassification(ctx, req.LetterTypeID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "jenis surat tidak valid"})
			return
		}
		req.Classification = classification
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi"})
		return
	}
	defer tx.Rollback(ctx)

	if err := validateDraftOnBehalf(ctx, tx, req.CreatorPositionID, req.OnBehalfOfPositionID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO letters
			(company_id, letter_type_id, subject, classification, priority,
			 creator_user_id, creator_position_id, on_behalf_of_position_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id::text`,
		req.CompanyID,
		req.LetterTypeID,
		req.Subject,
		req.Classification,
		req.Priority,
		userID,
		req.CreatorPositionID,
		req.OnBehalfOfPositionID,
	).Scan(&id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gagal membuat draft (perusahaan atau jenis surat tidak valid)"})
		return
	}
	if err := h.validateDraftRecipientTargets(ctx, tx, req.CreatorPositionID, req.Recipients); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := insertDraftRecipients(ctx, tx, id, req.Recipients); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan penerima draft"})
		return
	}

	bodyPlain := htmlToPlain(req.BodyHTML)
	if _, err := tx.Exec(ctx, `
		INSERT INTO letter_versions (letter_id, version, body_html, body_plain, edited_by)
		VALUES ($1, 1, $2, $3, $4)`,
		id, req.BodyHTML, bodyPlain, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan isi draft"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan draft"})
		return
	}

	h.audit(ctx, "letter", &id, "create_draft", &userID, map[string]any{
		"subject":    req.Subject,
		"recipients": len(req.Recipients),
		"status":     "draft",
	}, c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"id": id, "version": 1})
}

func (h *Handler) UpdateDraftLetter(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	id := c.Param("id")

	var req draftLetterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data draft tidak valid: " + err.Error()})
		return
	}
	if err := normalizeDraftLetterRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	if ok, err := h.userCanUsePosition(ctx, userID, req.CreatorPositionID); err != nil || !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jabatan pembuat tidak valid untuk pengguna ini"})
		return
	}
	if req.Classification == "" {
		classification, err := h.defaultLetterClassification(ctx, req.LetterTypeID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "jenis surat tidak valid"})
			return
		}
		req.Classification = classification
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi"})
		return
	}
	defer tx.Rollback(ctx)

	var currentStatus string
	err = tx.QueryRow(ctx, `
		SELECT status
		FROM letters
		WHERE id = $1 AND creator_user_id = $2`, id, userID).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "draft surat tidak ditemukan"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca draft surat"})
		return
	}
	if currentStatus != "draft" && currentStatus != "revision" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "surat sudah tidak dapat diedit sebagai draft"})
		return
	}
	if err := h.validateDraftRecipientTargets(ctx, tx, req.CreatorPositionID, req.Recipients); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateDraftOnBehalf(ctx, tx, req.CreatorPositionID, req.OnBehalfOfPositionID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var nextVersion int
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM letter_versions
		WHERE letter_id = $1`, id).Scan(&nextVersion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menentukan versi draft"})
		return
	}

	tag, err := tx.Exec(ctx, `
		UPDATE letters
		SET company_id = $2, letter_type_id = $3, subject = $4,
		    classification = $5, priority = $6, creator_position_id = $7,
		    on_behalf_of_position_id = $8,
		    updated_at = now()
		WHERE id = $1 AND creator_user_id = $9`,
		id,
		req.CompanyID,
		req.LetterTypeID,
		req.Subject,
		req.Classification,
		req.Priority,
		req.CreatorPositionID,
		req.OnBehalfOfPositionID,
		userID,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gagal memperbarui draft (perusahaan atau jenis surat tidak valid)"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "draft surat tidak ditemukan"})
		return
	}
	if _, err := tx.Exec(ctx, `DELETE FROM letter_recipients WHERE letter_id = $1`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menghapus penerima lama"})
		return
	}
	if err := insertDraftRecipients(ctx, tx, id, req.Recipients); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan penerima draft"})
		return
	}

	bodyPlain := htmlToPlain(req.BodyHTML)
	if _, err := tx.Exec(ctx, `
		INSERT INTO letter_versions (letter_id, version, body_html, body_plain, edited_by)
		VALUES ($1, $2, $3, $4, $5)`,
		id, nextVersion, req.BodyHTML, bodyPlain, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan versi draft"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan draft"})
		return
	}

	h.audit(ctx, "letter", &id, "update_draft", &userID, map[string]any{
		"subject":    req.Subject,
		"recipients": len(req.Recipients),
		"version":    nextVersion,
	}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id, "version": nextVersion})
}

func normalizeDraftLetterRequest(req *draftLetterRequest) error {
	req.CompanyID = strings.TrimSpace(req.CompanyID)
	req.LetterTypeID = strings.TrimSpace(req.LetterTypeID)
	req.CreatorPositionID = strings.TrimSpace(req.CreatorPositionID)
	if req.OnBehalfOfPositionID != nil {
		value := strings.TrimSpace(*req.OnBehalfOfPositionID)
		if value == "" {
			req.OnBehalfOfPositionID = nil
		} else {
			req.OnBehalfOfPositionID = &value
		}
	}
	req.Subject = strings.TrimSpace(req.Subject)
	req.Classification = strings.ToLower(strings.TrimSpace(req.Classification))
	req.Priority = strings.ToLower(strings.TrimSpace(req.Priority))
	req.BodyHTML = strings.TrimSpace(req.BodyHTML)

	if req.CompanyID == "" {
		return errors.New("perusahaan wajib dipilih")
	}
	if req.LetterTypeID == "" {
		return errors.New("jenis surat wajib dipilih")
	}
	if req.CreatorPositionID == "" {
		return errors.New("jabatan pembuat wajib dipilih")
	}
	if req.Subject == "" {
		return errors.New("perihal wajib diisi")
	}
	if len(req.Subject) > 255 {
		return errors.New("perihal maksimal 255 karakter")
	}
	if req.Classification != "" && req.Classification != "biasa" && req.Classification != "terbatas" && req.Classification != "rahasia" {
		return errors.New("klasifikasi tidak valid")
	}
	if req.Priority == "" {
		req.Priority = "normal"
	}
	if req.Priority != "normal" && req.Priority != "urgent" {
		return errors.New("prioritas tidak valid")
	}
	if err := normalizeDraftRecipients(req.Recipients); err != nil {
		return err
	}
	req.BodyHTML = sanitizeLetterHTML(req.BodyHTML)
	if req.BodyHTML == "" || htmlToPlain(req.BodyHTML) == "" {
		return errors.New("isi surat wajib diisi")
	}
	return nil
}

func normalizeDraftRecipients(recipients []draftRecipientRequest) error {
	hasTo := false
	seen := map[string]bool{}
	for i := range recipients {
		recipients[i].Type = strings.ToLower(strings.TrimSpace(recipients[i].Type))
		recipients[i].TargetType = strings.ToLower(strings.TrimSpace(recipients[i].TargetType))
		recipients[i].TargetID = strings.TrimSpace(recipients[i].TargetID)

		if recipients[i].Type != "to" && recipients[i].Type != "cc" {
			return errors.New("tipe penerima harus to atau cc")
		}
		if recipients[i].TargetType != "position" && recipients[i].TargetType != "org_unit" {
			return errors.New("target penerima harus position atau org_unit")
		}
		if recipients[i].TargetID == "" {
			return errors.New("target penerima wajib dipilih")
		}
		if recipients[i].Type == "to" {
			hasTo = true
		}

		key := recipients[i].Type + "|" + recipients[i].TargetType + "|" + recipients[i].TargetID
		if seen[key] {
			return errors.New("penerima tidak boleh duplikat")
		}
		seen[key] = true
	}
	if !hasTo {
		return errors.New("minimal satu penerima tujuan wajib dipilih")
	}
	return nil
}

func validateDraftOnBehalf(ctx context.Context, tx pgx.Tx, creatorPositionID string, onBehalfOfPositionID *string) error {
	if onBehalfOfPositionID == nil {
		return nil
	}

	var creatorType string
	var reportsTo *string
	err := tx.QueryRow(ctx, `
		SELECT position_type, reports_to::text
		FROM positions
		WHERE id = $1 AND is_active`, creatorPositionID).Scan(&creatorType, &reportsTo)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("jabatan pembuat tidak ditemukan atau tidak aktif")
	}
	if err != nil {
		return errors.New("gagal memvalidasi jabatan pembuat")
	}
	if creatorType != "secretary" {
		return errors.New("atas nama hanya dapat digunakan oleh jabatan secretary")
	}
	if reportsTo == nil || *reportsTo != *onBehalfOfPositionID {
		return errors.New("jabatan atas nama harus atasan langsung Secretary")
	}

	var onBehalfType string
	err = tx.QueryRow(ctx, `
		SELECT position_type
		FROM positions
		WHERE id = $1 AND is_active`, *onBehalfOfPositionID).Scan(&onBehalfType)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("jabatan atas nama tidak ditemukan atau tidak aktif")
	}
	if err != nil {
		return errors.New("gagal memvalidasi jabatan atas nama")
	}
	if onBehalfType != "director" && onBehalfType != "gm" {
		return errors.New("jabatan atas nama harus Director atau GM")
	}
	return nil
}

type draftPositionScope struct {
	PositionType  string
	DirectorateID *string
}

type draftRecipientScope struct {
	TargetType    string
	DirectorateID *string
}

func (h *Handler) validateDraftRecipientTargets(ctx context.Context, tx pgx.Tx, creatorPositionID string, recipients []draftRecipientRequest) error {
	creatorScope, err := h.draftCreatorPositionScope(ctx, tx, creatorPositionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("jabatan pembuat tidak ditemukan atau tidak aktif")
	}
	if err != nil {
		return errors.New("gagal memvalidasi jabatan pembuat")
	}

	for _, recipient := range recipients {
		recipientScope, err := h.draftRecipientScope(ctx, tx, recipient)
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("penerima tidak ditemukan atau tidak aktif")
		}
		if err != nil {
			return errors.New("gagal memvalidasi penerima")
		}
		if err := validateDraftRecipientDirectoratePolicy(creatorScope, recipientScope); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) draftCreatorPositionScope(ctx context.Context, tx pgx.Tx, positionID string) (draftPositionScope, error) {
	var scope draftPositionScope
	err := tx.QueryRow(ctx, `
		WITH RECURSIVE ancestors AS (
			SELECT ou.id, ou.parent_id, ou.unit_level
			FROM positions p
			JOIN org_units ou ON ou.id = p.org_unit_id
			WHERE p.id = $1 AND p.is_active AND ou.is_active
			UNION ALL
			SELECT parent.id, parent.parent_id, parent.unit_level
			FROM org_units parent
			JOIN ancestors child ON child.parent_id = parent.id
			WHERE parent.is_active
		)
		SELECT p.position_type,
		       (SELECT id::text FROM ancestors WHERE unit_level = 'directorate' LIMIT 1)
		FROM positions p
		WHERE p.id = $1 AND p.is_active`, positionID).Scan(&scope.PositionType, &scope.DirectorateID)
	return scope, err
}

func (h *Handler) draftRecipientScope(ctx context.Context, tx pgx.Tx, recipient draftRecipientRequest) (draftRecipientScope, error) {
	scope := draftRecipientScope{TargetType: recipient.TargetType}
	var err error
	switch recipient.TargetType {
	case "position":
		err = tx.QueryRow(ctx, `
			WITH RECURSIVE ancestors AS (
				SELECT ou.id, ou.parent_id, ou.unit_level
				FROM positions p
				JOIN org_units ou ON ou.id = p.org_unit_id
				WHERE p.id = $1 AND p.is_active AND ou.is_active
				UNION ALL
				SELECT parent.id, parent.parent_id, parent.unit_level
				FROM org_units parent
				JOIN ancestors child ON child.parent_id = parent.id
				WHERE parent.is_active
			)
			SELECT (SELECT id::text FROM ancestors WHERE unit_level = 'directorate' LIMIT 1)
			FROM positions p
			WHERE p.id = $1 AND p.is_active`, recipient.TargetID).Scan(&scope.DirectorateID)
	case "org_unit":
		err = tx.QueryRow(ctx, `
			WITH RECURSIVE ancestors AS (
				SELECT ou.id, ou.parent_id, ou.unit_level
				FROM org_units ou
				WHERE ou.id = $1 AND ou.is_active
				UNION ALL
				SELECT parent.id, parent.parent_id, parent.unit_level
				FROM org_units parent
				JOIN ancestors child ON child.parent_id = parent.id
				WHERE parent.is_active
			)
			SELECT (SELECT id::text FROM ancestors WHERE unit_level = 'directorate' LIMIT 1)
			FROM org_units ou
			WHERE ou.id = $1 AND ou.is_active`, recipient.TargetID).Scan(&scope.DirectorateID)
	}
	return scope, err
}

func validateDraftRecipientDirectoratePolicy(creator draftPositionScope, recipient draftRecipientScope) error {
	if creator.DirectorateID == nil || recipient.DirectorateID == nil {
		return nil
	}
	if *creator.DirectorateID == *recipient.DirectorateID {
		return nil
	}
	if !isManagerOrAbovePositionType(creator.PositionType) {
		return errors.New("surat lintas direktorat hanya dapat dibuat oleh level manager ke atas")
	}
	if recipient.TargetType == "org_unit" {
		return errors.New("penerima unit lintas direktorat tidak diizinkan; pilih jabatan tujuan")
	}
	return nil
}

func isManagerOrAbovePositionType(positionType string) bool {
	switch positionType {
	case "sub_dept_head", "dept_head", "gm", "director", "vp_director", "president_director":
		return true
	default:
		return false
	}
}

func insertDraftRecipients(ctx context.Context, tx pgx.Tx, letterID string, recipients []draftRecipientRequest) error {
	for _, recipient := range recipients {
		if recipient.TargetType == "position" {
			if _, err := tx.Exec(ctx, `
				INSERT INTO letter_recipients (letter_id, recipient_type, position_id)
				VALUES ($1, $2, $3)`,
				letterID, recipient.Type, recipient.TargetID); err != nil {
				return err
			}
			continue
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO letter_recipients (letter_id, recipient_type, org_unit_id)
			VALUES ($1, $2, $3)`,
			letterID, recipient.Type, recipient.TargetID); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) userCanUsePosition(ctx context.Context, userID string, positionID string) (bool, error) {
	var exists bool
	err := h.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM user_positions
			WHERE user_id = $1
			  AND position_id = $2
			  AND current_date >= valid_from
			  AND (valid_to IS NULL OR current_date < valid_to)
		)`, userID, positionID).Scan(&exists)
	return exists, err
}

func (h *Handler) defaultLetterClassification(ctx context.Context, letterTypeID string) (string, error) {
	var classification string
	err := h.DB.QueryRow(ctx, `
		SELECT default_classification
		FROM letter_types
		WHERE id = $1 AND is_active`, letterTypeID).Scan(&classification)
	return classification, err
}

func draftLetterSelect(suffix string) string {
	return `
		SELECT l.id::text, l.company_id::text, co.code, co.name,
		       l.letter_type_id::text, lt.code, lt.name,
		       l.letter_number, l.subject, l.classification, l.priority, l.status,
		       l.creator_position_id::text, p.title,
		       l.on_behalf_of_position_id::text, obp.title,
		       COALESCE(v.version, 0), COALESCE(v.body_html, ''),
		       COALESCE(v.body_plain, ''), l.created_at, l.updated_at
		FROM letters l
		JOIN companies co ON co.id = l.company_id
		JOIN letter_types lt ON lt.id = l.letter_type_id
		JOIN positions p ON p.id = l.creator_position_id
		LEFT JOIN positions obp ON obp.id = l.on_behalf_of_position_id
		LEFT JOIN LATERAL (
			SELECT version, body_html, body_plain
			FROM letter_versions
			WHERE letter_id = l.id
			ORDER BY version DESC
			LIMIT 1
		) v ON true
		` + suffix
}

func scanDraftLetters(c *gin.Context, rows pgx.Rows) ([]DraftLetter, bool) {
	letters := []DraftLetter{}
	for rows.Next() {
		var letter DraftLetter
		if err := rows.Scan(
			&letter.ID,
			&letter.CompanyID,
			&letter.CompanyCode,
			&letter.CompanyName,
			&letter.LetterTypeID,
			&letter.LetterTypeCode,
			&letter.LetterTypeName,
			&letter.LetterNumber,
			&letter.Subject,
			&letter.Classification,
			&letter.Priority,
			&letter.Status,
			&letter.CreatorPositionID,
			&letter.CreatorPositionTitle,
			&letter.OnBehalfOfPositionID,
			&letter.OnBehalfOfTitle,
			&letter.Version,
			&letter.BodyHTML,
			&letter.BodyPlain,
			&letter.CreatedAt,
			&letter.UpdatedAt,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca data draft surat"})
			return nil, false
		}
		letters = append(letters, letter)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar draft surat"})
		return nil, false
	}
	return letters, true
}

func (h *Handler) attachDraftRecipients(c *gin.Context, letters []DraftLetter) bool {
	if len(letters) == 0 {
		return true
	}

	letterIDs := make([]string, 0, len(letters))
	indexByID := map[string]int{}
	for i := range letters {
		letterIDs = append(letterIDs, letters[i].ID)
		indexByID[letters[i].ID] = i
		letters[i].Recipients = []DraftRecipient{}
	}

	rows, err := h.DB.Query(c.Request.Context(), `
		SELECT lr.letter_id::text, lr.recipient_type,
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
		WHERE lr.letter_id = ANY($1)
		ORDER BY lr.recipient_type DESC, label`, letterIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat penerima draft"})
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var letterID string
		var recipient DraftRecipient
		if err := rows.Scan(&letterID, &recipient.Type, &recipient.TargetType, &recipient.TargetID, &recipient.Label); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca penerima draft"})
			return false
		}
		if idx, ok := indexByID[letterID]; ok {
			letters[idx].Recipients = append(letters[idx].Recipients, recipient)
		}
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar penerima draft"})
		return false
	}
	return true
}

func htmlToPlain(body string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(body))
	parts := []string{}
	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case html.ErrorToken:
			return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
		case html.TextToken:
			text := strings.TrimSpace(string(tokenizer.Text()))
			if text != "" {
				parts = append(parts, text)
			}
		}
	}
}

func sanitizeLetterHTML(body string) string {
	textAlign := regexp.MustCompile(`^text-align:\s*(left|right|center|justify);?$`)
	cellSpan := regexp.MustCompile(`^[1-9][0-9]?$`)

	policy := bluemonday.NewPolicy()
	policy.AllowElements("p", "br", "strong", "em", "u", "s", "blockquote", "pre", "code")
	policy.AllowElements("h1", "h2", "h3", "h4", "h5", "h6")
	policy.AllowElements("ul", "ol", "li")
	policy.AllowElements("table", "thead", "tbody", "tfoot", "tr", "th", "td")
	policy.AllowAttrs("style").Matching(textAlign).OnElements("p", "h1", "h2", "h3", "h4", "h5", "h6", "th", "td")
	policy.AllowAttrs("colspan", "rowspan").Matching(cellSpan).OnElements("th", "td")

	return strings.TrimSpace(policy.Sanitize(body))
}
