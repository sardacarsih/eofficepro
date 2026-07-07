package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/auth"
	"github.com/kskgroup/eofficepro/internal/middleware"
)

func (h *Handler) ListUsers(c *gin.Context) {
	rows, err := h.DB.Query(c.Request.Context(), `
		SELECT u.id::text, u.nik, u.email, u.full_name, u.status,
		       COALESCE(array_agg(DISTINCT r.code) FILTER (WHERE r.code IS NOT NULL), '{}')
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		GROUP BY u.id ORDER BY u.full_name`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat pengguna"})
		return
	}
	defer rows.Close()

	type user struct {
		ID       string   `json:"id"`
		NIK      string   `json:"nik"`
		Email    string   `json:"email"`
		FullName string   `json:"full_name"`
		Status   string   `json:"status"`
		Roles    []string `json:"roles"`
	}
	users := []user{}
	for rows.Next() {
		var u user
		if err := rows.Scan(&u.ID, &u.NIK, &u.Email, &u.FullName, &u.Status, &u.Roles); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca data pengguna"})
			return
		}
		users = append(users, u)
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

type createUserRequest struct {
	NIK      string   `json:"nik" binding:"required"`
	Email    string   `json:"email" binding:"required,email"`
	FullName string   `json:"full_name" binding:"required"`
	Password string   `json:"password" binding:"required,min=10"`
	Roles    []string `json:"roles" binding:"required,min=1"`
	Status   string   `json:"status"`
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
	ctx := c.Request.Context()

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
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan pengguna"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "user", &id, "create", &actor, map[string]any{"nik": req.NIK, "roles": roles, "status": req.Status}, c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

type updateUserRequest struct {
	NIK      string   `json:"nik" binding:"required"`
	Email    string   `json:"email" binding:"required,email"`
	FullName string   `json:"full_name" binding:"required"`
	Password string   `json:"password"`
	Roles    []string `json:"roles" binding:"required,min=1"`
	Status   string   `json:"status" binding:"required,oneof=active inactive locked"`
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
	if id == actor && req.Status != "active" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tidak bisa menonaktifkan akun sendiri"})
		return
	}
	if id == actor && !hasRole(roles, "admin") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tidak bisa menghapus role admin dari akun sendiri"})
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
	}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// DeactivateUser menonaktifkan pengguna (E01-7). Pengalihan surat pending
// menyusul di Epic E03 saat workflow sudah ada.
func (h *Handler) DeactivateUser(c *gin.Context) {
	id := c.Param("id")
	actor := c.GetString(middleware.CtxUserID)
	if id == actor {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tidak bisa menonaktifkan akun sendiri"})
		return
	}
	ctx := c.Request.Context()

	tag, err := h.DB.Exec(ctx,
		`UPDATE users SET status = 'inactive', updated_at = now() WHERE id = $1 AND status <> 'inactive'`, id)
	if err != nil || tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "pengguna tidak ditemukan atau sudah nonaktif"})
		return
	}

	h.audit(ctx, "user", &id, "deactivate", &actor, nil, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
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
