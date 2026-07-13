package handler

// Delegasi wewenang approval berbatas waktu (E03-5, PRD P0-3).
// Status delegasi diturunkan dari waktu query — tanpa job/cron:
//   scheduled (belum mulai), active (berjalan), expired (lewat), revoked (dicabut).
// Selama aktif, delegate dapat melihat dan menindaklanjuti step waiting posisi
// delegator; aksi tercatat "a.n." lewat approval_actions.on_behalf_delegation_id.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

// activeDelegationExistsSQLTemplate adalah satu-satunya definisi fragmen SQL
// "delegasi aktif" — dipakai inbox/act/view/notify/SLA agar semantik waktu dan
// revoke selalu konsisten. %s pertama = ekspresi kolom posisi delegator,
// %s kedua = parameter user delegate.
const activeDelegationExistsSQLTemplate = `EXISTS (
	SELECT 1 FROM delegations dg_frag
	WHERE dg_frag.delegator_position_id = %s
	  AND dg_frag.delegate_user_id = %s
	  AND now() >= dg_frag.valid_from AND now() < dg_frag.valid_to
	  AND dg_frag.revoked_at IS NULL
)`

func activeDelegationExistsSQL(positionExpr string, userParam string) string {
	return fmt.Sprintf(activeDelegationExistsSQLTemplate, positionExpr, userParam)
}

// delegationStatusSQL menurunkan status delegasi dari waktu query.
const delegationStatusSQL = `CASE
	WHEN dg.revoked_at IS NOT NULL THEN 'revoked'
	WHEN now() >= dg.valid_to THEN 'expired'
	WHEN now() < dg.valid_from THEN 'scheduled'
	ELSE 'active'
END`

type delegationItem struct {
	ID                     string     `json:"id"`
	DelegatorPositionID    string     `json:"delegator_position_id"`
	DelegatorPositionTitle string     `json:"delegator_position_title"`
	DelegateUserID         string     `json:"delegate_user_id"`
	DelegateName           string     `json:"delegate_name"`
	Reason                 string     `json:"reason"`
	ValidFrom              time.Time  `json:"valid_from"`
	ValidTo                time.Time  `json:"valid_to"`
	RevokedAt              *time.Time `json:"revoked_at"`
	Status                 string     `json:"status"`
	CreatedByName          string     `json:"created_by_name"`
	CreatedAt              time.Time  `json:"created_at"`
}

type createDelegationRequest struct {
	DelegatorPositionID string `json:"delegator_position_id"`
	DelegateUserID      string `json:"delegate_user_id"`
	Reason              string `json:"reason"`
	ValidFrom           string `json:"valid_from"`
	ValidTo             string `json:"valid_to"`
}

const selectDelegationItemSQL = `
	SELECT dg.id::text, dg.delegator_position_id::text, p.title,
	       dg.delegate_user_id::text, du.full_name, dg.reason,
	       dg.valid_from, dg.valid_to, dg.revoked_at,
	       ` + delegationStatusSQL + `,
	       cu.full_name, dg.created_at
	FROM delegations dg
	JOIN positions p ON p.id = dg.delegator_position_id
	JOIN users du ON du.id = dg.delegate_user_id
	JOIN users cu ON cu.id = dg.created_by`

func scanDelegationItem(row pgx.Row) (delegationItem, error) {
	var item delegationItem
	err := row.Scan(
		&item.ID,
		&item.DelegatorPositionID,
		&item.DelegatorPositionTitle,
		&item.DelegateUserID,
		&item.DelegateName,
		&item.Reason,
		&item.ValidFrom,
		&item.ValidTo,
		&item.RevokedAt,
		&item.Status,
		&item.CreatedByName,
		&item.CreatedAt,
	)
	return item, err
}

// delegationPositionCompany memuat company posisi delegator; posisi harus aktif.
func (h *Handler) delegationPositionCompany(ctx context.Context, positionID string) (companyID string, title string, err error) {
	err = h.DB.QueryRow(ctx, `
		SELECT ou.company_id::text, p.title
		FROM positions p
		JOIN org_units ou ON ou.id = p.org_unit_id
		WHERE p.id = $1 AND p.is_active`, positionID).Scan(&companyID, &title)
	return companyID, title, err
}

