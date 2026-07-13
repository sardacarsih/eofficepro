package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

type ApprovalMatrix struct {
	ID                   string     `json:"id"`
	LetterTypeID         string     `json:"letter_type_id"`
	LetterTypeCode       string     `json:"letter_type_code"`
	LetterTypeName       string     `json:"letter_type_name"`
	OriginatorLevel      *string    `json:"originator_level"`
	ApprovalCategoryID   *string    `json:"approval_category_id"`
	ApprovalCategoryName *string    `json:"approval_category_name"`
	ResolutionMode       string     `json:"resolution_mode"`
	MinFinalLevel        *string    `json:"min_final_level"`
	MaxFinalLevel        *string    `json:"max_final_level"`
	FinalLevel           string     `json:"final_level"`
	FlowMode             string     `json:"flow_mode"`
	IsActive             bool       `json:"is_active"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            *time.Time `json:"updated_at"`
}

type approvalMatrixRequest struct {
	LetterTypeID       string  `json:"letter_type_id" binding:"required"`
	OriginatorLevel    *string `json:"originator_level"`
	ApprovalCategoryID *string `json:"approval_category_id"`
	ResolutionMode     string  `json:"resolution_mode"`
	MinFinalLevel      *string `json:"min_final_level"`
	MaxFinalLevel      *string `json:"max_final_level"`
	FinalLevel         string  `json:"final_level" binding:"required"`
	FlowMode           string  `json:"flow_mode"`
	IsActive           *bool   `json:"is_active"`
}

var validApprovalMatrixOriginatorLevels = map[string]bool{
	"president_director": true,
	"vp_director":        true,
	"director":           true,
	"gm":                 true,
	"dept_head":          true,
	"sub_dept_head":      true,
	"division_head":      true,
	"assistant":          true,
	"secretary":          true,
	"staff":              true,
	"auditor":            true,
}

var validApprovalMatrixFinalLevels = map[string]bool{
	"president_director": true,
	"vp_director":        true,
	"director":           true,
	"gm":                 true,
	"dept_head":          true,
	"sub_dept_head":      true,
	"division_head":      true,
}

func (h *Handler) ListApprovalMatrices(c *gin.Context) {
	includeInactive := c.Query("include_inactive") == "true"
	page, pageSize, offset, ok := parsePagination(c.Query("page"), c.Query("page_size"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page atau page_size tidak valid"})
		return
	}

	whereSQL := ""
	if !includeInactive {
		whereSQL = ` WHERE am.is_active`
	}

	ctx := c.Request.Context()
	var total int64
	if err := h.DB.QueryRow(ctx, `
		SELECT count(*)
		FROM approval_matrices am
		JOIN letter_types lt ON lt.id = am.letter_type_id`+whereSQL).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menghitung matrix approval"})
		return
	}

	query := `
		SELECT am.id::text, am.letter_type_id::text, lt.code, lt.name,
		       am.originator_level, am.approval_category_id::text, ac.name,
		       am.resolution_mode, am.min_final_level, am.max_final_level,
		       am.final_level, am.flow_mode, am.is_active,
		       am.created_at, am.updated_at
		FROM approval_matrices am
		JOIN letter_types lt ON lt.id = am.letter_type_id` +
		` LEFT JOIN approval_categories ac ON ac.id=am.approval_category_id` +
		whereSQL + `
		ORDER BY lt.code,
		         CASE WHEN am.originator_level IS NULL THEN 0 ELSE 1 END,
		         am.originator_level NULLS FIRST,
		         am.created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := h.DB.Query(ctx, query, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat matrix approval"})
		return
	}
	defer rows.Close()

	matrices := []ApprovalMatrix{}
	for rows.Next() {
		var matrix ApprovalMatrix
		if err := rows.Scan(
			&matrix.ID,
			&matrix.LetterTypeID,
			&matrix.LetterTypeCode,
			&matrix.LetterTypeName,
			&matrix.OriginatorLevel,
			&matrix.ApprovalCategoryID,
			&matrix.ApprovalCategoryName,
			&matrix.ResolutionMode,
			&matrix.MinFinalLevel,
			&matrix.MaxFinalLevel,
			&matrix.FinalLevel,
			&matrix.FlowMode,
			&matrix.IsActive,
			&matrix.CreatedAt,
			&matrix.UpdatedAt,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca data matrix approval"})
			return
		}
		matrices = append(matrices, matrix)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar matrix approval"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": matrices, "meta": newPageMeta(page, pageSize, total)})
}

