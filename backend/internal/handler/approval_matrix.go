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
	ID              string     `json:"id"`
	LetterTypeID    string     `json:"letter_type_id"`
	LetterTypeCode  string     `json:"letter_type_code"`
	LetterTypeName  string     `json:"letter_type_name"`
	OriginatorLevel *string    `json:"originator_level"`
	FinalLevel      string     `json:"final_level"`
	FlowMode        string     `json:"flow_mode"`
	IsActive        bool       `json:"is_active"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at"`
}

type approvalMatrixRequest struct {
	LetterTypeID    string  `json:"letter_type_id" binding:"required"`
	OriginatorLevel *string `json:"originator_level"`
	FinalLevel      string  `json:"final_level" binding:"required"`
	FlowMode        string  `json:"flow_mode"`
	IsActive        *bool   `json:"is_active"`
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
	query := `
		SELECT am.id::text, am.letter_type_id::text, lt.code, lt.name,
		       am.originator_level, am.final_level, am.flow_mode, am.is_active,
		       am.created_at, am.updated_at
		FROM approval_matrices am
		JOIN letter_types lt ON lt.id = am.letter_type_id`
	if !includeInactive {
		query += ` WHERE am.is_active`
	}
	query += `
		ORDER BY lt.code,
		         CASE WHEN am.originator_level IS NULL THEN 0 ELSE 1 END,
		         am.originator_level NULLS FIRST,
		         am.created_at DESC`

	rows, err := h.DB.Query(c.Request.Context(), query)
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

	c.JSON(http.StatusOK, gin.H{"approval_matrices": matrices})
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
		duplicate, err := h.activeApprovalMatrixRuleExists(ctx, req.LetterTypeID, req.OriginatorLevel, nil)
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
			(letter_type_id, originator_level, final_level, flow_mode, extra_steps, is_active)
		VALUES ($1, $2, $3, 'serial', '[]'::jsonb, $4)
		RETURNING id::text`,
		req.LetterTypeID, req.OriginatorLevel, req.FinalLevel, isActive).Scan(&id)
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
		duplicate, err := h.activeApprovalMatrixRuleExists(ctx, req.LetterTypeID, req.OriginatorLevel, &id)
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
		    final_level = $4,
		    flow_mode = 'serial',
		    extra_steps = '[]'::jsonb,
		    is_active = $5,
		    updated_at = now()
		WHERE id = $1`,
		id, req.LetterTypeID, req.OriginatorLevel, req.FinalLevel, isActive)
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
	if !validApprovalMatrixFinalLevels[req.FinalLevel] {
		return errors.New("final approval tidak valid")
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

func (h *Handler) activeApprovalMatrixRuleExists(ctx context.Context, letterTypeID string, originatorLevel *string, exceptID *string) (bool, error) {
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
	if exceptID != nil {
		args = append(args, *exceptID)
		query += ` AND id <> $` + strconv.Itoa(len(args))
	}
	query += `)`

	var exists bool
	err := h.DB.QueryRow(ctx, query, args...).Scan(&exists)
	return exists, err
}
