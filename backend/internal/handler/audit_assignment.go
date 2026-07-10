package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

type auditAssignment struct {
	ID                string  `json:"id"`
	UserID            string  `json:"user_id"`
	UserName          string  `json:"user_name"`
	UserEmail         string  `json:"user_email"`
	OrgUnitID         string  `json:"org_unit_id"`
	OrgUnitName       string  `json:"org_unit_name"`
	OrgUnitCode       string  `json:"org_unit_code"`
	MaxClassification string  `json:"max_classification"`
	CanExport         bool    `json:"can_export"`
	ValidFrom         string  `json:"valid_from"`
	ValidTo           *string `json:"valid_to"`
}

type auditAssignmentOption struct {
	ID        string `json:"id"`
	FullName  string `json:"full_name"`
	Email     string `json:"email"`
	OrgUnitID string `json:"org_unit_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Code      string `json:"code,omitempty"`
}

type auditAssignmentRequest struct {
	UserID            string  `json:"user_id" binding:"required"`
	OrgUnitID         string  `json:"org_unit_id" binding:"required"`
	MaxClassification string  `json:"max_classification" binding:"required,oneof=biasa terbatas rahasia"`
	CanExport         bool    `json:"can_export"`
	ValidFrom         string  `json:"valid_from" binding:"required"`
	ValidTo           *string `json:"valid_to"`
}

func (h *Handler) ListAuditAssignments(c *gin.Context) {
	rows, err := h.DB.Query(c.Request.Context(), `
		SELECT aa.id::text, u.id::text, u.full_name, u.email,
		       ou.id::text, ou.name, ou.code, aa.max_classification,
		       aa.can_export, aa.valid_from::text, aa.valid_to::text
		FROM audit_assignments aa
		JOIN users u ON u.id = aa.user_id
		JOIN org_units ou ON ou.id = aa.org_unit_id
		ORDER BY (aa.valid_to IS NULL) DESC, aa.valid_from DESC, u.full_name, ou.name`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat scope audit"})
		return
	}
	defer rows.Close()

	assignments := []auditAssignment{}
	for rows.Next() {
		var assignment auditAssignment
		if err := rows.Scan(
			&assignment.ID,
			&assignment.UserID,
			&assignment.UserName,
			&assignment.UserEmail,
			&assignment.OrgUnitID,
			&assignment.OrgUnitName,
			&assignment.OrgUnitCode,
			&assignment.MaxClassification,
			&assignment.CanExport,
			&assignment.ValidFrom,
			&assignment.ValidTo,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca scope audit"})
			return
		}
		assignments = append(assignments, assignment)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar scope audit"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": assignments})
}

func (h *Handler) AuditAssignmentOptions(c *gin.Context) {
	ctx := c.Request.Context()
	auditorRows, err := h.DB.Query(ctx, `
		SELECT DISTINCT u.id::text, u.full_name, u.email
		FROM users u
		JOIN user_roles ur ON ur.user_id = u.id
		JOIN roles r ON r.id = ur.role_id
		WHERE u.status = 'active' AND r.code = 'auditor'
		ORDER BY u.full_name`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat pengguna auditor"})
		return
	}
	defer auditorRows.Close()

	auditors := []auditAssignmentOption{}
	for auditorRows.Next() {
		var auditor auditAssignmentOption
		if err := auditorRows.Scan(&auditor.ID, &auditor.FullName, &auditor.Email); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca pengguna auditor"})
			return
		}
		auditors = append(auditors, auditor)
	}
	if err := auditorRows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca pengguna auditor"})
		return
	}

	unitRows, err := h.DB.Query(ctx, `
		SELECT id::text, name, code
		FROM org_units
		WHERE is_active
		ORDER BY name`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat unit organisasi"})
		return
	}
	defer unitRows.Close()

	orgUnits := []auditAssignmentOption{}
	for unitRows.Next() {
		var unit auditAssignmentOption
		if err := unitRows.Scan(&unit.OrgUnitID, &unit.Name, &unit.Code); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca unit organisasi"})
			return
		}
		orgUnits = append(orgUnits, unit)
	}
	if err := unitRows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca unit organisasi"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"auditors": auditors, "org_units": orgUnits})
}

func (h *Handler) CreateAuditAssignment(c *gin.Context) {
	var req auditAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data scope audit tidak valid: " + err.Error()})
		return
	}
	validTo, ok := validateAuditAssignmentRequest(c, &req)
	if !ok {
		return
	}
	if !h.validateAuditAssignmentReferences(c, req.UserID, req.OrgUnitID) {
		return
	}

	var id string
	err := h.DB.QueryRow(c.Request.Context(), `
		INSERT INTO audit_assignments
			(user_id, org_unit_id, max_classification, can_export, valid_from, valid_to)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id::text`,
		req.UserID, req.OrgUnitID, req.MaxClassification, req.CanExport, req.ValidFrom, validTo).Scan(&id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gagal membuat scope audit"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(c.Request.Context(), "audit_assignment", &id, "create", &actor, auditAssignmentAuditDetail(req), c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *Handler) UpdateAuditAssignment(c *gin.Context) {
	var req auditAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data scope audit tidak valid: " + err.Error()})
		return
	}
	validTo, ok := validateAuditAssignmentRequest(c, &req)
	if !ok {
		return
	}
	if !h.validateAuditAssignmentReferences(c, req.UserID, req.OrgUnitID) {
		return
	}

	id := c.Param("id")
	tag, err := h.DB.Exec(c.Request.Context(), `
		UPDATE audit_assignments
		SET user_id = $2, org_unit_id = $3, max_classification = $4,
		    can_export = $5, valid_from = $6, valid_to = $7
		WHERE id = $1`, id, req.UserID, req.OrgUnitID, req.MaxClassification, req.CanExport, req.ValidFrom, validTo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gagal memperbarui scope audit"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "scope audit tidak ditemukan"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(c.Request.Context(), "audit_assignment", &id, "update", &actor, auditAssignmentAuditDetail(req), c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
}

func (h *Handler) DeleteAuditAssignment(c *gin.Context) {
	id := c.Param("id")
	tag, err := h.DB.Exec(c.Request.Context(), `DELETE FROM audit_assignments WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menghapus scope audit"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "scope audit tidak ditemukan"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(c.Request.Context(), "audit_assignment", &id, "delete", &actor, nil, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
}

