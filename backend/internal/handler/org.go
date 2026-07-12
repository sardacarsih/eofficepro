package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

type OrgUnit struct {
	ID          string     `json:"id"`
	CompanyID   string     `json:"company_id"`
	CompanyCode string     `json:"company_code"`
	CompanyName string     `json:"company_name"`
	ParentID    *string    `json:"parent_id"`
	Code        string     `json:"code"`
	Name        string     `json:"name"`
	UnitLevel   string     `json:"unit_level"`
	Region      *string    `json:"region"`
	IsActive    bool       `json:"is_active"`
	Children    []*OrgUnit `json:"children,omitempty"`
}

// OrgTree mengembalikan seluruh unit aktif sebagai pohon bersarang.
func (h *Handler) OrgTree(c *gin.Context) {
	companies, err := h.accessibleCompanies(c.Request.Context(), c.GetString(middleware.CtxUserID), false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa akses perusahaan"})
		return
	}
	companyIDs := make([]string, 0, len(companies))
	for _, company := range companies {
		companyIDs = append(companyIDs, company.ID)
	}
	rows, err := h.DB.Query(c.Request.Context(), `
		SELECT ou.id::text, company.id::text, company.code, company.name,
		       ou.parent_id::text, ou.code, ou.name, ou.unit_level, ou.region, ou.is_active
		FROM org_units ou JOIN companies company ON company.id = ou.company_id
		WHERE ou.is_active AND ou.company_id::text = ANY($1::text[])
		ORDER BY company.code, ou.unit_level, ou.name`, companyIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat struktur organisasi"})
		return
	}
	defer rows.Close()

	byID := map[string]*OrgUnit{}
	order := []*OrgUnit{}
	for rows.Next() {
		u := &OrgUnit{}
		if err := rows.Scan(&u.ID, &u.CompanyID, &u.CompanyCode, &u.CompanyName, &u.ParentID, &u.Code, &u.Name, &u.UnitLevel, &u.Region, &u.IsActive); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca data unit"})
			return
		}
		byID[u.ID] = u
		order = append(order, u)
	}

	roots := []*OrgUnit{}
	for _, u := range order {
		if u.ParentID != nil {
			if parent, ok := byID[*u.ParentID]; ok {
				parent.Children = append(parent.Children, u)
				continue
			}
		}
		roots = append(roots, u)
	}
	c.JSON(http.StatusOK, gin.H{"tree": roots, "total": len(order)})
}

type orgUnitRequest struct {
	CompanyID string  `json:"company_id"`
	ParentID  *string `json:"parent_id"`
	Code      string  `json:"code" binding:"required"`
	Name      string  `json:"name" binding:"required"`
	UnitLevel string  `json:"unit_level" binding:"required,oneof=directorate biro department division office"`
	Region    *string `json:"region" binding:"omitempty,oneof=HO REG1 REG2 REPO_JKT REPO_PKB"`
}

func (h *Handler) CreateOrgUnit(c *gin.Context) {
	var req orgUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data unit tidak lengkap: " + err.Error()})
		return
	}
	ctx := c.Request.Context()
	if req.CompanyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perusahaan wajib dipilih"})
		return
	}
	if !h.requireAdminCompany(c, req.CompanyID) {
		return
	}
	if req.ParentID != nil {
		var validParent bool
		if err := h.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM org_units WHERE id = $1 AND company_id = $2 AND is_active)`, *req.ParentID, req.CompanyID).Scan(&validParent); err != nil || !validParent {
			c.JSON(http.StatusBadRequest, gin.H{"error": "parent unit harus berasal dari perusahaan yang sama"})
			return
		}
	}

	var id string
	err := h.DB.QueryRow(ctx, `
		INSERT INTO org_units (company_id, parent_id, code, name, unit_level, region)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id::text`,
		req.CompanyID, req.ParentID, req.Code, req.Name, req.UnitLevel, req.Region).Scan(&id)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "gagal membuat unit (kode mungkin sudah dipakai)"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "org_unit", &id, "create", &actor, map[string]any{"code": req.Code, "name": req.Name}, c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *Handler) UpdateOrgUnit(c *gin.Context) {
	id := c.Param("id")
	companyID, ok := h.requireAdminResourceCompany(c, `SELECT company_id::text FROM org_units WHERE id = $1`, id)
	if !ok {
		return
	}
	var req orgUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data unit tidak lengkap: " + err.Error()})
		return
	}
	ctx := c.Request.Context()
	if req.CompanyID != "" && req.CompanyID != companyID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perusahaan unit tidak dapat dipindahkan"})
		return
	}
	if req.ParentID != nil {
		var validParent bool
		if err := h.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM org_units WHERE id = $1 AND company_id = $2 AND id <> $3 AND is_active)`, *req.ParentID, companyID, id).Scan(&validParent); err != nil || !validParent {
			c.JSON(http.StatusBadRequest, gin.H{"error": "parent unit harus berasal dari perusahaan yang sama"})
			return
		}
	}

	tag, err := h.DB.Exec(ctx, `
		UPDATE org_units SET parent_id = $2, code = $3, name = $4, unit_level = $5, region = $6
		WHERE id = $1 AND is_active`,
		id, req.ParentID, req.Code, req.Name, req.UnitLevel, req.Region)
	if err != nil || tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "unit tidak ditemukan atau kode bentrok"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "org_unit", &id, "update", &actor, map[string]any{"name": req.Name}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// DeactivateOrgUnit melakukan soft delete; ditolak bila masih punya sub-unit aktif.
func (h *Handler) DeactivateOrgUnit(c *gin.Context) {
	id := c.Param("id")
	if _, ok := h.requireAdminResourceCompany(c, `SELECT company_id::text FROM org_units WHERE id = $1`, id); !ok {
		return
	}
	ctx := c.Request.Context()

	var activeChildren int
	_ = h.DB.QueryRow(ctx,
		`SELECT count(*) FROM org_units WHERE parent_id = $1 AND is_active`, id).Scan(&activeChildren)
	if activeChildren > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "unit masih memiliki sub-unit aktif — nonaktifkan sub-unit terlebih dahulu"})
		return
	}

	tag, err := h.DB.Exec(ctx,
		`UPDATE org_units SET is_active = false, valid_to = current_date WHERE id = $1 AND is_active`, id)
	if err != nil || tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "unit tidak ditemukan"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "org_unit", &id, "deactivate", &actor, nil, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
}