func (h *Handler) CreateApprovalMatrix(c *gin.Context) {
	var req approvalMatrixRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data matrix approval tidak valid: " + err.Error()})
		return
	}
	if err := normalizeApprovalMatrixRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	exists, err := h.letterTypeExists(ctx, req.LetterTypeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memvalidasi jenis surat"})
		return
	}
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jenis surat tidak ditemukan"})
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	if isActive {
		duplicate, err := h.activeApprovalMatrixRuleExists(ctx, req.LetterTypeID, req.OriginatorLevel, req.ApprovalCategoryID, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa duplikasi matrix approval"})
			return
		}
		if duplicate {
			c.JSON(http.StatusConflict, gin.H{"error": "aturan aktif untuk jenis surat dan originator ini sudah ada"})
			return
		}
	}

	var id string
	err = h.DB.QueryRow(ctx, `
		INSERT INTO approval_matrices
			(letter_type_id, originator_level, approval_category_id, resolution_mode,
			 min_final_level, max_final_level, final_level, flow_mode, extra_steps, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'serial', '[]'::jsonb, $8)
		RETURNING id::text`,
		req.LetterTypeID, req.OriginatorLevel, req.ApprovalCategoryID, req.ResolutionMode,
		req.MinFinalLevel, req.MaxFinalLevel, req.FinalLevel, isActive).Scan(&id)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "gagal membuat matrix approval (aturan aktif untuk jenis surat dan originator ini mungkin sudah ada)"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "approval_matrix", &id, "create", &actor, map[string]any{
		"letter_type_id":   req.LetterTypeID,
		"originator_level": req.OriginatorLevel,
		"final_level":      req.FinalLevel,
		"flow_mode":        "serial",
		"is_active":        isActive,
	}, c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *Handler) UpdateApprovalMatrix(c *gin.Context) {
	id := c.Param("id")
	var req approvalMatrixRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data matrix approval tidak valid: " + err.Error()})
		return
	}
	if err := normalizeApprovalMatrixRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	exists, err := h.letterTypeExists(ctx, req.LetterTypeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memvalidasi jenis surat"})
		return
	}
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jenis surat tidak ditemukan"})
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	if isActive {
		duplicate, err := h.activeApprovalMatrixRuleExists(ctx, req.LetterTypeID, req.OriginatorLevel, req.ApprovalCategoryID, &id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa duplikasi matrix approval"})
			return
		}
		if duplicate {
			c.JSON(http.StatusConflict, gin.H{"error": "aturan aktif untuk jenis surat dan originator ini sudah ada"})
			return
		}
	}

	tag, err := h.DB.Exec(ctx, `
		UPDATE approval_matrices
		SET letter_type_id = $2,
		    originator_level = $3,
		    approval_category_id = $4, resolution_mode=$5,
		    min_final_level=$6, max_final_level=$7, final_level = $8,
		    flow_mode = 'serial',
		    extra_steps = '[]'::jsonb,
		    is_active = $9,
		    updated_at = now()
		WHERE id = $1`,
		id, req.LetterTypeID, req.OriginatorLevel, req.ApprovalCategoryID, req.ResolutionMode,
		req.MinFinalLevel, req.MaxFinalLevel, req.FinalLevel, isActive)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "gagal memperbarui matrix approval (aturan aktif untuk jenis surat dan originator ini mungkin sudah ada)"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "matrix approval tidak ditemukan"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "approval_matrix", &id, "update", &actor, map[string]any{
		"letter_type_id":   req.LetterTypeID,
		"originator_level": req.OriginatorLevel,
		"final_level":      req.FinalLevel,
		"flow_mode":        "serial",
		"is_active":        isActive,
	}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
}

