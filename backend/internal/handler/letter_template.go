package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

type LetterTemplate struct {
	ID             string          `json:"id"`
	LetterTypeID   string          `json:"letter_type_id"`
	LetterTypeCode string          `json:"letter_type_code"`
	LetterTypeName string          `json:"letter_type_name"`
	CompanyID      string          `json:"company_id"`
	CompanyCode    string          `json:"company_code"`
	CompanyName    string          `json:"company_name"`
	Version        int             `json:"version"`
	LayoutConfig   json.RawMessage `json:"layout_config"`
	BodySkeleton   string          `json:"body_skeleton"`
	IsActive       bool            `json:"is_active"`
	CreatedAt      time.Time       `json:"created_at"`
}

type letterTemplateRequest struct {
	LetterTypeID string          `json:"letter_type_id"`
	CompanyID    string          `json:"company_id"`
	Version      *int            `json:"version"`
	LayoutConfig json.RawMessage `json:"layout_config"`
	BodySkeleton string          `json:"body_skeleton"`
	IsActive     *bool           `json:"is_active"`
}

func (h *Handler) ListLetterTemplates(c *gin.Context) {
	includeInactive := c.Query("include_inactive") == "true"
	letterTypeID := strings.TrimSpace(c.Query("letter_type_id"))
	companyID := strings.TrimSpace(c.Query("company_id"))

	args := []any{}
	where := []string{}
	if !includeInactive {
		where = append(where, "t.is_active")
	}
	if letterTypeID != "" {
		args = append(args, letterTypeID)
		where = append(where, fmt.Sprintf("t.letter_type_id = $%d", len(args)))
	}
	if companyID != "" {
		args = append(args, companyID)
		where = append(where, fmt.Sprintf("t.company_id = $%d", len(args)))
	}

	query := `
		SELECT t.id::text, t.letter_type_id::text, lt.code, lt.name,
		       t.company_id::text, co.code, co.name, t.version,
		       t.layout_config, t.body_skeleton, t.is_active, t.created_at
		FROM letter_templates t
		JOIN letter_types lt ON lt.id = t.letter_type_id
		JOIN companies co ON co.id = t.company_id`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY co.code, lt.code, t.version DESC"

	rows, err := h.DB.Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat template surat"})
		return
	}
	defer rows.Close()

	templates, ok := scanLetterTemplates(c, rows)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"letter_templates": templates})
}

