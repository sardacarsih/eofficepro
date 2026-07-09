package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

type LetterType struct {
	ID                    string `json:"id"`
	Code                  string `json:"code"`
	Name                  string `json:"name"`
	DefaultClassification string `json:"default_classification"`
	DefaultSLAHours       int    `json:"default_sla_hours"`
	IsActive              bool   `json:"is_active"`
}

func (h *Handler) ListLetterTypes(c *gin.Context) {
	includeInactive := c.Query("include_inactive") == "true"
	page, pageSize, offset, ok := parsePagination(c.Query("page"), c.Query("page_size"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page atau page_size tidak valid"})
		return
	}

	whereSQL := ""
	if !includeInactive {
		whereSQL = ` WHERE is_active`
	}

	ctx := c.Request.Context()
	var total int64
	if err := h.DB.QueryRow(ctx, `SELECT count(*) FROM letter_types`+whereSQL).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menghitung jenis surat"})
		return
	}

	query := `
		SELECT id::text, code, name, default_classification, default_sla_hours, is_active
		FROM letter_types` + whereSQL + ` ORDER BY code LIMIT $1 OFFSET $2`

	rows, err := h.DB.Query(ctx, query, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat jenis surat"})
		return
	}
	defer rows.Close()

	letterTypes := []LetterType{}
	for rows.Next() {
		var lt LetterType
		if err := rows.Scan(&lt.ID, &lt.Code, &lt.Name, &lt.DefaultClassification, &lt.DefaultSLAHours, &lt.IsActive); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca data jenis surat"})
			return
		}
		letterTypes = append(letterTypes, lt)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar jenis surat"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": letterTypes, "meta": newPageMeta(page, pageSize, total)})
}

type letterTypeRequest struct {
	Code                  string `json:"code" binding:"required,max=5"`
	Name                  string `json:"name" binding:"required,max=100"`
	DefaultClassification string `json:"default_classification" binding:"required,oneof=biasa terbatas rahasia"`
	DefaultSLAHours       int    `json:"default_sla_hours" binding:"required,min=1,max=720"`
	IsActive              *bool  `json:"is_active"`
}

func normalizeLetterTypeRequest(req *letterTypeRequest) {
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	req.Name = strings.TrimSpace(req.Name)
	req.DefaultClassification = strings.ToLower(strings.TrimSpace(req.DefaultClassification))
}

func (h *Handler) CreateLetterType(c *gin.Context) {
	var req letterTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data jenis surat tidak valid: " + err.Error()})
		return
	}
	normalizeLetterTypeRequest(&req)

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	ctx := c.Request.Context()
	var id string
	err := h.DB.QueryRow(ctx, `
		INSERT INTO letter_types (code, name, default_classification, default_sla_hours, is_active)
		VALUES ($1, $2, $3, $4, $5) RETURNING id::text`,
		req.Code, req.Name, req.DefaultClassification, req.DefaultSLAHours, isActive).Scan(&id)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "gagal membuat jenis surat (kode mungkin sudah dipakai)"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "letter_type", &id, "create", &actor, map[string]any{
		"code": req.Code, "name": req.Name, "classification": req.DefaultClassification,
	}, c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *Handler) UpdateLetterType(c *gin.Context) {
	id := c.Param("id")
	var req letterTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data jenis surat tidak valid: " + err.Error()})
		return
	}
	normalizeLetterTypeRequest(&req)

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	ctx := c.Request.Context()
	tag, err := h.DB.Exec(ctx, `
		UPDATE letter_types
		SET code = $2, name = $3, default_classification = $4, default_sla_hours = $5, is_active = $6
		WHERE id = $1`,
		id, req.Code, req.Name, req.DefaultClassification, req.DefaultSLAHours, isActive)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "gagal memperbarui jenis surat (kode mungkin sudah dipakai)"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "jenis surat tidak ditemukan"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "letter_type", &id, "update", &actor, map[string]any{
		"code": req.Code, "name": req.Name, "classification": req.DefaultClassification, "is_active": isActive,
	}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
}

func (h *Handler) DeactivateLetterType(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	tag, err := h.DB.Exec(ctx, `UPDATE letter_types SET is_active = false WHERE id = $1 AND is_active`, id)
	if err != nil || tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "jenis surat tidak ditemukan atau sudah nonaktif"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "letter_type", &id, "deactivate", &actor, nil, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
}
