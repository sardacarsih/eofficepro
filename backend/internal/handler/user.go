package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/kskgroup/eofficepro/internal/auth"
	"github.com/kskgroup/eofficepro/internal/middleware"
)

func (h *Handler) ListUsers(c *gin.Context) {
	page, pageSize, offset, ok := parsePagination(c.Query("page"), c.Query("page_size"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page atau page_size tidak valid"})
		return
	}
	ctx := c.Request.Context()
	actor := c.GetString(middleware.CtxUserID)
	isSuperAdmin, err := h.userIsSuperAdmin(ctx, actor)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa akses administrator"})
		return
	}
	assignments, err := h.companyRoles(ctx, actor)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa akses perusahaan"})
		return
	}
	companyIDs := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		if assignment.RoleCode == "admin" {
			companyIDs = append(companyIDs, assignment.CompanyID)
		}
	}
	userScopeSQL := `(
		$1 OR EXISTS (
			SELECT 1 FROM user_positions scope_up
			JOIN positions scope_p ON scope_p.id = scope_up.position_id
			JOIN org_units scope_ou ON scope_ou.id = scope_p.org_unit_id
			WHERE scope_up.user_id = u.id AND scope_ou.company_id::text = ANY($2::text[])
			  AND current_date >= scope_up.valid_from
			  AND (scope_up.valid_to IS NULL OR current_date < scope_up.valid_to)
		) OR EXISTS (
			SELECT 1 FROM user_company_roles scope_ucr
			WHERE scope_ucr.user_id = u.id AND scope_ucr.company_id::text = ANY($2::text[])
			  AND current_date >= scope_ucr.valid_from
			  AND (scope_ucr.valid_to IS NULL OR current_date < scope_ucr.valid_to)
		))`

	var total int64
	if err := h.DB.QueryRow(ctx, `SELECT count(*) FROM users u WHERE `+userScopeSQL, isSuperAdmin, companyIDs).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menghitung pengguna"})
		return
	}

	rows, err := h.DB.Query(ctx, `
		SELECT u.id::text, u.nik, u.email, u.full_name, u.status,
		       COALESCE(user_roles.roles, '{}'::text[]),
		       COALESCE(user_positions.positions, '[]'::jsonb),
		       COALESCE(company_roles.assignments, '[]'::jsonb)
		FROM users u
		LEFT JOIN LATERAL (
			SELECT array_agg(DISTINCT r.code ORDER BY r.code) AS roles
			FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			WHERE ur.user_id = u.id
		) user_roles ON true
		LEFT JOIN LATERAL (
			SELECT jsonb_agg(
				jsonb_build_object(
					'assignment_id', up.id::text,
					'position_id', p.id::text,
					'title', p.title,
					'position_type', p.position_type,
					'org_unit_name', ou.name,
					'company_id', company.id::text,
					'company_code', company.code,
					'company_name', company.name,
					'assignment_type', up.assignment_type,
					'valid_from', up.valid_from::text,
					'valid_to', up.valid_to::text
				)
				ORDER BY ou.name, p.title, up.valid_from DESC
			) AS positions
			FROM user_positions up
			JOIN positions p ON p.id = up.position_id
			JOIN org_units ou ON ou.id = p.org_unit_id
			JOIN companies company ON company.id = ou.company_id
			WHERE up.user_id = u.id
			  AND ($1 OR company.id::text = ANY($2::text[]))
			  AND current_date >= up.valid_from
			  AND (up.valid_to IS NULL OR current_date < up.valid_to)
			  AND p.is_active
		) user_positions ON true
		LEFT JOIN LATERAL (
			SELECT jsonb_agg(jsonb_build_object(
				'company_id', company.id::text,
				'company_code', company.code,
				'company_name', company.name,
				'role_code', r.code,
				'valid_from', ucr.valid_from::text,
				'valid_to', ucr.valid_to::text
			) ORDER BY company.code, r.code) AS assignments
			FROM user_company_roles ucr
			JOIN companies company ON company.id = ucr.company_id
			JOIN roles r ON r.id = ucr.role_id
			WHERE ucr.user_id = u.id
			  AND ($1 OR company.id::text = ANY($2::text[]))
			  AND (ucr.valid_to IS NULL OR current_date < ucr.valid_to)
		) company_roles ON true
		WHERE `+userScopeSQL+`
		ORDER BY u.full_name
		LIMIT $3 OFFSET $4`, isSuperAdmin, companyIDs, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat pengguna"})
		return
	}
	defer rows.Close()

	type user struct {
		ID           string                   `json:"id"`
		NIK          string                   `json:"nik"`
		Email        string                   `json:"email"`
		FullName     string                   `json:"full_name"`
		Status       string                   `json:"status"`
		Roles        []string                 `json:"roles"`
		Positions    []userPositionAssignment `json:"positions"`
		CompanyRoles []companyRoleAssignment  `json:"company_roles"`
	}
	users := []user{}
	for rows.Next() {
		var u user
		var positionsJSON, companyRolesJSON []byte
		if err := rows.Scan(&u.ID, &u.NIK, &u.Email, &u.FullName, &u.Status, &u.Roles, &positionsJSON, &companyRolesJSON); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca data pengguna"})
			return
		}
		if err := json.Unmarshal(positionsJSON, &u.Positions); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca jabatan pengguna"})
			return
		}
		if err := json.Unmarshal(companyRolesJSON, &u.CompanyRoles); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca role perusahaan pengguna"})
			return
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca data pengguna"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": users, "meta": newPageMeta(page, pageSize, total)})
}

