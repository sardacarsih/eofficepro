package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

var approvalLevelRank = map[string]int{
	"division_head": 1, "sub_dept_head": 2, "dept_head": 3,
	"gm": 4, "director": 5, "vp_director": 6, "president_director": 7,
}

type ApprovalCategory struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

type approvalCategoryRequest struct {
	Code string `json:"code" binding:"required,max=30"`
	Name string `json:"name" binding:"required,max=100"`
}

type CoordinationScopeRule struct {
	Scope      string `json:"scope"`
	FinalLevel string `json:"final_level"`
	IsActive   bool   `json:"is_active"`
}

type ApprovalRoutePreview struct {
	ResolutionMode string              `json:"resolution_mode"`
	FinalLevel     string              `json:"final_level"`
	Scope          string              `json:"coordination_scope,omitempty"`
	AllowedLevels  []string            `json:"allowed_levels"`
	Steps          []approvalRouteStep `json:"steps"`
}

type approvalPreviewRequest struct {
	LetterTypeID        string                  `json:"letter_type_id" binding:"required"`
	CreatorPositionID   string                  `json:"creator_position_id" binding:"required"`
	ApprovalCategoryID  *string                 `json:"approval_category_id"`
	RequestedFinalLevel *string                 `json:"requested_final_level"`
	Recipients          []draftRecipientRequest `json:"recipients"`
}

func (h *Handler) ListApprovalCategories(c *gin.Context) {
	includeInactive := c.Query("include_inactive") == "true"
	query := `SELECT id::text, code, name, is_active FROM approval_categories`
	if !includeInactive {
		query += ` WHERE is_active`
	}
	query += ` ORDER BY name`
	rows, err := h.DB.Query(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat kategori persetujuan"})
		return
	}
	defer rows.Close()
	categories := []ApprovalCategory{}
	for rows.Next() {
		var category ApprovalCategory
		if err := rows.Scan(&category.ID, &category.Code, &category.Name, &category.IsActive); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca kategori persetujuan"})
			return
		}
		categories = append(categories, category)
	}
	c.JSON(http.StatusOK, gin.H{"data": categories})
}