func (h *Handler) CreateLetterTemplate(c *gin.Context) {
	var req letterTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data template tidak valid: " + err.Error()})
		return
	}
	if err := normalizeLetterTemplateRequest(&req, true); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi"})
		return
	}
	defer tx.Rollback(ctx)

	version := 0
	if req.Version != nil {
		version = *req.Version
	} else {
		version, err = nextTemplateVersion(ctx, tx, req.LetterTypeID, req.CompanyID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menentukan versi template"})
			return
		}
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	if isActive {
		if err := deactivateSiblingTemplates(ctx, tx, req.LetterTypeID, req.CompanyID, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menonaktifkan template lama"})
			return
		}
	}

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO letter_templates
			(letter_type_id, company_id, version, layout_config, body_skeleton, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id::text`,
		req.LetterTypeID, req.CompanyID, version, req.LayoutConfig, req.BodySkeleton, isActive).Scan(&id)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "gagal membuat template (kombinasi versi mungkin sudah ada)"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan template"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "letter_template", &id, "create", &actor, map[string]any{
		"letter_type_id": req.LetterTypeID,
		"company_id":     req.CompanyID,
		"version":        version,
		"is_active":      isActive,
	}, c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *Handler) UpdateLetterTemplate(c *gin.Context) {
	id := c.Param("id")
	var req letterTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data template tidak valid: " + err.Error()})
		return
	}
	if err := normalizeLetterTemplateRequest(&req, false); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi"})
		return
	}
	defer tx.Rollback(ctx)

	if req.Version == nil {
		var version int
		err := tx.QueryRow(ctx, `SELECT version FROM letter_templates WHERE id = $1`, id).Scan(&version)
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "template tidak ditemukan"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca template"})
			return
		}
		req.Version = &version
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	if isActive {
		if err := deactivateSiblingTemplates(ctx, tx, req.LetterTypeID, req.CompanyID, &id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menonaktifkan template lama"})
			return
		}
	}

	tag, err := tx.Exec(ctx, `
		UPDATE letter_templates
		SET letter_type_id = $2, company_id = $3, version = $4,
		    layout_config = $5, body_skeleton = $6, is_active = $7
		WHERE id = $1`,
		id, req.LetterTypeID, req.CompanyID, *req.Version, req.LayoutConfig, req.BodySkeleton, isActive)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "gagal memperbarui template (kombinasi versi mungkin sudah ada)"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "template tidak ditemukan"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan template"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "letter_template", &id, "update", &actor, map[string]any{
		"letter_type_id": req.LetterTypeID,
		"company_id":     req.CompanyID,
		"version":        *req.Version,
		"is_active":      isActive,
	}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
}

func (h *Handler) ActivateLetterTemplate(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi"})
		return
	}
	defer tx.Rollback(ctx)

	var letterTypeID, companyID string
	err = tx.QueryRow(ctx, `
		SELECT letter_type_id::text, company_id::text
		FROM letter_templates
		WHERE id = $1`, id).Scan(&letterTypeID, &companyID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "template tidak ditemukan"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca template"})
		return
	}
	if err := deactivateSiblingTemplates(ctx, tx, letterTypeID, companyID, &id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menonaktifkan template lama"})
		return
	}
	if _, err := tx.Exec(ctx, `UPDATE letter_templates SET is_active = true WHERE id = $1`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengaktifkan template"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan template"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "letter_template", &id, "activate", &actor, nil, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
}

func (h *Handler) DeactivateLetterTemplate(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	tag, err := h.DB.Exec(ctx, `UPDATE letter_templates SET is_active = false WHERE id = $1 AND is_active`, id)
	if err != nil || tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "template tidak ditemukan atau sudah nonaktif"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "letter_template", &id, "deactivate", &actor, nil, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
}

func normalizeLetterTemplateRequest(req *letterTemplateRequest, allowAutoVersion bool) error {
	req.LetterTypeID = strings.TrimSpace(req.LetterTypeID)
	req.CompanyID = strings.TrimSpace(req.CompanyID)
	req.BodySkeleton = strings.TrimSpace(req.BodySkeleton)

	if req.LetterTypeID == "" {
		return errors.New("jenis surat wajib dipilih")
	}
	if req.CompanyID == "" {
		return errors.New("perusahaan wajib dipilih")
	}
	if req.Version != nil && *req.Version < 1 {
		return errors.New("versi template wajib lebih besar dari 0")
	}
	if !allowAutoVersion && req.Version == nil {
		return errors.New("versi template wajib diisi")
	}
	if len(req.LayoutConfig) == 0 {
		req.LayoutConfig = json.RawMessage(`{}`)
	}
	var layout map[string]any
	if err := json.Unmarshal(req.LayoutConfig, &layout); err != nil {
		return errors.New("layout_config wajib berupa JSON object yang valid")
	}
	if layout == nil {
		return errors.New("layout_config wajib berupa JSON object")
	}
	if req.BodySkeleton == "" {
		return errors.New("body_skeleton wajib diisi")
	}
	return nil
}

func scanLetterTemplates(c *gin.Context, rows pgx.Rows) ([]LetterTemplate, bool) {
	templates := []LetterTemplate{}
	for rows.Next() {
		var template LetterTemplate
		var layout []byte
		if err := rows.Scan(
			&template.ID,
			&template.LetterTypeID,
			&template.LetterTypeCode,
			&template.LetterTypeName,
			&template.CompanyID,
			&template.CompanyCode,
			&template.CompanyName,
			&template.Version,
			&layout,
			&template.BodySkeleton,
			&template.IsActive,
			&template.CreatedAt,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca data template surat"})
			return nil, false
		}
		template.LayoutConfig = json.RawMessage(layout)
		templates = append(templates, template)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar template surat"})
		return nil, false
	}
	return templates, true
}

func nextTemplateVersion(ctx context.Context, tx pgx.Tx, letterTypeID string, companyID string) (int, error) {
	var version int
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM letter_templates
		WHERE letter_type_id = $1 AND company_id = $2`,
		letterTypeID, companyID).Scan(&version)
	return version, err
}

func deactivateSiblingTemplates(ctx context.Context, tx pgx.Tx, letterTypeID string, companyID string, exceptID *string) error {
	if exceptID == nil {
		_, err := tx.Exec(ctx, `
			UPDATE letter_templates
			SET is_active = false
			WHERE letter_type_id = $1 AND company_id = $2`,
			letterTypeID, companyID)
		return err
	}

	_, err := tx.Exec(ctx, `
		UPDATE letter_templates
		SET is_active = false
		WHERE letter_type_id = $1 AND company_id = $2 AND id <> $3`,
		letterTypeID, companyID, *exceptID)
	return err
}
