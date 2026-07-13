package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

type accessibleCompany struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

type companyRoleAssignment struct {
	CompanyID   string  `json:"company_id"`
	CompanyCode string  `json:"company_code"`
	CompanyName string  `json:"company_name"`
	RoleCode    string  `json:"role_code"`
	ValidFrom   string  `json:"valid_from"`
	ValidTo     *string `json:"valid_to"`
}

func (h *Handler) userIsSuperAdmin(ctx context.Context, userID string) (bool, error) {
	var allowed bool
	err := h.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			WHERE ur.user_id = $1 AND r.code = 'super_admin'
		)`, userID).Scan(&allowed)
	return allowed, err
}

func (h *Handler) canAdminCompany(ctx context.Context, userID string, companyID string) (bool, error) {
	var allowed bool
	err := h.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			WHERE ur.user_id = $1 AND r.code = 'super_admin'
			UNION ALL
			SELECT 1 FROM user_company_roles ucr
			JOIN roles r ON r.id = ucr.role_id
			WHERE ucr.user_id = $1 AND ucr.company_id = $2
			  AND r.code = 'admin'
			  AND current_date >= ucr.valid_from
			  AND (ucr.valid_to IS NULL OR current_date < ucr.valid_to)
		)`, userID, companyID).Scan(&allowed)
	return allowed, err
}

func (h *Handler) accessibleCompanies(ctx context.Context, userID string, includeInactive bool) ([]accessibleCompany, error) {
	rows, err := h.DB.Query(ctx, `
		SELECT DISTINCT company.id::text, company.code, company.name, company.is_active
		FROM companies company
		WHERE ($2 OR company.is_active)
		  AND (
			EXISTS (
				SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id
				WHERE ur.user_id = $1 AND r.code = 'super_admin'
			)
			OR EXISTS (
				SELECT 1 FROM user_positions up
				JOIN positions p ON p.id = up.position_id
				JOIN org_units ou ON ou.id = p.org_unit_id
				WHERE up.user_id = $1 AND ou.company_id = company.id
				  AND current_date >= up.valid_from
				  AND (up.valid_to IS NULL OR current_date < up.valid_to)
				  AND p.is_active AND ou.is_active AND company.is_active
			)
			OR EXISTS (
				SELECT 1 FROM user_company_roles ucr
				WHERE ucr.user_id = $1 AND ucr.company_id = company.id
				  AND current_date >= ucr.valid_from
				  AND (ucr.valid_to IS NULL OR current_date < ucr.valid_to)
			)
		  )
		ORDER BY company.code`, userID, includeInactive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []accessibleCompany{}
	for rows.Next() {
		var company accessibleCompany
		if err := rows.Scan(&company.ID, &company.Code, &company.Name, &company.IsActive); err != nil {
			return nil, err
		}
		result = append(result, company)
	}
	return result, rows.Err()
}

func (h *Handler) companyRoles(ctx context.Context, userID string) ([]companyRoleAssignment, error) {
	rows, err := h.DB.Query(ctx, `
		SELECT c.id::text, c.code, c.name, r.code, ucr.valid_from::text, ucr.valid_to::text
		FROM user_company_roles ucr
		JOIN companies c ON c.id = ucr.company_id
		JOIN roles r ON r.id = ucr.role_id
		WHERE ucr.user_id = $1
		  AND current_date >= ucr.valid_from
		  AND (ucr.valid_to IS NULL OR current_date < ucr.valid_to)
		ORDER BY c.code, r.code`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []companyRoleAssignment{}
	for rows.Next() {
		var item companyRoleAssignment
		if err := rows.Scan(&item.CompanyID, &item.CompanyCode, &item.CompanyName, &item.RoleCode, &item.ValidFrom, &item.ValidTo); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (h *Handler) RequireCompanyAdministrator() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString(middleware.CtxUserID)
		var allowed bool
		err := h.DB.QueryRow(c.Request.Context(), `
			SELECT EXISTS (
				SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id
				WHERE ur.user_id = $1 AND r.code = 'super_admin'
				UNION ALL
				SELECT 1 FROM user_company_roles ucr JOIN roles r ON r.id = ucr.role_id
				WHERE ucr.user_id = $1 AND r.code = 'admin'
				  AND current_date >= ucr.valid_from
				  AND (ucr.valid_to IS NULL OR current_date < ucr.valid_to)
			)`, userID).Scan(&allowed)
		if err != nil || !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "tidak punya akses administrasi perusahaan"})
			return
		}
		c.Next()
	}
}

