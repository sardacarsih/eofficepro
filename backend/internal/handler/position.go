package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

type positionResponse struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	PositionType   string  `json:"position_type"`
	IsApprover     bool    `json:"is_approver"`
	IsActive       bool    `json:"is_active"`
	ReportsTo      *string `json:"reports_to"`
	ReportsToTitle string  `json:"reports_to_title"`
	OrgUnitID      string  `json:"org_unit_id"`
	OrgUnitName    string  `json:"org_unit_name"`
	OrgUnitLevel   string  `json:"org_unit_level"`
	HolderName     string  `json:"holder_name"`
	HolderUserID   string  `json:"holder_user_id"`
	IdentityLocked bool    `json:"identity_locked"`
}

func (h *Handler) ListPositions(c *gin.Context) {
	orgUnitID := c.Query("org_unit_id")
	includeInactive := c.Query("include_inactive") == "true"
	ctx := c.Request.Context()

	query := `
		SELECT p.id::text, p.title, p.position_type, p.is_approver,
		       p.is_active, p.reports_to::text, COALESCE(manager.title, ''),
		       ou.id::text, ou.name, ou.unit_level,
		       COALESCE(holder.full_name, ''), COALESCE(holder.user_id, ''),
		       (
		           EXISTS (SELECT 1 FROM user_positions up WHERE up.position_id = p.id)
		           OR EXISTS (SELECT 1 FROM positions child WHERE child.reports_to = p.id)
		           OR EXISTS (SELECT 1 FROM delegations d WHERE d.delegator_position_id = p.id)
		           OR EXISTS (
		               SELECT 1 FROM letters l
		               WHERE l.creator_position_id = p.id OR l.on_behalf_of_position_id = p.id
		           )
		           OR EXISTS (SELECT 1 FROM letter_recipients lr WHERE lr.position_id = p.id)
		           OR EXISTS (SELECT 1 FROM approval_steps aps WHERE aps.approver_position_id = p.id)
		           OR EXISTS (SELECT 1 FROM dispositions d WHERE d.from_position_id = p.id)
		           OR EXISTS (SELECT 1 FROM disposition_recipients dr WHERE dr.position_id = p.id)
		       ) AS identity_locked
		FROM positions p
		JOIN org_units ou ON ou.id = p.org_unit_id
		LEFT JOIN positions manager ON manager.id = p.reports_to
		LEFT JOIN LATERAL (
			SELECT string_agg(u.full_name, ', ' ORDER BY u.full_name) AS full_name,
			       string_agg(u.id::text, ',' ORDER BY u.full_name) AS user_id
			FROM user_positions up
			JOIN users u ON u.id = up.user_id AND u.status = 'active'
			WHERE up.position_id = p.id
			  AND current_date >= up.valid_from
			  AND (up.valid_to IS NULL OR current_date < up.valid_to)
		) holder ON true
		WHERE true`
	args := []any{}
	if !includeInactive {
		query += ` AND p.is_active`
	}
	if orgUnitID != "" {
		query += ` AND p.org_unit_id = $1`
		args = append(args, orgUnitID)
	}
	query += ` ORDER BY p.is_active DESC, ou.name, p.title`

	rows, err := h.DB.Query(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat jabatan"})
		return
	}
	defer rows.Close()

	positions := []positionResponse{}
	for rows.Next() {
		var p positionResponse
		if err := rows.Scan(&p.ID, &p.Title, &p.PositionType, &p.IsApprover,
			&p.IsActive, &p.ReportsTo, &p.ReportsToTitle,
			&p.OrgUnitID, &p.OrgUnitName, &p.OrgUnitLevel,
			&p.HolderName, &p.HolderUserID, &p.IdentityLocked); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca data jabatan"})
			return
		}
		positions = append(positions, p)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar jabatan"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"positions": positions})
}

type positionRequest struct {
	OrgUnitID    string  `json:"org_unit_id" binding:"required"`
	Title        string  `json:"title" binding:"required,max=150"`
	PositionType string  `json:"position_type" binding:"required,oneof=president_director vp_director director gm dept_head sub_dept_head division_head assistant secretary staff auditor"`
	ReportsTo    *string `json:"reports_to"`
	IsApprover   bool    `json:"is_approver"`
}

type positionQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type positionValidationError struct {
	Status  int
	Message string
}

var singletonPositionTypes = map[string]bool{
	"president_director": true,
	"vp_director":        true,
	"director":           true,
	"gm":                 true,
	"dept_head":          true,
	"sub_dept_head":      true,
	"division_head":      true,
	"secretary":          true,
	"auditor":            true,
}