type userPositionAssignment struct {
	AssignmentID string  `json:"assignment_id"`
	PositionID   string  `json:"position_id"`
	Title        string  `json:"title"`
	PositionType string  `json:"position_type"`
	OrgUnitName  string  `json:"org_unit_name"`
	CompanyID    string  `json:"company_id"`
	CompanyCode  string  `json:"company_code"`
	CompanyName  string  `json:"company_name"`
	Assignment   string  `json:"assignment_type"`
	ValidFrom    string  `json:"valid_from"`
	ValidTo      *string `json:"valid_to"`
}

type userPositionPayload struct {
	PositionID     string `json:"position_id"`
	AssignmentType string `json:"assignment_type"`
}

type userCompanyRolePayload struct {
	CompanyID string  `json:"company_id"`
	RoleCode  string  `json:"role_code"`
	ValidFrom string  `json:"valid_from"`
	ValidTo   *string `json:"valid_to"`
}

type createUserRequest struct {
	NIK          string                    `json:"nik" binding:"required"`
	Email        string                    `json:"email" binding:"required,email"`
	FullName     string                    `json:"full_name" binding:"required"`
	Password     string                    `json:"password" binding:"required,min=10"`
	Roles        []string                  `json:"roles" binding:"required,min=1"`
	Status       string                    `json:"status"`
	Positions    []userPositionPayload     `json:"positions"`
	CompanyRoles *[]userCompanyRolePayload `json:"company_roles"`
}