func (h *Handler) RequireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed, err := h.userIsSuperAdmin(c.Request.Context(), c.GetString(middleware.CtxUserID))
		if err != nil || !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "hanya super admin yang dapat melakukan aksi ini"})
			return
		}
		c.Next()
	}
}

func (h *Handler) requireAdminCompany(c *gin.Context, companyID string) bool {
	allowed, err := h.canAdminCompany(c.Request.Context(), c.GetString(middleware.CtxUserID), companyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa akses perusahaan"})
		return false
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "tidak punya akses ke perusahaan ini"})
		return false
	}
	return true
}

func (h *Handler) requireAdminResourceCompany(c *gin.Context, query string, resourceID string) (string, bool) {
	var companyID string
	if err := h.DB.QueryRow(c.Request.Context(), query, resourceID).Scan(&companyID); err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "data tidak ditemukan"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa perusahaan data"})
		}
		return "", false
	}
	return companyID, h.requireAdminCompany(c, companyID)
}

// userHasPendingApproval reports whether the user currently holds a position
// that has a waiting approval step. Approval authority is determined by the
// workflow assignment, not by a broad job-title role.
func (h *Handler) userHasPendingApproval(ctx context.Context, userID string) (bool, error) {
	var allowed bool
	err := h.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM approval_steps step
			JOIN letters letter ON letter.id = step.letter_id
			JOIN user_positions assignment
			  ON assignment.position_id = step.approver_position_id
			WHERE assignment.user_id = $1
			  AND current_date >= assignment.valid_from
			  AND (assignment.valid_to IS NULL OR current_date < assignment.valid_to)
			  AND step.status = 'waiting'
			  AND letter.status = 'in_approval'
		)`, userID).Scan(&allowed)
	return allowed, err
}

func (h *Handler) userHasAuditExport(ctx context.Context, userID string) (bool, error) {
	var allowed bool
	err := h.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM audit_assignments aa
			JOIN user_roles ur ON ur.user_id = aa.user_id
			JOIN roles r ON r.id = ur.role_id AND r.code = 'auditor'
			WHERE aa.user_id = $1
			  AND aa.can_export
			  AND current_date >= aa.valid_from
			  AND (aa.valid_to IS NULL OR current_date < aa.valid_to)
		)`, userID).Scan(&allowed)
	return allowed, err
}

// auditLetterAccessSQL limits auditor access to an active assignment, its
// organisational subtree, and the assignment's maximum classification.
// letterAlias must be a SQL identifier controlled by the caller.
func auditLetterAccessSQL(userParam string, letterAlias string) string {
	return auditLetterScopeSQL(userParam, letterAlias, "")
}

func auditLetterExportAccessSQL(userParam string, letterAlias string) string {
	return auditLetterScopeSQL(userParam, letterAlias, "AND aa.can_export")
}

func auditLetterScopeSQL(userParam string, letterAlias string, additionalCondition string) string {
	return `EXISTS (
		SELECT 1
		FROM audit_assignments aa
		JOIN user_roles ur ON ur.user_id = aa.user_id
		JOIN roles r ON r.id = ur.role_id AND r.code = 'auditor'
		JOIN positions creator_position ON creator_position.id = ` + letterAlias + `.creator_position_id
		WHERE aa.user_id = ` + userParam + `
		  AND ` + letterAlias + `.status = 'published'
		  ` + additionalCondition + `
		  AND current_date >= aa.valid_from
		  AND (aa.valid_to IS NULL OR current_date < aa.valid_to)
		  AND CASE ` + letterAlias + `.classification
			WHEN 'biasa' THEN 1
			WHEN 'terbatas' THEN 2
			WHEN 'rahasia' THEN 3
			ELSE 0
		  END <= CASE aa.max_classification
			WHEN 'biasa' THEN 1
			WHEN 'terbatas' THEN 2
			WHEN 'rahasia' THEN 3
			ELSE 0
		  END
		  AND EXISTS (
			WITH RECURSIVE scoped_units AS (
				SELECT aa.org_unit_id AS id
				UNION ALL
				SELECT child.id
				FROM org_units child
				JOIN scoped_units parent ON parent.id = child.parent_id
				WHERE child.is_active
			)
			SELECT 1 FROM scoped_units WHERE id = creator_position.org_unit_id
		  )
	)`
}