// userHoldsPositionActive memeriksa apakah user pemegang aktif posisi.
func (h *Handler) userHoldsPositionActive(ctx context.Context, userID string, positionID string) (bool, error) {
	var holds bool
	err := h.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM user_positions up
			JOIN users u ON u.id = up.user_id
			WHERE up.user_id = $1
			  AND up.position_id = $2
			  AND current_date >= up.valid_from
			  AND (up.valid_to IS NULL OR current_date < up.valid_to)
			  AND u.status = 'active'
		)`, userID, positionID).Scan(&holds)
	return holds, err
}

// requireDelegationManager memastikan caller pemegang aktif posisi delegator
// atau admin company posisi itu. Mengembalikan companyID + judul posisi.
func (h *Handler) requireDelegationManager(c *gin.Context, userID string, positionID string) (companyID string, positionTitle string, ok bool) {
	ctx := c.Request.Context()
	companyID, positionTitle, err := h.delegationPositionCompany(ctx, positionID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "posisi delegator tidak ditemukan"})
		return "", "", false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa posisi delegator"})
		return "", "", false
	}

	holds, err := h.userHoldsPositionActive(ctx, userID, positionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa pemegang posisi"})
		return "", "", false
	}
	if !holds {
		isAdmin, err := h.canAdminCompany(ctx, userID, companyID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa akses perusahaan"})
			return "", "", false
		}
		if !isAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "hanya pemegang posisi atau admin perusahaan yang dapat mengelola delegasi posisi ini"})
			return "", "", false
		}
	}
	return companyID, positionTitle, true
}

func (h *Handler) CreateDelegation(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	ctx := c.Request.Context()

	var req createDelegationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data delegasi tidak valid: " + err.Error()})
		return
	}
	req.DelegatorPositionID = strings.TrimSpace(req.DelegatorPositionID)
	req.DelegateUserID = strings.TrimSpace(req.DelegateUserID)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.DelegatorPositionID == "" || req.DelegateUserID == "" || req.Reason == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "posisi delegator, user delegate, dan alasan wajib diisi"})
		return
	}
	if len([]rune(req.Reason)) > 255 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "alasan delegasi maksimal 255 karakter"})
		return
	}
	validFrom, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ValidFrom))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid_from wajib berformat RFC3339"})
		return
	}
	validTo, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ValidTo))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid_to wajib berformat RFC3339"})
		return
	}
	if !validFrom.Before(validTo) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid_from harus sebelum valid_to"})
		return
	}
	if !validTo.After(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid_to harus di masa depan"})
		return
	}

	companyID, _, ok := h.requireDelegationManager(c, userID, req.DelegatorPositionID)
	if !ok {
		return
	}

	// Delegate wajib user aktif pemegang posisi aktif pada company yang sama.
	var delegateEligible bool
	if err := h.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM users u
			JOIN user_positions up ON up.user_id = u.id
			JOIN positions p ON p.id = up.position_id
			JOIN org_units ou ON ou.id = p.org_unit_id
			WHERE u.id = $1
			  AND u.status = 'active'
			  AND p.is_active AND ou.is_active
			  AND ou.company_id = $2
			  AND current_date >= up.valid_from
			  AND (up.valid_to IS NULL OR current_date < up.valid_to)
		)`, req.DelegateUserID, companyID).Scan(&delegateEligible); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa user delegate"})
		return
	}
	if !delegateEligible {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user delegate tidak aktif atau bukan pemegang posisi aktif pada perusahaan yang sama"})
		return
	}

	// Tanpa self-delegation: delegate bukan pemegang aktif posisi delegator.
	delegateHoldsDelegator, err := h.userHoldsPositionActive(ctx, req.DelegateUserID, req.DelegatorPositionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa pemegang posisi delegator"})
		return
	}
	if delegateHoldsDelegator {
		c.JSON(http.StatusBadRequest, gin.H{"error": "delegate sudah pemegang aktif posisi delegator (self-delegation tidak diizinkan)"})
		return
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi"})
		return
	}
	defer tx.Rollback(ctx)

	var delegationID string
	err = tx.QueryRow(ctx, `
		INSERT INTO delegations (delegator_position_id, delegate_user_id, reason, valid_from, valid_to, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id::text`,
		req.DelegatorPositionID, req.DelegateUserID, req.Reason, validFrom, validTo, userID).Scan(&delegationID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
			c.JSON(http.StatusConflict, gin.H{"error": "rentang delegasi tumpang tindih dengan delegasi lain untuk posisi ini"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan delegasi"})
		return
	}

	pendingEmails, err := notifyDelegationChanged(ctx, tx, delegationID, "delegation_created")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengirim notifikasi delegasi"})
		return
	}
	if err := enqueueNotificationOutbox(ctx, tx, pendingEmails); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengantrikan notifikasi delegasi"})
		return
	}

	item, err := scanDelegationItem(tx.QueryRow(ctx, selectDelegationItemSQL+` WHERE dg.id = $1`, delegationID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat delegasi"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan delegasi"})
		return
	}

	h.audit(ctx, "delegation", &delegationID, "create", &userID, map[string]any{
		"delegator_position_id": req.DelegatorPositionID,
		"delegate_user_id":      req.DelegateUserID,
		"reason":                req.Reason,
		"valid_from":            validFrom.Format(time.RFC3339),
		"valid_to":              validTo.Format(time.RFC3339),
	}, c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"delegation": item})
}