func (h *Handler) CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data pengguna tidak valid (password minimal 10 karakter): " + err.Error()})
		return
	}
	roles := normalizeRoles(req.Roles)
	if len(roles) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "minimal satu role wajib diisi"})
		return
	}
	if req.Status == "" {
		req.Status = "active"
	}
	if !validUserStatus(req.Status) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status pengguna tidak valid"})
		return
	}
	positions, err := normalizeUserPositionPayloads(req.Positions)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if hasRole(roles, "creator") && len(positions) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role creator wajib memiliki minimal satu jabatan aktif"})
		return
	}
	companyRoles := []userCompanyRolePayload{}
	if req.CompanyRoles != nil {
		companyRoles, err = normalizeUserCompanyRoles(*req.CompanyRoles)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	ctx := c.Request.Context()
	if !h.validateUserAdministrationScope(c, "", roles, positions, companyRoles) {
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memproses password"})
		return
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi"})
		return
	}
	defer tx.Rollback(ctx)

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (nik, email, full_name, password_hash, status)
		VALUES ($1, $2, $3, $4, $5) RETURNING id::text`,
		req.NIK, req.Email, req.FullName, hash, req.Status).Scan(&id)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "NIK atau email sudah terdaftar"})
		return
	}

	for _, role := range roles {
		tag, err := tx.Exec(ctx, `
			INSERT INTO user_roles (user_id, role_id)
			SELECT $1, id FROM roles WHERE code = $2`, id, role)
		if err != nil || tag.RowsAffected() == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "role tidak dikenal: " + role})
			return
		}
	}
	if err := syncUserPositions(ctx, tx, id, positions); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := syncUserCompanyRoles(ctx, tx, id, c.GetString(middleware.CtxUserID), companyRoles); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan pengguna"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "user", &id, "create", &actor, map[string]any{"nik": req.NIK, "roles": roles, "company_roles": companyRoles, "status": req.Status}, c.ClientIP())
	if len(companyRoles) > 0 {
		h.audit(ctx, "user_company_role", &id, "sync", &actor, map[string]any{"assignments": companyRoles}, c.ClientIP())
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

type updateUserRequest struct {
	NIK          string                    `json:"nik" binding:"required"`
	Email        string                    `json:"email" binding:"required,email"`
	FullName     string                    `json:"full_name" binding:"required"`
	Password     string                    `json:"password"`
	Roles        []string                  `json:"roles" binding:"required,min=1"`
	Status       string                    `json:"status" binding:"required,oneof=active inactive locked"`
	Positions    []userPositionPayload     `json:"positions"`
	CompanyRoles *[]userCompanyRolePayload `json:"company_roles"`
}

func (h *Handler) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	actor := c.GetString(middleware.CtxUserID)

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data pengguna tidak valid: " + err.Error()})
		return
	}
	roles := normalizeRoles(req.Roles)
	if len(roles) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "minimal satu role wajib diisi"})
		return
	}
	if req.Password != "" && len(req.Password) < 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password minimal 10 karakter"})
		return
	}
	positions, err := normalizeUserPositionPayloads(req.Positions)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if hasRole(roles, "creator") && len(positions) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role creator wajib memiliki minimal satu jabatan aktif"})
		return
	}
	companyRoles := []userCompanyRolePayload{}
	if req.CompanyRoles != nil {
		companyRoles, err = normalizeUserCompanyRoles(*req.CompanyRoles)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if !h.validateUserAdministrationScope(c, id, roles, positions, companyRoles) {
		return
	}
	if id == actor && req.Status != "active" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tidak bisa menonaktifkan akun sendiri"})
		return
	}
	if id == actor && !hasRole(roles, "super_admin") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tidak bisa menghapus role super admin dari akun sendiri"})
		return
	}

	ctx := c.Request.Context()
	var oldStatus string
	if err := h.DB.QueryRow(ctx, `SELECT status FROM users WHERE id = $1`, id).Scan(&oldStatus); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "pengguna tidak ditemukan"})
		return
	}

	var passwordHash string
	if req.Password != "" {
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memproses password"})
			return
		}
		passwordHash = hash
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi"})
		return
	}
	defer tx.Rollback(ctx)

	var tag interface{ RowsAffected() int64 }
	if passwordHash != "" {
		tag, err = tx.Exec(ctx, `
			UPDATE users SET
				nik = $2, email = $3, full_name = $4, status = $5,
				password_hash = $6, failed_login_count = 0, locked_until = NULL, updated_at = now()
			WHERE id = $1`, id, req.NIK, req.Email, req.FullName, req.Status, passwordHash)
	} else if req.Status == "active" {
		tag, err = tx.Exec(ctx, `
			UPDATE users SET
				nik = $2, email = $3, full_name = $4, status = $5,
				failed_login_count = 0, locked_until = NULL, updated_at = now()
			WHERE id = $1`, id, req.NIK, req.Email, req.FullName, req.Status)
	} else {
		tag, err = tx.Exec(ctx, `
			UPDATE users SET nik = $2, email = $3, full_name = $4, status = $5, updated_at = now()
			WHERE id = $1`, id, req.NIK, req.Email, req.FullName, req.Status)
	}
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "NIK atau email sudah terdaftar"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "pengguna tidak ditemukan"})
		return
	}

	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memperbarui role"})
		return
	}
	for _, role := range roles {
		tag, err := tx.Exec(ctx, `
			INSERT INTO user_roles (user_id, role_id)
			SELECT $1, id FROM roles WHERE code = $2`, id, role)
		if err != nil || tag.RowsAffected() == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "role tidak dikenal: " + role})
			return
		}
	}
	if err := syncUserPositions(ctx, tx, id, positions); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.CompanyRoles != nil {
		if err := syncUserCompanyRoles(ctx, tx, id, actor, companyRoles); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan pengguna"})
		return
	}

	action := "update"
	if oldStatus != "active" && req.Status == "active" {
		action = "reactivate"
	} else if oldStatus == "active" && req.Status != "active" {
		action = "deactivate"
	}
	h.audit(ctx, "user", &id, action, &actor, map[string]any{
		"nik":              req.NIK,
		"roles":            roles,
		"status":           req.Status,
		"password_changed": passwordHash != "",
		"company_roles":    companyRoles,
	}, c.ClientIP())
	if req.CompanyRoles != nil {
		h.audit(ctx, "user_company_role", &id, "sync", &actor, map[string]any{"assignments": companyRoles}, c.ClientIP())
	}
	c.JSON(http.StatusOK, gin.H{"id": id})
}

type deactivationPositionImpact struct {
	PositionID     string `json:"position_id"`
	Title          string `json:"title"`
	OrgUnitName    string `json:"org_unit_name"`
	AssignmentType string `json:"assignment_type"`
}

type deactivationDraftImpact struct {
	LetterID             string `json:"letter_id"`
	Subject              string `json:"subject"`
	CreatorPositionID    string `json:"creator_position_id"`
	CreatorPositionTitle string `json:"creator_position_title"`
	Status               string `json:"status"`
}

type deactivationApprovalImpact struct {
	StepID        string `json:"step_id"`
	LetterID      string `json:"letter_id"`
	Subject       string `json:"subject"`
	PositionID    string `json:"position_id"`
	PositionTitle string `json:"position_title"`
	Status        string `json:"status"`
}

type deactivationImpact struct {
	Positions     []deactivationPositionImpact `json:"positions"`
	Drafts        []deactivationDraftImpact    `json:"drafts"`
	ApprovalSteps []deactivationApprovalImpact `json:"approval_steps"`
	HasImpact     bool                         `json:"has_impact"`
}

type positionReplacementRequest struct {
	PositionID        string `json:"position_id"`
	ReplacementUserID string `json:"replacement_user_id"`
	AssignmentType    string `json:"assignment_type"`
}

type draftTransferRequest struct {
	LetterID              string `json:"letter_id"`
	ReplacementUserID     string `json:"replacement_user_id"`
	ReplacementPositionID string `json:"replacement_position_id"`
}

type deactivateUserRequest struct {
	PositionReplacements []positionReplacementRequest `json:"position_replacements"`
	DraftTransfers       []draftTransferRequest       `json:"draft_transfers"`
}

func (h *Handler) DeactivationImpact(c *gin.Context) {
	id := c.Param("id")
	if !h.validateUserAdministrationScope(c, id, nil, nil, nil) {
		return
	}
	ctx := c.Request.Context()

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi"})
		return
	}
	defer tx.Rollback(ctx)

	impact, err := loadDeactivationImpact(ctx, tx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "pengguna tidak ditemukan"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca dampak nonaktif pengguna"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"impact": impact})
}

// DeactivateUser menonaktifkan pengguna. Bila ada jabatan kosong atau surat
// draft/revisi, request harus menyertakan pengganti eksplisit.
func (h *Handler) DeactivateUser(c *gin.Context) {
	id := c.Param("id")
	if !h.validateUserAdministrationScope(c, id, nil, nil, nil) {
		return
	}
	actor := c.GetString(middleware.CtxUserID)
	if id == actor {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tidak bisa menonaktifkan akun sendiri"})
		return
	}
	ctx := c.Request.Context()

	var req deactivateUserRequest
	if c.Request.Method == http.MethodPost && c.Request.Body != nil {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "data pengalihan tidak valid: " + err.Error()})
			return
		}
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi"})
		return
	}
	defer tx.Rollback(ctx)

	impact, err := loadDeactivationImpact(ctx, tx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "pengguna tidak ditemukan atau sudah nonaktif"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca dampak nonaktif pengguna"})
		return
	}
	if err := applyDeactivationPlan(ctx, tx, id, impact, req); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "impact": impact})
		return
	}
	tag, err := tx.Exec(ctx, `
		UPDATE users
		SET status = 'inactive', updated_at = now()
		WHERE id = $1 AND status <> 'inactive'`, id)
	if err != nil || tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "pengguna tidak ditemukan atau sudah nonaktif"})
		return
	}
	if _, err := tx.Exec(ctx, `
		UPDATE user_positions
		SET valid_to = current_date
		WHERE user_id = $1
		  AND current_date >= valid_from
		  AND (valid_to IS NULL OR current_date < valid_to)`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengakhiri jabatan pengguna"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menonaktifkan pengguna"})
		return
	}

	h.audit(ctx, "user", &id, "deactivate", &actor, nil, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
}

func normalizeUserPositionPayloads(input []userPositionPayload) ([]userPositionPayload, error) {
	seen := map[string]bool{}
	positions := []userPositionPayload{}
	for _, position := range input {
		position.PositionID = strings.TrimSpace(position.PositionID)
		position.AssignmentType = strings.ToLower(strings.TrimSpace(position.AssignmentType))
		if position.PositionID == "" {
			return nil, errors.New("jabatan wajib dipilih")
		}
		if position.AssignmentType == "" {
			position.AssignmentType = "definitive"
		}
		if position.AssignmentType != "definitive" && position.AssignmentType != "plt" && position.AssignmentType != "plh" {
			return nil, errors.New("tipe penempatan jabatan tidak valid")
		}
		if seen[position.PositionID] {
			return nil, errors.New("jabatan pengguna tidak boleh duplikat")
		}
		seen[position.PositionID] = true
		positions = append(positions, position)
	}
	return positions, nil
}

func normalizeUserCompanyRoles(input []userCompanyRolePayload) ([]userCompanyRolePayload, error) {
	seen := map[string]bool{}
	result := make([]userCompanyRolePayload, 0, len(input))
	for _, assignment := range input {
		assignment.CompanyID = strings.TrimSpace(assignment.CompanyID)
		assignment.RoleCode = strings.ToLower(strings.TrimSpace(assignment.RoleCode))
		if assignment.CompanyID == "" {
			return nil, errors.New("perusahaan assignment admin wajib dipilih")
		}
		if assignment.RoleCode == "" {
			assignment.RoleCode = "admin"
		}
		if assignment.RoleCode != "admin" {
			return nil, errors.New("role perusahaan tidak dikenal: " + assignment.RoleCode)
		}
		if assignment.ValidFrom == "" {
			assignment.ValidFrom = time.Now().Format(time.DateOnly)
		}
		validFrom, err := time.Parse(time.DateOnly, assignment.ValidFrom)
		if err != nil {
			return nil, errors.New("tanggal mulai assignment perusahaan tidak valid")
		}
		if assignment.ValidTo != nil {
			trimmed := strings.TrimSpace(*assignment.ValidTo)
			if trimmed == "" {
				assignment.ValidTo = nil
			} else {
				validTo, err := time.Parse(time.DateOnly, trimmed)
				if err != nil || !validTo.After(validFrom) {
					return nil, errors.New("tanggal akhir assignment perusahaan harus setelah tanggal mulai")
				}
				assignment.ValidTo = &trimmed
			}
		}
		key := assignment.CompanyID + ":" + assignment.RoleCode
		if seen[key] {
			return nil, errors.New("assignment role perusahaan tidak boleh duplikat")
		}
		seen[key] = true
		result = append(result, assignment)
	}
	return result, nil
}

func (h *Handler) validateUserAdministrationScope(c *gin.Context, targetUserID string, roles []string, positions []userPositionPayload, companyRoles []userCompanyRolePayload) bool {
	ctx := c.Request.Context()
	actor := c.GetString(middleware.CtxUserID)
	isSuperAdmin, err := h.userIsSuperAdmin(ctx, actor)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa akses administrator"})
		return false
	}
	if isSuperAdmin {
		return true
	}
	if hasRole(roles, "admin") || hasRole(roles, "super_admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "company admin tidak dapat memberikan role global admin"})
		return false
	}
	if targetUserID != "" {
		var targetPrivileged bool
		if err := h.DB.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id
			WHERE ur.user_id = $1 AND r.code IN ('admin', 'super_admin'))`, targetUserID).Scan(&targetPrivileged); err != nil || targetPrivileged {
			c.JSON(http.StatusForbidden, gin.H{"error": "company admin tidak dapat mengelola administrator global"})
			return false
		}
		var outsideScope bool
		err := h.DB.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM (
					SELECT ou.company_id FROM user_positions up
					JOIN positions p ON p.id = up.position_id JOIN org_units ou ON ou.id = p.org_unit_id
					WHERE up.user_id = $2 AND current_date >= up.valid_from
					  AND (up.valid_to IS NULL OR current_date < up.valid_to)
					UNION
					SELECT company_id FROM user_company_roles
					WHERE user_id = $2 AND current_date >= valid_from
					  AND (valid_to IS NULL OR current_date < valid_to)
				) target_scope
				WHERE NOT EXISTS (
					SELECT 1 FROM user_company_roles actor_scope JOIN roles r ON r.id = actor_scope.role_id
					WHERE actor_scope.user_id = $1 AND actor_scope.company_id = target_scope.company_id
					  AND r.code = 'admin' AND current_date >= actor_scope.valid_from
					  AND (actor_scope.valid_to IS NULL OR current_date < actor_scope.valid_to)
				)
			)`, actor, targetUserID).Scan(&outsideScope)
		if err != nil || outsideScope {
			c.JSON(http.StatusForbidden, gin.H{"error": "pengguna memiliki assignment di luar cakupan administrasi Anda"})
			return false
		}
	}
	for _, position := range positions {
		var companyID string
		if err := h.DB.QueryRow(ctx, `SELECT ou.company_id::text FROM positions p JOIN org_units ou ON ou.id = p.org_unit_id WHERE p.id = $1`, position.PositionID).Scan(&companyID); err != nil || !h.requireAdminCompany(c, companyID) {
			return false
		}
	}
	for _, assignment := range companyRoles {
		if !h.requireAdminCompany(c, assignment.CompanyID) {
			return false
		}
	}
	return true
}

func syncUserCompanyRoles(ctx context.Context, tx pgx.Tx, userID string, actorID string, assignments []userCompanyRolePayload) error {
	if _, err := tx.Exec(ctx, `
		UPDATE user_company_roles SET valid_to = current_date
		WHERE user_id = $1 AND current_date >= valid_from
		  AND (valid_to IS NULL OR current_date < valid_to)`, userID); err != nil {
		return errors.New("gagal mengakhiri assignment perusahaan lama")
	}
	for _, assignment := range assignments {
		tag, err := tx.Exec(ctx, `
		INSERT INTO user_company_roles (user_id, company_id, role_id, valid_from, valid_to, created_by)
		SELECT $1, c.id, r.id, $4::date, $5::date, $6
		FROM companies c CROSS JOIN roles r
		WHERE c.id = $2 AND c.is_active AND r.code = $3
		ON CONFLICT (user_id, company_id, role_id, valid_from)
		DO UPDATE SET valid_to = EXCLUDED.valid_to, created_by = EXCLUDED.created_by`,
			userID, assignment.CompanyID, assignment.RoleCode, assignment.ValidFrom, assignment.ValidTo, actorID)
		if err != nil || tag.RowsAffected() == 0 {
			return errors.New("perusahaan atau role assignment tidak valid")
		}
	}
	return nil
}

func syncUserPositions(ctx context.Context, tx pgx.Tx, userID string, positions []userPositionPayload) error {
	desired := map[string]string{}
	for _, position := range positions {
		desired[position.PositionID] = position.AssignmentType
	}

	for positionID := range desired {
		var exists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM positions WHERE id = $1 AND is_active
			)`, positionID).Scan(&exists); err != nil {
			return errors.New("gagal memvalidasi jabatan")
		}
		if !exists {
			return errors.New("jabatan tidak ditemukan atau tidak aktif")
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE user_positions
		SET valid_to = current_date
		WHERE user_id = $1
		  AND current_date >= valid_from
		  AND (valid_to IS NULL OR current_date < valid_to)
		  AND NOT (position_id::text = ANY($2::text[]))`,
		userID, mapKeys(desired)); err != nil {
		return errors.New("gagal mengakhiri jabatan lama")
	}

	for _, position := range positions {
		if position.AssignmentType == "definitive" {
			if _, err := tx.Exec(ctx, `
				UPDATE user_positions
				SET valid_to = current_date
				WHERE position_id = $1
				  AND user_id <> $2
				  AND assignment_type = 'definitive'
				  AND current_date >= valid_from
				  AND (valid_to IS NULL OR current_date < valid_to)`,
				position.PositionID, userID); err != nil {
				return errors.New("gagal menutup pemegang definitif lama")
			}
		}

		tag, err := tx.Exec(ctx, `
			UPDATE user_positions
			SET assignment_type = $3, valid_to = NULL
			WHERE user_id = $1
			  AND position_id = $2
			  AND current_date >= valid_from
			  AND (valid_to IS NULL OR current_date < valid_to)`,
			userID, position.PositionID, position.AssignmentType)
		if err != nil {
			return errors.New("gagal memperbarui jabatan pengguna")
		}
		if tag.RowsAffected() > 0 {
			continue
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO user_positions (user_id, position_id, assignment_type)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_id, position_id, valid_from)
			DO UPDATE SET assignment_type = EXCLUDED.assignment_type, valid_to = NULL`,
			userID, position.PositionID, position.AssignmentType); err != nil {
			return errors.New("gagal menempatkan pengguna pada jabatan")
		}
	}
	return nil
}