func normalizePositionRequest(req *positionRequest) {
	req.OrgUnitID = strings.TrimSpace(req.OrgUnitID)
	req.Title = strings.TrimSpace(req.Title)
	req.PositionType = strings.TrimSpace(req.PositionType)
	if req.ReportsTo != nil {
		value := strings.TrimSpace(*req.ReportsTo)
		if value == "" {
			req.ReportsTo = nil
		} else {
			req.ReportsTo = &value
		}
	}
}

func validatePositionMaster(
	ctx context.Context,
	db positionQueryer,
	req positionRequest,
	currentID *string,
	willBeActive bool,
) *positionValidationError {
	if req.Title == "" {
		return &positionValidationError{Status: http.StatusBadRequest, Message: "nama jabatan wajib diisi"}
	}
	var unitLevel string
	if err := db.QueryRow(ctx, `
		SELECT unit_level
		FROM org_units
		WHERE id = $1 AND is_active`, req.OrgUnitID).Scan(&unitLevel); err != nil {
		return &positionValidationError{Status: http.StatusBadRequest, Message: "unit jabatan tidak ditemukan atau tidak aktif"}
	}
	if !validPositionTypeForUnitLevel(unitLevel, req.PositionType) {
		return &positionValidationError{
			Status:  http.StatusBadRequest,
			Message: "tipe jabatan " + req.PositionType + " tidak valid untuk unit level " + unitLevel,
		}
	}
	if req.PositionType == "president_director" {
		if req.ReportsTo != nil {
			return &positionValidationError{Status: http.StatusBadRequest, Message: "President Director tidak boleh memiliki atasan langsung"}
		}
	} else if req.ReportsTo == nil {
		return &positionValidationError{Status: http.StatusBadRequest, Message: "atasan langsung wajib dipilih"}
	}

	if req.ReportsTo != nil {
		if currentID != nil && *req.ReportsTo == *currentID {
			return &positionValidationError{Status: http.StatusBadRequest, Message: "jabatan tidak boleh menjadi atasan bagi dirinya sendiri"}
		}
		var managerExists bool
		if err := db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM positions
				WHERE id = $1 AND is_active
			)`, *req.ReportsTo).Scan(&managerExists); err != nil || !managerExists {
			return &positionValidationError{Status: http.StatusBadRequest, Message: "atasan langsung tidak ditemukan atau tidak aktif"}
		}
		if currentID != nil {
			var createsCycle bool
			err := db.QueryRow(ctx, `
				WITH RECURSIVE manager_chain AS (
					SELECT id, reports_to
					FROM positions
					WHERE id = $1 AND is_active
					UNION
					SELECT manager.id, manager.reports_to
					FROM positions manager
					JOIN manager_chain child ON child.reports_to = manager.id
					WHERE manager.is_active
				)
				SELECT EXISTS (
					SELECT 1 FROM manager_chain WHERE id = $2
				)`, *req.ReportsTo, *currentID).Scan(&createsCycle)
			if err != nil {
				return &positionValidationError{Status: http.StatusInternalServerError, Message: "gagal memvalidasi rantai atasan"}
			}
			if createsCycle {
				return &positionValidationError{Status: http.StatusConflict, Message: "atasan langsung membentuk siklus jabatan"}
			}
		}
	}

	if !willBeActive {
		return nil
	}

	excludedID := ""
	if currentID != nil {
		excludedID = *currentID
	}
	var duplicate bool
	if singletonPositionTypes[req.PositionType] {
		if err := db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM positions
				WHERE org_unit_id = $1
				  AND position_type = $2
				  AND is_active
				  AND ($3 = '' OR id::text <> $3)
			)`, req.OrgUnitID, req.PositionType, excludedID).Scan(&duplicate); err != nil {
			return &positionValidationError{Status: http.StatusInternalServerError, Message: "gagal memvalidasi duplikasi jabatan"}
		}
	} else {
		if err := db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM positions
				WHERE org_unit_id = $1
				  AND lower(btrim(title)) = lower(btrim($2))
				  AND is_active
				  AND ($3 = '' OR id::text <> $3)
			)`, req.OrgUnitID, req.Title, excludedID).Scan(&duplicate); err != nil {
			return &positionValidationError{Status: http.StatusInternalServerError, Message: "gagal memvalidasi duplikasi jabatan"}
		}
	}
	if duplicate {
		return &positionValidationError{Status: http.StatusConflict, Message: "jabatan aktif dengan tipe atau nama yang sama sudah ada pada unit ini"}
	}
	return nil
}

func (h *Handler) CreatePosition(c *gin.Context) {
	var req positionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data jabatan tidak lengkap: " + err.Error()})
		return
	}
	normalizePositionRequest(&req)
	ctx := c.Request.Context()

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi jabatan"})
		return
	}
	defer tx.Rollback(ctx)

	if validationErr := validatePositionMaster(ctx, tx, req, nil, true); validationErr != nil {
		c.JSON(validationErr.Status, gin.H{"error": validationErr.Message})
		return
	}

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO positions (org_unit_id, title, position_type, reports_to, is_approver)
		VALUES ($1, $2, $3, $4, $5) RETURNING id::text`,
		req.OrgUnitID, req.Title, req.PositionType, req.ReportsTo, req.IsApprover).Scan(&id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gagal membuat jabatan (unit atau atasan tidak valid)"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan jabatan"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "position", &id, "create", &actor, map[string]any{
		"title": req.Title, "position_type": req.PositionType, "org_unit_id": req.OrgUnitID,
	}, c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *Handler) UpdatePosition(c *gin.Context) {
	id := c.Param("id")
	var req positionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data jabatan tidak lengkap: " + err.Error()})
		return
	}
	normalizePositionRequest(&req)
	ctx := c.Request.Context()

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi jabatan"})
		return
	}
	defer tx.Rollback(ctx)

	var existingOrgUnitID, existingPositionType string
	var isActive bool
	err = tx.QueryRow(ctx, `
		SELECT org_unit_id::text, position_type, is_active
		FROM positions
		WHERE id = $1
		FOR UPDATE`, id).Scan(&existingOrgUnitID, &existingPositionType, &isActive)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "jabatan tidak ditemukan"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca jabatan"})
		return
	}

	identityChanged := existingOrgUnitID != req.OrgUnitID || existingPositionType != req.PositionType
	if identityChanged {
		locked, err := positionIdentityLocked(ctx, tx, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memvalidasi histori jabatan"})
			return
		}
		if locked {
			c.JSON(http.StatusConflict, gin.H{"error": "unit dan tipe jabatan tidak dapat diubah karena jabatan sudah memiliki histori"})
			return
		}
	}

	if validationErr := validatePositionMaster(ctx, tx, req, &id, isActive); validationErr != nil {
		c.JSON(validationErr.Status, gin.H{"error": validationErr.Message})
		return
	}

	_, err = tx.Exec(ctx, `
		UPDATE positions
		SET org_unit_id = $2,
		    title = $3,
		    position_type = $4,
		    reports_to = $5,
		    is_approver = $6
		WHERE id = $1`,
		id, req.OrgUnitID, req.Title, req.PositionType, req.ReportsTo, req.IsApprover)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gagal memperbarui jabatan"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan perubahan jabatan"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "position", &id, "update", &actor, map[string]any{
		"title": req.Title, "position_type": req.PositionType, "org_unit_id": req.OrgUnitID,
		"reports_to": req.ReportsTo, "is_approver": req.IsApprover,
	}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
}