func (h *Handler) ListDelegations(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	ctx := c.Request.Context()

	scope := strings.ToLower(strings.TrimSpace(c.Query("scope")))
	if scope != "" && scope != "delegator" && scope != "delegate" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope tidak valid (pakai: delegator | delegate)"})
		return
	}
	includePast := strings.EqualFold(strings.TrimSpace(c.Query("include_past")), "true")

	delegatorCond := `EXISTS (
		SELECT 1 FROM user_positions up
		WHERE up.position_id = dg.delegator_position_id
		  AND up.user_id = $1
		  AND current_date >= up.valid_from
		  AND (up.valid_to IS NULL OR current_date < up.valid_to)
	)`
	delegateCond := `dg.delegate_user_id = $1`
	adminCond := `EXISTS (
		SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1 AND r.code = 'super_admin'
	) OR EXISTS (
		SELECT 1 FROM user_company_roles ucr JOIN roles r ON r.id = ucr.role_id
		WHERE ucr.user_id = $1 AND ucr.company_id = ou.company_id
		  AND r.code = 'admin'
		  AND current_date >= ucr.valid_from
		  AND (ucr.valid_to IS NULL OR current_date < ucr.valid_to)
	)`

	var visibility string
	switch scope {
	case "delegator":
		visibility = delegatorCond
	case "delegate":
		visibility = delegateCond
	default:
		visibility = delegatorCond + ` OR ` + delegateCond + ` OR ` + adminCond
	}

	query := selectDelegationItemSQL + `
		JOIN org_units ou ON ou.id = p.org_unit_id
		WHERE (` + visibility + `)`
	if !includePast {
		query += ` AND dg.revoked_at IS NULL AND now() < dg.valid_to`
	}
	query += ` ORDER BY dg.valid_from DESC, dg.created_at DESC`

	rows, err := h.DB.Query(ctx, query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat daftar delegasi"})
		return
	}
	defer rows.Close()

	items := []delegationItem{}
	for rows.Next() {
		item, err := scanDelegationItem(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca delegasi"})
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar delegasi"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) RevokeDelegation(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	delegationID := c.Param("id")
	ctx := c.Request.Context()

	var (
		positionID string
		companyID  string
		createdBy  string
		status     string
	)
	err := h.DB.QueryRow(ctx, `
		SELECT dg.delegator_position_id::text, ou.company_id::text, dg.created_by::text,
		       `+delegationStatusSQL+`
		FROM delegations dg
		JOIN positions p ON p.id = dg.delegator_position_id
		JOIN org_units ou ON ou.id = p.org_unit_id
		WHERE dg.id = $1`, delegationID).Scan(&positionID, &companyID, &createdBy, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "delegasi tidak ditemukan"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat delegasi"})
		return
	}

	allowed := createdBy == userID
	if !allowed {
		holds, err := h.userHoldsPositionActive(ctx, userID, positionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa pemegang posisi"})
			return
		}
		allowed = holds
	}
	if !allowed {
		isAdmin, err := h.canAdminCompany(ctx, userID, companyID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa akses perusahaan"})
			return
		}
		allowed = isAdmin
	}
	if !allowed {
		// Tanpa relasi apa pun ke company delegasi: sembunyikan keberadaannya.
		related, err := h.userRelatedToCompany(ctx, userID, companyID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa akses perusahaan"})
			return
		}
		if !related {
			c.JSON(http.StatusNotFound, gin.H{"error": "delegasi tidak ditemukan"})
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "hanya pembuat, pemegang posisi delegator, atau admin perusahaan yang dapat mencabut delegasi"})
		return
	}

	if status == "expired" {
		c.JSON(http.StatusConflict, gin.H{"error": "delegasi sudah berakhir"})
		return
	}
	if status == "revoked" {
		c.JSON(http.StatusConflict, gin.H{"error": "delegasi sudah dicabut"})
		return
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi"})
		return
	}
	defer tx.Rollback(ctx)

	// Guard atomik: hanya delegasi non-revoked yang belum berakhir yang dicabut.
	tag, err := tx.Exec(ctx, `
		UPDATE delegations
		SET revoked_at = now(), revoked_by = $2
		WHERE id = $1
		  AND revoked_at IS NULL
		  AND now() < valid_to`, delegationID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mencabut delegasi"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "delegasi sudah berakhir atau sudah dicabut"})
		return
	}

	pendingEmails, err := notifyDelegationChanged(ctx, tx, delegationID, "delegation_revoked")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengirim notifikasi delegasi"})
		return
	}
	if err := enqueueNotificationOutbox(ctx, tx, pendingEmails); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengantrikan notifikasi delegasi"})
		return
	}

	item, err := scanDelegationItem(tx.QueryRow(ctx, selectDelegationItemSQL+` WHERE dg.id = $1`, delegationID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat delegasi"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan pencabutan delegasi"})
		return
	}

	h.audit(ctx, "delegation", &delegationID, "revoke", &userID, map[string]any{
		"delegator_position_id": positionID,
		"delegate_user_id":      item.DelegateUserID,
	}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"delegation": item})
}