func validateAuditAssignmentRequest(c *gin.Context, req *auditAssignmentRequest) (*string, bool) {
	req.UserID = strings.TrimSpace(req.UserID)
	req.OrgUnitID = strings.TrimSpace(req.OrgUnitID)
	req.ValidFrom = strings.TrimSpace(req.ValidFrom)
	validFrom, err := time.Parse(time.DateOnly, req.ValidFrom)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tanggal mulai harus berformat YYYY-MM-DD"})
		return nil, false
	}
	if req.ValidTo == nil || strings.TrimSpace(*req.ValidTo) == "" {
		return nil, true
	}
	value := strings.TrimSpace(*req.ValidTo)
	validTo, err := time.Parse(time.DateOnly, value)
	if err != nil || !validTo.After(validFrom) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tanggal selesai harus setelah tanggal mulai"})
		return nil, false
	}
	return &value, true
}

func (h *Handler) validateAuditAssignmentReferences(c *gin.Context, userID string, orgUnitID string) bool {
	ctx := c.Request.Context()
	var auditorExists bool
	err := h.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM users u
			JOIN user_roles ur ON ur.user_id = u.id
			JOIN roles r ON r.id = ur.role_id
			WHERE u.id = $1 AND u.status = 'active' AND r.code = 'auditor'
		)`, userID).Scan(&auditorExists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa pengguna auditor"})
		return false
	}
	if !auditorExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pilih pengguna aktif dengan role auditor"})
		return false
	}

	var orgUnitExists bool
	err = h.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM org_units WHERE id = $1 AND is_active)`, orgUnitID).Scan(&orgUnitExists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa unit organisasi"})
		return false
	}
	if !orgUnitExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unit organisasi aktif tidak ditemukan"})
		return false
	}
	return true
}

func auditAssignmentAuditDetail(req auditAssignmentRequest) map[string]any {
	return map[string]any{
		"user_id":            req.UserID,
		"org_unit_id":        req.OrgUnitID,
		"max_classification": req.MaxClassification,
		"can_export":         req.CanExport,
		"valid_from":         req.ValidFrom,
		"valid_to":           req.ValidTo,
	}
}