type positionDeactivationImpact struct {
	ActiveAssignments  int  `json:"active_assignments"`
	ActiveSubordinates int  `json:"active_subordinates"`
	ActiveDelegations  int  `json:"active_delegations"`
	ActiveDrafts       int  `json:"active_drafts"`
	ActiveApprovals    int  `json:"active_approvals"`
	ActiveDispositions int  `json:"active_dispositions"`
	CanDeactivate      bool `json:"can_deactivate"`
}

func loadPositionDeactivationImpact(
	ctx context.Context,
	db positionQueryer,
	positionID string,
) (positionDeactivationImpact, error) {
	var impact positionDeactivationImpact
	err := db.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM user_positions assignment
			 WHERE assignment.position_id = $1
			   AND current_date >= assignment.valid_from
			   AND (assignment.valid_to IS NULL OR current_date < assignment.valid_to)),
			(SELECT count(*) FROM positions child
			 WHERE child.reports_to = $1 AND child.is_active),
			(SELECT count(*) FROM delegations delegation
			 WHERE delegation.delegator_position_id = $1
			   AND current_timestamp >= delegation.valid_from
			   AND current_timestamp < delegation.valid_to),
			(SELECT count(DISTINCT letter.id)
			 FROM letters letter
			 LEFT JOIN letter_recipients recipient ON recipient.letter_id = letter.id
			 WHERE letter.status IN ('draft', 'revision')
			   AND (
			       letter.creator_position_id = $1
			       OR letter.on_behalf_of_position_id = $1
			       OR recipient.position_id = $1
			   )),
			(SELECT count(*) FROM approval_steps step
			 WHERE step.approver_position_id = $1
			   AND step.status IN ('pending', 'waiting')),
			(SELECT count(DISTINCT disposition.id)
			 FROM dispositions disposition
			 JOIN disposition_recipients recipient
			   ON recipient.disposition_id = disposition.id
			  AND recipient.status IN ('open', 'in_progress')
			 WHERE disposition.from_position_id = $1 OR recipient.position_id = $1)`,
		positionID).Scan(
		&impact.ActiveAssignments,
		&impact.ActiveSubordinates,
		&impact.ActiveDelegations,
		&impact.ActiveDrafts,
		&impact.ActiveApprovals,
		&impact.ActiveDispositions,
	)
	if err != nil {
		return positionDeactivationImpact{}, err
	}
	impact.CanDeactivate = impact.ActiveAssignments == 0 &&
		impact.ActiveSubordinates == 0 &&
		impact.ActiveDelegations == 0 &&
		impact.ActiveDrafts == 0 &&
		impact.ActiveApprovals == 0 &&
		impact.ActiveDispositions == 0
	return impact, nil
}

func positionIdentityLocked(ctx context.Context, db positionQueryer, positionID string) (bool, error) {
	var locked bool
	err := db.QueryRow(ctx, `
		SELECT
			EXISTS (SELECT 1 FROM user_positions WHERE position_id = $1)
			OR EXISTS (SELECT 1 FROM positions WHERE reports_to = $1)
			OR EXISTS (SELECT 1 FROM delegations WHERE delegator_position_id = $1)
			OR EXISTS (
				SELECT 1 FROM letters
				WHERE creator_position_id = $1 OR on_behalf_of_position_id = $1
			)
			OR EXISTS (SELECT 1 FROM letter_recipients WHERE position_id = $1)
			OR EXISTS (SELECT 1 FROM approval_steps WHERE approver_position_id = $1)
			OR EXISTS (SELECT 1 FROM dispositions WHERE from_position_id = $1)
			OR EXISTS (SELECT 1 FROM disposition_recipients WHERE position_id = $1)`,
		positionID).Scan(&locked)
	return locked, err
}

func (h *Handler) PositionDeactivationImpact(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	var exists bool
	if err := h.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM positions WHERE id = $1)`, id).Scan(&exists); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa jabatan"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "jabatan tidak ditemukan"})
		return
	}
	impact, err := loadPositionDeactivationImpact(ctx, h.DB, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa penggunaan jabatan"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"impact": impact})
}