func mapKeys(input map[string]string) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	return keys
}

func loadDeactivationImpact(ctx context.Context, tx pgx.Tx, userID string) (deactivationImpact, error) {
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM users WHERE id = $1`, userID).Scan(&status); err != nil {
		return deactivationImpact{}, err
	}
	_ = status

	impact := deactivationImpact{}
	rows, err := tx.Query(ctx, `
		SELECT up.position_id::text, p.title, ou.name, up.assignment_type
		FROM user_positions up
		JOIN positions p ON p.id = up.position_id
		JOIN org_units ou ON ou.id = p.org_unit_id
		WHERE up.user_id = $1
		  AND current_date >= up.valid_from
		  AND (up.valid_to IS NULL OR current_date < up.valid_to)
		  AND NOT EXISTS (
			  SELECT 1
			  FROM user_positions other_up
			  JOIN users other_u ON other_u.id = other_up.user_id
			  WHERE other_up.position_id = up.position_id
			    AND other_up.user_id <> up.user_id
			    AND current_date >= other_up.valid_from
			    AND (other_up.valid_to IS NULL OR current_date < other_up.valid_to)
			    AND other_u.status = 'active'
		  )
		ORDER BY ou.name, p.title`, userID)
	if err != nil {
		return deactivationImpact{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item deactivationPositionImpact
		if err := rows.Scan(&item.PositionID, &item.Title, &item.OrgUnitName, &item.AssignmentType); err != nil {
			return deactivationImpact{}, err
		}
		impact.Positions = append(impact.Positions, item)
	}
	if err := rows.Err(); err != nil {
		return deactivationImpact{}, err
	}

	rows, err = tx.Query(ctx, `
		SELECT l.id::text, l.subject, l.creator_position_id::text, p.title, l.status
		FROM letters l
		JOIN positions p ON p.id = l.creator_position_id
		WHERE l.creator_user_id = $1
		  AND l.status IN ('draft', 'revision')
		ORDER BY l.updated_at DESC`, userID)
	if err != nil {
		return deactivationImpact{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item deactivationDraftImpact
		if err := rows.Scan(&item.LetterID, &item.Subject, &item.CreatorPositionID, &item.CreatorPositionTitle, &item.Status); err != nil {
			return deactivationImpact{}, err
		}
		impact.Drafts = append(impact.Drafts, item)
	}
	if err := rows.Err(); err != nil {
		return deactivationImpact{}, err
	}

	rows, err = tx.Query(ctx, `
		SELECT s.id::text, s.letter_id::text, l.subject, s.approver_position_id::text, p.title, s.status
		FROM approval_steps s
		JOIN letters l ON l.id = s.letter_id
		JOIN positions p ON p.id = s.approver_position_id
		WHERE l.status = 'in_approval'
		  AND s.status IN ('pending', 'waiting')
		  AND s.approver_position_id IN (
			  SELECT up.position_id
			  FROM user_positions up
			  WHERE up.user_id = $1
			    AND current_date >= up.valid_from
			    AND (up.valid_to IS NULL OR current_date < up.valid_to)
			    AND NOT EXISTS (
				    SELECT 1
				    FROM user_positions other_up
				    JOIN users other_u ON other_u.id = other_up.user_id
				    WHERE other_up.position_id = up.position_id
				      AND other_up.user_id <> up.user_id
				      AND current_date >= other_up.valid_from
				      AND (other_up.valid_to IS NULL OR current_date < other_up.valid_to)
				      AND other_u.status = 'active'
			    )
		  )
		ORDER BY l.updated_at DESC, s.step_order`, userID)
	if err != nil {
		return deactivationImpact{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item deactivationApprovalImpact
		if err := rows.Scan(&item.StepID, &item.LetterID, &item.Subject, &item.PositionID, &item.PositionTitle, &item.Status); err != nil {
			return deactivationImpact{}, err
		}
		impact.ApprovalSteps = append(impact.ApprovalSteps, item)
	}
	if err := rows.Err(); err != nil {
		return deactivationImpact{}, err
	}

	impact.HasImpact = len(impact.Positions) > 0 || len(impact.Drafts) > 0 || len(impact.ApprovalSteps) > 0
	return impact, nil
}

func applyDeactivationPlan(ctx context.Context, tx pgx.Tx, userID string, impact deactivationImpact, req deactivateUserRequest) error {
	positionReplacements := map[string]positionReplacementRequest{}
	for _, replacement := range req.PositionReplacements {
		replacement.PositionID = strings.TrimSpace(replacement.PositionID)
		replacement.ReplacementUserID = strings.TrimSpace(replacement.ReplacementUserID)
		replacement.AssignmentType = strings.ToLower(strings.TrimSpace(replacement.AssignmentType))
		if replacement.AssignmentType == "" {
			replacement.AssignmentType = "definitive"
		}
		positionReplacements[replacement.PositionID] = replacement
	}
	for _, position := range impact.Positions {
		replacement, ok := positionReplacements[position.PositionID]
		if !ok || replacement.ReplacementUserID == "" {
			return errors.New("pilih pengganti untuk semua jabatan aktif pengguna")
		}
		if replacement.ReplacementUserID == userID {
			return errors.New("pengganti jabatan tidak boleh pengguna yang dinonaktifkan")
		}
		if _, err := normalizeUserPositionPayloads([]userPositionPayload{{
			PositionID:     replacement.PositionID,
			AssignmentType: replacement.AssignmentType,
		}}); err != nil {
			return err
		}
		if err := ensureReplacementAssignment(ctx, tx, replacement.ReplacementUserID, replacement.PositionID, replacement.AssignmentType); err != nil {
			return err
		}
	}

	draftTransfers := map[string]draftTransferRequest{}
	for _, transfer := range req.DraftTransfers {
		transfer.LetterID = strings.TrimSpace(transfer.LetterID)
		transfer.ReplacementUserID = strings.TrimSpace(transfer.ReplacementUserID)
		transfer.ReplacementPositionID = strings.TrimSpace(transfer.ReplacementPositionID)
		draftTransfers[transfer.LetterID] = transfer
	}
	for _, draft := range impact.Drafts {
		transfer, ok := draftTransfers[draft.LetterID]
		if !ok || transfer.ReplacementUserID == "" || transfer.ReplacementPositionID == "" {
			return errors.New("pilih pengganti untuk semua draft atau revisi pengguna")
		}
		if transfer.ReplacementUserID == userID {
			return errors.New("pengganti draft tidak boleh pengguna yang dinonaktifkan")
		}
		if ok, err := userCanUsePositionTx(ctx, tx, transfer.ReplacementUserID, transfer.ReplacementPositionID); err != nil || !ok {
			return errors.New("jabatan pengganti draft tidak valid untuk pengguna pengganti")
		}
		tag, err := tx.Exec(ctx, `
			UPDATE letters
			SET creator_user_id = $2,
			    creator_position_id = $3,
			    updated_at = now()
			WHERE id = $1
			  AND creator_user_id = $4
			  AND status IN ('draft', 'revision')`,
			transfer.LetterID, transfer.ReplacementUserID, transfer.ReplacementPositionID, userID)
		if err != nil || tag.RowsAffected() == 0 {
			return errors.New("gagal mengalihkan draft pengguna")
		}
	}

	if len(impact.ApprovalSteps) > 0 && len(impact.Positions) == 0 {
		return errors.New("approval pending membutuhkan pengganti jabatan")
	}
	return nil
}

func ensureReplacementAssignment(ctx context.Context, tx pgx.Tx, userID string, positionID string, assignmentType string) error {
	var active bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND status = 'active')`, userID).Scan(&active); err != nil {
		return errors.New("gagal memvalidasi pengguna pengganti")
	}
	if !active {
		return errors.New("pengguna pengganti tidak aktif")
	}
	var positionActive bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM positions WHERE id = $1 AND is_active)`, positionID).Scan(&positionActive); err != nil {
		return errors.New("gagal memvalidasi jabatan pengganti")
	}
	if !positionActive {
		return errors.New("jabatan pengganti tidak aktif")
	}
	if assignmentType == "definitive" {
		if _, err := tx.Exec(ctx, `
			UPDATE user_positions
			SET valid_to = current_date
			WHERE position_id = $1
			  AND user_id <> $2
			  AND assignment_type = 'definitive'
			  AND current_date >= valid_from
			  AND (valid_to IS NULL OR current_date < valid_to)`,
			positionID, userID); err != nil {
			return errors.New("gagal menutup pemegang definitif lama")
		}
	}
	tag, err := tx.Exec(ctx, `
		UPDATE user_positions
		SET assignment_type = $3, valid_to = NULL
		WHERE user_id = $1
		  AND position_id = $2
		  AND current_date >= valid_from
		  AND (valid_to IS NULL OR current_date < valid_to)`,
		userID, positionID, assignmentType)
	if err != nil {
		return errors.New("gagal memperbarui jabatan pengganti")
	}
	if tag.RowsAffected() > 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_positions (user_id, position_id, assignment_type)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, position_id, valid_from)
		DO UPDATE SET assignment_type = EXCLUDED.assignment_type, valid_to = NULL`,
		userID, positionID, assignmentType); err != nil {
		return errors.New("gagal menempatkan pengganti pada jabatan")
	}
	return nil
}

func userCanUsePositionTx(ctx context.Context, tx pgx.Tx, userID string, positionID string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
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

func normalizeRoles(input []string) []string {
	seen := map[string]bool{}
	roles := []string{}
	for _, role := range input {
		role = strings.TrimSpace(role)
		if role == "" || seen[role] {
			continue
		}
		seen[role] = true
		roles = append(roles, role)
	}
	return roles
}

func hasRole(roles []string, want string) bool {
	for _, role := range roles {
		if role == want {
			return true
		}
	}
	return false
}

func validUserStatus(status string) bool {
	return status == "active" || status == "inactive" || status == "locked"
}
