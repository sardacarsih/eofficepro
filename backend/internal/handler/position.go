package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

func (h *Handler) ListPositions(c *gin.Context) {
	orgUnitID := c.Query("org_unit_id")
	ctx := c.Request.Context()

	query := `
		SELECT p.id::text, p.title, p.position_type, p.is_approver,
		       p.reports_to::text, ou.id::text, ou.name,
		       COALESCE(u.full_name, ''), COALESCE(u.id::text, '')
		FROM positions p
		JOIN org_units ou ON ou.id = p.org_unit_id
		LEFT JOIN user_positions up ON up.position_id = p.id
		     AND current_date >= up.valid_from
		     AND (up.valid_to IS NULL OR current_date <= up.valid_to)
		LEFT JOIN users u ON u.id = up.user_id AND u.status = 'active'
		WHERE p.is_active`
	args := []any{}
	if orgUnitID != "" {
		query += ` AND p.org_unit_id = $1`
		args = append(args, orgUnitID)
	}
	query += ` ORDER BY ou.name, p.title`

	rows, err := h.DB.Query(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat jabatan"})
		return
	}
	defer rows.Close()

	type position struct {
		ID           string  `json:"id"`
		Title        string  `json:"title"`
		PositionType string  `json:"position_type"`
		IsApprover   bool    `json:"is_approver"`
		ReportsTo    *string `json:"reports_to"`
		OrgUnitID    string  `json:"org_unit_id"`
		OrgUnitName  string  `json:"org_unit_name"`
		HolderName   string  `json:"holder_name"`
		HolderUserID string  `json:"holder_user_id"`
	}
	positions := []position{}
	for rows.Next() {
		var p position
		if err := rows.Scan(&p.ID, &p.Title, &p.PositionType, &p.IsApprover,
			&p.ReportsTo, &p.OrgUnitID, &p.OrgUnitName, &p.HolderName, &p.HolderUserID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca data jabatan"})
			return
		}
		positions = append(positions, p)
	}
	c.JSON(http.StatusOK, gin.H{"positions": positions})
}

type positionRequest struct {
	OrgUnitID    string  `json:"org_unit_id" binding:"required"`
	Title        string  `json:"title" binding:"required"`
	PositionType string  `json:"position_type" binding:"required,oneof=president_director vp_director director gm dept_head section_head division_head assistant secretary staff auditor"`
	ReportsTo    *string `json:"reports_to"`
	IsApprover   bool    `json:"is_approver"`
}

func (h *Handler) CreatePosition(c *gin.Context) {
	var req positionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data jabatan tidak lengkap: " + err.Error()})
		return
	}
	ctx := c.Request.Context()

	var id string
	err := h.DB.QueryRow(ctx, `
		INSERT INTO positions (org_unit_id, title, position_type, reports_to, is_approver)
		VALUES ($1, $2, $3, $4, $5) RETURNING id::text`,
		req.OrgUnitID, req.Title, req.PositionType, req.ReportsTo, req.IsApprover).Scan(&id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gagal membuat jabatan (unit atau atasan tidak valid)"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "position", &id, "create", &actor, map[string]any{"title": req.Title}, c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

type assignRequest struct {
	UserID         string `json:"user_id" binding:"required"`
	AssignmentType string `json:"assignment_type" binding:"omitempty,oneof=definitive plt plh"`
}

// AssignPosition menempatkan pengguna pada jabatan. Pemegang definitif lama
// (bila ada) ditutup masa berlakunya per hari ini.
func (h *Handler) AssignPosition(c *gin.Context) {
	positionID := c.Param("id")
	var req assignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id wajib diisi"})
		return
	}
	if req.AssignmentType == "" {
		req.AssignmentType = "definitive"
	}
	ctx := c.Request.Context()

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi"})
		return
	}
	defer tx.Rollback(ctx)

	if req.AssignmentType == "definitive" {
		_, err = tx.Exec(ctx, `
			UPDATE user_positions SET valid_to = current_date
			WHERE position_id = $1 AND assignment_type = 'definitive'
			  AND (valid_to IS NULL OR valid_to > current_date)`, positionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menutup penempatan lama"})
			return
		}
	}

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO user_positions (user_id, position_id, assignment_type)
		VALUES ($1, $2, $3) RETURNING id::text`,
		req.UserID, positionID, req.AssignmentType).Scan(&id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gagal menempatkan pengguna (user atau jabatan tidak valid)"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan penempatan"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "position", &positionID, "assign", &actor,
		map[string]any{"user_id": req.UserID, "assignment_type": req.AssignmentType}, c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"id": id})
}