func (h *Handler) DeactivateApprovalMatrix(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	tag, err := h.DB.Exec(ctx, `UPDATE approval_matrices SET is_active = false, updated_at = now() WHERE id = $1 AND is_active`, id)
	if err != nil || tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "matrix approval tidak ditemukan atau sudah nonaktif"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "approval_matrix", &id, "deactivate", &actor, nil, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
}

func normalizeApprovalMatrixRequest(req *approvalMatrixRequest) error {
	req.LetterTypeID = strings.TrimSpace(req.LetterTypeID)
	req.FinalLevel = strings.ToLower(strings.TrimSpace(req.FinalLevel))
	req.FlowMode = strings.ToLower(strings.TrimSpace(req.FlowMode))
	req.ResolutionMode = strings.ToLower(strings.TrimSpace(req.ResolutionMode))
	if req.ResolutionMode == "" {
		req.ResolutionMode = "fixed"
	}
	if req.FlowMode == "" {
		req.FlowMode = "serial"
	}

	if req.LetterTypeID == "" {
		return errors.New("jenis surat wajib dipilih")
	}
	if req.OriginatorLevel != nil {
		originatorLevel := strings.ToLower(strings.TrimSpace(*req.OriginatorLevel))
		if originatorLevel == "" {
			req.OriginatorLevel = nil
		} else {
			if !validApprovalMatrixOriginatorLevels[originatorLevel] {
				return errors.New("originator level tidak valid")
			}
			req.OriginatorLevel = &originatorLevel
		}
	}
	if req.ApprovalCategoryID != nil {
		value := strings.TrimSpace(*req.ApprovalCategoryID)
		if value == "" {
			req.ApprovalCategoryID = nil
		} else {
			req.ApprovalCategoryID = &value
		}
	}
	for _, level := range []*string{req.MinFinalLevel, req.MaxFinalLevel} {
		if level != nil {
			*level = strings.ToLower(strings.TrimSpace(*level))
		}
	}
	if !validApprovalMatrixFinalLevels[req.FinalLevel] {
		return errors.New("final approval tidak valid")
	}
	if req.ResolutionMode != "fixed" && req.ResolutionMode != "user_selected" && req.ResolutionMode != "scope_derived" {
		return errors.New("mode resolusi approval tidak valid")
	}
	for _, level := range []*string{req.MinFinalLevel, req.MaxFinalLevel} {
		if level != nil && !validApprovalMatrixFinalLevels[*level] {
			return errors.New("batas level approval tidak valid")
		}
	}
	if req.ResolutionMode == "user_selected" {
		if req.ApprovalCategoryID == nil || req.MinFinalLevel == nil || req.MaxFinalLevel == nil {
			return errors.New("kategori serta batas minimum dan maksimum wajib untuk mode pilihan pengguna")
		}
		if approvalLevelRank[*req.MinFinalLevel] > approvalLevelRank[*req.MaxFinalLevel] {
			return errors.New("batas minimum tidak boleh lebih tinggi dari batas maksimum")
		}
	}
	if req.FlowMode != "serial" {
		return errors.New("mode approval parallel belum didukung; gunakan serial")
	}
	return nil
}

func (h *Handler) letterTypeExists(ctx context.Context, letterTypeID string) (bool, error) {
	var exists bool
	err := h.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM letter_types WHERE id = $1)`, letterTypeID).Scan(&exists)
	return exists, err
}

func (h *Handler) activeApprovalMatrixRuleExists(ctx context.Context, letterTypeID string, originatorLevel, categoryID *string, exceptID *string) (bool, error) {
	args := []any{letterTypeID}
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM approval_matrices
			WHERE letter_type_id = $1
			  AND is_active`
	if originatorLevel == nil {
		query += ` AND originator_level IS NULL`
	} else {
		args = append(args, *originatorLevel)
		query += ` AND originator_level = $2`
	}
	if categoryID == nil {
		query += ` AND approval_category_id IS NULL`
	} else {
		args = append(args, *categoryID)
		query += ` AND approval_category_id = $` + strconv.Itoa(len(args))
	}
	if exceptID != nil {
		args = append(args, *exceptID)
		query += ` AND id <> $` + strconv.Itoa(len(args))
	}
	query += `)`

	var exists bool
	err := h.DB.QueryRow(ctx, query, args...).Scan(&exists)
	return exists, err
}