func (h *Handler) DeactivatePosition(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi jabatan"})
		return
	}
	defer tx.Rollback(ctx)

	var title string
	err = tx.QueryRow(ctx, `SELECT title FROM positions WHERE id = $1 AND is_active FOR UPDATE`, id).Scan(&title)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "jabatan tidak ditemukan atau sudah nonaktif"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca jabatan"})
		return
	}

	impact, err := loadPositionDeactivationImpact(ctx, tx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa penggunaan jabatan"})
		return
	}
	if !impact.CanDeactivate {
		c.JSON(http.StatusConflict, gin.H{
			"error":  "jabatan masih digunakan dan belum dapat dinonaktifkan",
			"impact": impact,
		})
		return
	}
	if _, err := tx.Exec(ctx, `UPDATE positions SET is_active = false WHERE id = $1`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menonaktifkan jabatan"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan status jabatan"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "position", &id, "deactivate", &actor, map[string]any{"title": title}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
}

func (h *Handler) ActivatePosition(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi jabatan"})
		return
	}
	defer tx.Rollback(ctx)

	var req positionRequest
	var isActive bool
	err = tx.QueryRow(ctx, `
		SELECT org_unit_id::text, title, position_type, reports_to::text, is_approver, is_active
		FROM positions
		WHERE id = $1
		FOR UPDATE`, id).Scan(
		&req.OrgUnitID,
		&req.Title,
		&req.PositionType,
		&req.ReportsTo,
		&req.IsApprover,
		&isActive,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "jabatan tidak ditemukan"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca jabatan"})
		return
	}
	if isActive {
		c.JSON(http.StatusConflict, gin.H{"error": "jabatan sudah aktif"})
		return
	}
	if validationErr := validatePositionMaster(ctx, tx, req, &id, true); validationErr != nil {
		c.JSON(validationErr.Status, gin.H{"error": validationErr.Message})
		return
	}
	if _, err := tx.Exec(ctx, `UPDATE positions SET is_active = true WHERE id = $1`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengaktifkan jabatan"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan status jabatan"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "position", &id, "activate", &actor, map[string]any{"title": req.Title}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
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

func (h *Handler) EndUserPositionAssignment(c *gin.Context) {
	id := c.Param("id")
	actor := c.GetString(middleware.CtxUserID)
	ctx := c.Request.Context()

	var userID, positionID, assignmentType string
	err := h.DB.QueryRow(ctx, `
		UPDATE user_positions
		SET valid_to = current_date
		WHERE id = $1
		  AND current_date >= valid_from
		  AND (valid_to IS NULL OR current_date < valid_to)
		RETURNING user_id::text, position_id::text, assignment_type`, id).
		Scan(&userID, &positionID, &assignmentType)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "penempatan tidak ditemukan atau sudah berakhir"})
		return
	}

	h.audit(ctx, "position_assignment", &id, "position_unassign", &actor,
		map[string]any{
			"user_id":         userID,
			"position_id":     positionID,
			"assignment_type": assignmentType,
		}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
}
