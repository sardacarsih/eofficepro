package handler

import "context"

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