func (h *Handler) CreateApprovalCategory(c *gin.Context) {
	var req approvalCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data kategori tidak valid"})
		return
	}
	req.Code, req.Name = strings.ToUpper(strings.TrimSpace(req.Code)), strings.TrimSpace(req.Name)
	var id string
	if err := h.DB.QueryRow(c.Request.Context(), `INSERT INTO approval_categories(code,name) VALUES($1,$2) RETURNING id::text`, req.Code, req.Name).Scan(&id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "kode kategori sudah digunakan"})
		return
	}
	actor := c.GetString(middleware.CtxUserID)
	h.audit(c.Request.Context(), "approval_category", &id, "create", &actor, map[string]any{"code": req.Code, "name": req.Name}, c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *Handler) UpdateApprovalCategory(c *gin.Context) {
	var req approvalCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data kategori tidak valid"})
		return
	}
	req.Code, req.Name = strings.ToUpper(strings.TrimSpace(req.Code)), strings.TrimSpace(req.Name)
	tag, err := h.DB.Exec(c.Request.Context(), `UPDATE approval_categories SET code=$2,name=$3,updated_at=now() WHERE id=$1`, c.Param("id"), req.Code, req.Name)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "kode kategori sudah digunakan"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "kategori tidak ditemukan"})
		return
	}
	actor := c.GetString(middleware.CtxUserID)
	h.audit(c.Request.Context(), "approval_category", nil, "update", &actor, map[string]any{"id": c.Param("id")}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (h *Handler) DeactivateApprovalCategory(c *gin.Context) {
	tag, _ := h.DB.Exec(c.Request.Context(), `UPDATE approval_categories SET is_active=false,updated_at=now() WHERE id=$1 AND is_active`, c.Param("id"))
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "kategori tidak ditemukan atau sudah nonaktif"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (h *Handler) ListCoordinationScopeRules(c *gin.Context) {
	rows, err := h.DB.Query(c.Request.Context(), `SELECT scope,final_level,is_active FROM coordination_scope_rules ORDER BY scope`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat aturan cakupan"})
		return
	}
	defer rows.Close()
	rules := []CoordinationScopeRule{}
	for rows.Next() {
		var rule CoordinationScopeRule
		if rows.Scan(&rule.Scope, &rule.FinalLevel, &rule.IsActive) == nil {
			rules = append(rules, rule)
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": rules})
}

func (h *Handler) UpdateCoordinationScopeRule(c *gin.Context) {
	var req struct {
		FinalLevel string `json:"final_level" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !validApprovalMatrixFinalLevels[req.FinalLevel] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "level akhir tidak valid"})
		return
	}
	tag, err := h.DB.Exec(c.Request.Context(), `UPDATE coordination_scope_rules SET final_level=$2,is_active=true,updated_at=now() WHERE scope=$1`, c.Param("scope"), req.FinalLevel)
	if err != nil || tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "aturan cakupan tidak ditemukan"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"scope": c.Param("scope")})
}

func (h *Handler) PreviewApprovalRoute(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	var req approvalPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data preview rute tidak valid"})
		return
	}
	var companyID string
	err := h.DB.QueryRow(c.Request.Context(), `
		SELECT ou.company_id::text FROM positions p JOIN org_units ou ON ou.id=p.org_unit_id
		WHERE p.id=$1 AND p.is_active`, req.CreatorPositionID).Scan(&companyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jabatan pembuat tidak valid"})
		return
	}
	if allowed, err := h.userCanUsePositionForCompany(c.Request.Context(), userID, req.CreatorPositionID, companyID); err != nil || !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "jabatan pembuat tidak tersedia untuk pengguna ini"})
		return
	}
	preview, err := h.resolvePolicyRoute(c.Request.Context(), h.DB, req.LetterTypeID, req.CreatorPositionID,
		req.ApprovalCategoryID, req.RequestedFinalLevel, req.Recipients)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, preview)
}

type policyQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (h *Handler) resolvePolicyRoute(ctx context.Context, db policyQueryer, letterTypeID, creatorPositionID string,
	categoryID, requestedLevel *string, recipients []draftRecipientRequest) (ApprovalRoutePreview, error) {
	var code, mode, minLevel, maxLevel, fixedLevel string
	var category any
	if categoryID != nil && strings.TrimSpace(*categoryID) != "" {
		category = strings.TrimSpace(*categoryID)
	}
	err := db.QueryRow(ctx, `
		SELECT lt.code, am.resolution_mode, COALESCE(am.min_final_level,''),
		       COALESCE(am.max_final_level,''), am.final_level
		FROM letter_types lt
		JOIN positions creator ON creator.id=$2 AND creator.is_active
		JOIN approval_matrices am ON am.letter_type_id=lt.id AND am.is_active
		 AND (am.originator_level IS NULL OR am.originator_level=creator.position_type)
		 AND (($3::uuid IS NULL AND am.approval_category_id IS NULL) OR am.approval_category_id=$3)
		LEFT JOIN approval_categories ac ON ac.id=am.approval_category_id
		WHERE lt.id=$1 AND lt.is_active AND (ac.id IS NULL OR ac.is_active)
		ORDER BY (am.originator_level=creator.position_type) DESC LIMIT 1`,
		letterTypeID, creatorPositionID, category).Scan(&code, &mode, &minLevel, &maxLevel, &fixedLevel)
	if errors.Is(err, pgx.ErrNoRows) {
		return ApprovalRoutePreview{}, errors.New("kebijakan approval belum dikonfigurasi untuk jenis, kategori, dan jabatan pembuat")
	}
	if err != nil {
		return ApprovalRoutePreview{}, errors.New("gagal membaca kebijakan approval")
	}

	finalLevel := fixedLevel
	scope := ""
	allowed := []string{}
	if mode == "user_selected" {
		for level, rank := range approvalLevelRank {
			if rank >= approvalLevelRank[minLevel] && rank <= approvalLevelRank[maxLevel] {
				allowed = append(allowed, level)
			}
		}
		sort.Slice(allowed, func(i, j int) bool { return approvalLevelRank[allowed[i]] < approvalLevelRank[allowed[j]] })
		if requestedLevel == nil || strings.TrimSpace(*requestedLevel) == "" {
			finalLevel = minLevel
		} else if !containsString(allowed, *requestedLevel) {
			return ApprovalRoutePreview{ResolutionMode: mode, AllowedLevels: allowed}, errors.New("level akhir harus berada dalam batas kebijakan")
		} else {
			finalLevel = *requestedLevel
		}
	}
	if mode == "scope_derived" {
		var err error
		scope, err = resolveCoordinationScope(ctx, db, creatorPositionID, recipients)
		if err != nil {
			return ApprovalRoutePreview{}, err
		}
		if err := db.QueryRow(ctx, `SELECT final_level FROM coordination_scope_rules WHERE scope=$1 AND is_active`, scope).Scan(&finalLevel); err != nil {
			return ApprovalRoutePreview{}, errors.New("aturan cakupan koordinasi belum dikonfigurasi")
		}
		if scope == "same_unit" {
			finalLevel, err = resolveSameUnitFinalLevel(ctx, db, creatorPositionID, finalLevel)
			if err != nil {
				return ApprovalRoutePreview{}, err
			}
		}
	}

	route, err := buildRouteToLevel(ctx, db, creatorPositionID, finalLevel)
	if err != nil {
		return ApprovalRoutePreview{}, err
	}
	return ApprovalRoutePreview{ResolutionMode: mode, FinalLevel: finalLevel, Scope: scope, AllowedLevels: allowed, Steps: route.Steps}, nil
}

func buildRouteToLevel(ctx context.Context, db policyQueryer, creatorPositionID, finalLevel string) (approvalRoute, error) {
	creator, err := loadApprovalPosition(ctx, db, creatorPositionID)
	if err != nil {
		return approvalRoute{}, errors.New("jabatan pembuat tidak ditemukan")
	}
	steps := []approvalRouteStep{}
	nextID := creator.ReportsTo
	for nextID != nil {
		position, err := loadApprovalPosition(ctx, db, *nextID)
		if err != nil {
			return approvalRoute{}, errors.New("rantai atasan terputus atau tidak aktif")
		}
		if position.PositionType != "sub_dept_head" || positionHasHolder(ctx, db, position.ID) {
			steps = append(steps, approvalRouteStep{StepOrder: len(steps) + 1, FlowGroup: len(steps) + 1,
				PositionID: position.ID, PositionType: position.PositionType, Title: position.Title})
		}
		if position.PositionType == finalLevel {
			return approvalRoute{SLAHours: 24, Steps: steps}, nil
		}
		nextID = position.ReportsTo
	}
	return approvalRoute{}, errors.New("level akhir tidak ditemukan dalam rantai atasan pembuat")
}

func positionHasHolder(ctx context.Context, db policyQueryer, positionID string) bool {
	var exists bool
	_ = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_positions up JOIN users u ON u.id=up.user_id
		WHERE up.position_id=$1 AND u.status='active' AND current_date>=up.valid_from
		AND (up.valid_to IS NULL OR current_date<up.valid_to))`, positionID).Scan(&exists)
	return exists
}

type orgAncestry struct{ unit, department, biro, directorate, level string }

func resolveCoordinationScope(ctx context.Context, db policyQueryer, creatorPositionID string, recipients []draftRecipientRequest) (string, error) {
	creator, err := loadPositionAncestry(ctx, db, creatorPositionID, "position")
	if err != nil {
		return "", errors.New("unit jabatan pembuat tidak ditemukan")
	}
	scopeRank := 0
	targetDirectorates := map[string]bool{}
	for _, recipient := range recipients {
		if recipient.Type != "to" {
			continue
		}
		target, err := loadPositionAncestry(ctx, db, recipient.TargetID, recipient.TargetType)
		if err != nil {
			return "", fmt.Errorf("penerima koordinasi %s (%s) tidak ditemukan atau tidak aktif", recipient.TargetID, recipient.TargetType)
		}
		if target.directorate != "" {
			targetDirectorates[target.directorate] = true
		}
		rank := 0
		switch {
		case target.level == "office":
			rank = 4
		case creator.directorate != target.directorate:
			rank = 3
		case creator.biro != target.biro:
			rank = 2
		case creator.department != target.department:
			rank = 1
		}
		if rank > scopeRank {
			scopeRank = rank
		}
	}
	if scopeRank == 3 {
		var totalDirectorates int
		err := db.QueryRow(ctx, `
			SELECT count(*) FROM org_units
			WHERE company_id=(SELECT ou.company_id FROM positions p JOIN org_units ou ON ou.id=p.org_unit_id WHERE p.id=$1)
			  AND unit_level='directorate' AND is_active`, creatorPositionID).Scan(&totalDirectorates)
		if err != nil {
			return "", errors.New("gagal menghitung cakupan direktorat aktif")
		}
		scopeRank = promoteCorporateScope(scopeRank, len(targetDirectorates), totalDirectorates)
	}
	return []string{"same_unit", "cross_department", "cross_biro", "cross_directorate", "corporate"}[scopeRank], nil
}

func resolveSameUnitFinalLevel(ctx context.Context, db policyQueryer, creatorPositionID, fallback string) (string, error) {
	var positionType, unitLevel string
	err := db.QueryRow(ctx, `
		SELECT p.position_type, ou.unit_level
		FROM positions p JOIN org_units ou ON ou.id=p.org_unit_id
		WHERE p.id=$1 AND p.is_active AND ou.is_active`, creatorPositionID).Scan(&positionType, &unitLevel)
	if err != nil {
		return "", errors.New("jabatan pembuat tidak aktif")
	}
	return sameUnitFinalLevel(positionType, unitLevel, fallback), nil
}

func sameUnitFinalLevel(positionType, unitLevel, fallback string) string {
	if unitLevel == "division" && approvalLevelRank[positionType] < approvalLevelRank["division_head"] {
		return "division_head"
	}
	return fallback
}

func promoteCorporateScope(scopeRank, targetedDirectorates, totalDirectorates int) int {
	if scopeRank == 3 && totalDirectorates > 0 && targetedDirectorates >= totalDirectorates {
		return 4
	}
	return scopeRank
}

func loadPositionAncestry(ctx context.Context, db policyQueryer, id, targetType string) (orgAncestry, error) {
	unitExpr := `$1::uuid`
	if targetType == "position" {
		unitExpr = `(SELECT org_unit_id FROM positions WHERE id=$1 AND is_active)`
	}
	var item orgAncestry
	err := db.QueryRow(ctx, `WITH RECURSIVE chain AS (
		SELECT id,parent_id,unit_level FROM org_units WHERE id=`+unitExpr+` AND is_active
		UNION ALL SELECT p.id,p.parent_id,p.unit_level FROM org_units p JOIN chain c ON c.parent_id=p.id WHERE p.is_active)
		SELECT `+unitExpr+`::text,
		COALESCE(max(id::text) FILTER(WHERE unit_level='department'),''),
		COALESCE(max(id::text) FILTER(WHERE unit_level='biro'),''),
		COALESCE(max(id::text) FILTER(WHERE unit_level='directorate'),''),
		COALESCE(max(unit_level) FILTER(WHERE id=`+unitExpr+`),'') FROM chain`, id).Scan(
		&item.unit, &item.department, &item.biro, &item.directorate, &item.level)
	return item, err
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