// ListDelegateOptions mengembalikan kandidat delegate: user aktif pemegang
// posisi aktif pada company posisi delegator, tanpa pemegang posisi itu sendiri.
func (h *Handler) ListDelegateOptions(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	ctx := c.Request.Context()

	positionID := strings.TrimSpace(c.Query("position_id"))
	if positionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "position_id wajib diisi"})
		return
	}
	companyID, _, ok := h.requireDelegationManager(c, userID, positionID)
	if !ok {
		return
	}

	rows, err := h.DB.Query(ctx, `
		SELECT u.id::text, u.full_name,
		       array_agg(DISTINCT p.title ORDER BY p.title)
		FROM users u
		JOIN user_positions up ON up.user_id = u.id
		JOIN positions p ON p.id = up.position_id
		JOIN org_units ou ON ou.id = p.org_unit_id
		WHERE ou.company_id = $1
		  AND u.status = 'active'
		  AND p.is_active AND ou.is_active
		  AND current_date >= up.valid_from
		  AND (up.valid_to IS NULL OR current_date < up.valid_to)
		  AND NOT EXISTS (
			SELECT 1 FROM user_positions uph
			WHERE uph.user_id = u.id
			  AND uph.position_id = $2
			  AND current_date >= uph.valid_from
			  AND (uph.valid_to IS NULL OR current_date < uph.valid_to)
		  )
		GROUP BY u.id, u.full_name
		ORDER BY u.full_name`, companyID, positionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat kandidat delegate"})
		return
	}
	defer rows.Close()

	type delegateOption struct {
		UserID         string   `json:"user_id"`
		FullName       string   `json:"full_name"`
		PositionTitles []string `json:"position_titles"`
	}
	items := []delegateOption{}
	for rows.Next() {
		var item delegateOption
		if err := rows.Scan(&item.UserID, &item.FullName, &item.PositionTitles); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca kandidat delegate"})
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar kandidat delegate"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// userRelatedToCompany memeriksa relasi minimal user ke company (posisi aktif
// atau company role aktif) untuk membedakan 403 vs 404 lintas company.
func (h *Handler) userRelatedToCompany(ctx context.Context, userID string, companyID string) (bool, error) {
	var related bool
	err := h.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM user_positions up
			JOIN positions p ON p.id = up.position_id
			JOIN org_units ou ON ou.id = p.org_unit_id
			WHERE up.user_id = $1 AND ou.company_id = $2
			  AND current_date >= up.valid_from
			  AND (up.valid_to IS NULL OR current_date < up.valid_to)
		) OR EXISTS (
			SELECT 1 FROM user_company_roles ucr
			WHERE ucr.user_id = $1 AND ucr.company_id = $2
			  AND current_date >= ucr.valid_from
			  AND (ucr.valid_to IS NULL OR current_date < ucr.valid_to)
		)`, userID, companyID).Scan(&related)
	return related, err
}

// notifyDelegationChanged memberi tahu delegate dan seluruh pemegang aktif
// posisi delegator bahwa delegasi dibuat/dicabut. Berjalan di dalam transaksi;
// email dikirim lewat outbox setelah commit.
func notifyDelegationChanged(ctx context.Context, tx pgx.Tx, delegationID string, eventType string) ([]notificationEmail, error) {
	var title, bodyPrefix string
	switch eventType {
	case "delegation_created":
		title = "Delegasi wewenang dibuat"
		bodyPrefix = "Delegasi wewenang approval"
	case "delegation_revoked":
		title = "Delegasi wewenang dicabut"
		bodyPrefix = "Pencabutan delegasi wewenang approval"
	default:
		return nil, errors.New("event delegasi tidak dikenal")
	}

	rows, err := tx.Query(ctx, `
		WITH dg AS (
			SELECT d.id, d.delegator_position_id, d.delegate_user_id,
			       d.valid_from, d.valid_to,
			       p.title AS position_title, du.full_name AS delegate_name
			FROM delegations d
			JOIN positions p ON p.id = d.delegator_position_id
			JOIN users du ON du.id = d.delegate_user_id
			WHERE d.id = $1
		),
		targets AS (
			SELECT DISTINCT u.id AS user_id, dg.position_title, dg.delegate_name,
			       dg.valid_from, dg.valid_to
			FROM dg
			JOIN users u ON u.id = dg.delegate_user_id AND u.status = 'active'
			UNION
			SELECT DISTINCT up.user_id, dg.position_title, dg.delegate_name,
			       dg.valid_from, dg.valid_to
			FROM dg
			JOIN user_positions up ON up.position_id = dg.delegator_position_id
			JOIN users u ON u.id = up.user_id AND u.status = 'active'
			WHERE current_date >= up.valid_from
			  AND (up.valid_to IS NULL OR current_date < up.valid_to)
		),
		inserted AS (
			INSERT INTO notifications (user_id, event_type, letter_id, title, body)
			SELECT t.user_id, $2, NULL,
			       $3 || ': ' || t.position_title,
			       $4 || ' posisi ' || t.position_title || ' kepada ' || t.delegate_name
			         || ' (berlaku ' || to_char(t.valid_from AT TIME ZONE 'Asia/Jakarta', 'DD Mon YYYY HH24:MI')
			         || ' - ' || to_char(t.valid_to AT TIME ZONE 'Asia/Jakarta', 'DD Mon YYYY HH24:MI') || ' WIB).'
			FROM targets t
			RETURNING user_id, event_type, title, body
		)
		SELECT u.email, i.event_type, '', i.title, i.body
		FROM inserted i
		JOIN users u ON u.id = i.user_id`, delegationID, eventType, title, bodyPrefix)
	if err != nil {
		return nil, err
	}
	return collectNotificationEmails(rows)
}
