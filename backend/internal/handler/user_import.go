package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"github.com/kskgroup/eofficepro/internal/auth"
	"github.com/kskgroup/eofficepro/internal/middleware"
)

const maxImportFileSize = 5 << 20 // 5 MB

var importColumns = []string{
	"nik", "email", "full_name", "password", "roles", "org_unit_code", "position_title",
}

// ImportTemplate mengunduh template xlsx dengan header + satu baris contoh.
func (h *Handler) ImportTemplate(c *gin.Context) {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	_ = f.SetSheetRow(sheet, "A1", &importColumns)
	example := []string{"EMP001", "budi@ksk.co.id", "Budi Santoso", "PasswordAwal#01", "creator,approver", "IT-SW", "Staff IT Software"}
	_ = f.SetSheetRow(sheet, "A2", &example)

	c.Header("Content-Disposition", `attachment; filename="template-import-pengguna.xlsx"`)
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	if err := f.Write(c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membuat template"})
	}
}

type importRowError struct {
	Row   int    `json:"row"`
	Error string `json:"error"`
}

// ImportUsers (E01-6) membaca xlsx dan membuat pengguna per baris.
// Baris valid tetap tersimpan meski baris lain gagal; semua kegagalan
// dilaporkan per baris.
func (h *Handler) ImportUsers(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lampirkan file xlsx pada field 'file'"})
		return
	}
	if fileHeader.Size > maxImportFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ukuran file maksimal 5 MB"})
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file tidak bisa dibuka"})
		return
	}
	defer file.Close()

	xl, err := excelize.OpenReader(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format file bukan xlsx yang valid"})
		return
	}
	defer xl.Close()

	rows, err := xl.GetRows(xl.GetSheetName(0))
	if err != nil || len(rows) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file kosong atau tidak punya baris data"})
		return
	}

	// Validasi header sesuai template.
	header := rows[0]
	for i, want := range importColumns {
		if i >= len(header) || !strings.EqualFold(strings.TrimSpace(header[i]), want) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("header kolom %d harus '%s' — unduh template dari GET /users/import/template", i+1, want),
			})
			return
		}
	}

	ctx := c.Request.Context()

	validRoles := map[string]bool{}
	roleRows, err := h.DB.Query(ctx, `SELECT code FROM roles`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat role"})
		return
	}
	for roleRows.Next() {
		var code string
		if roleRows.Scan(&code) == nil {
			validRoles[code] = true
		}
	}
	roleRows.Close()

	cell := func(row []string, idx int) string {
		if idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	imported := 0
	errorsList := []importRowError{}

	for i, row := range rows[1:] {
		rowNum := i + 2 // nomor baris di file (1-based, setelah header)
		nik, email, fullName := cell(row, 0), cell(row, 1), cell(row, 2)
		password, rolesRaw := cell(row, 3), cell(row, 4)
		orgUnitCode, positionTitle := cell(row, 5), cell(row, 6)

		fail := func(msg string) {
			errorsList = append(errorsList, importRowError{Row: rowNum, Error: msg})
		}

		if nik == "" && email == "" && fullName == "" {
			continue // baris kosong dilewati tanpa error
		}
		if nik == "" || email == "" || fullName == "" {
			fail("nik, email, dan full_name wajib diisi")
			continue
		}
		if !strings.Contains(email, "@") {
			fail("format email tidak valid")
			continue
		}
		if len(password) < 10 {
			fail("password minimal 10 karakter")
			continue
		}
		roles := []string{}
		for _, r := range strings.Split(rolesRaw, ",") {
			if r = strings.TrimSpace(r); r != "" {
				if !validRoles[r] {
					roles = nil
					fail("role tidak dikenal: " + r)
					break
				}
				roles = append(roles, r)
			}
		}
		if roles == nil {
			continue
		}
		if len(roles) == 0 {
			fail("minimal satu role wajib diisi")
			continue
		}
		if (orgUnitCode == "") != (positionTitle == "") {
			fail("org_unit_code dan position_title harus diisi berpasangan atau dikosongkan keduanya")
			continue
		}

		var positionID string
		if orgUnitCode != "" {
			err := h.DB.QueryRow(ctx, `
				SELECT p.id::text FROM positions p
				JOIN org_units ou ON ou.id = p.org_unit_id
				WHERE ou.code = $1 AND lower(p.title) = lower($2) AND p.is_active AND ou.is_active`,
				orgUnitCode, positionTitle).Scan(&positionID)
			if err != nil {
				fail(fmt.Sprintf("jabatan '%s' pada unit '%s' tidak ditemukan", positionTitle, orgUnitCode))
				continue
			}
		}

		hash, err := auth.HashPassword(password)
		if err != nil {
			fail("gagal memproses password")
			continue
		}

		// Transaksi per baris: baris gagal tidak membatalkan baris lain.
		tx, err := h.DB.Begin(ctx)
		if err != nil {
			fail("gagal memulai transaksi")
			continue
		}
		var userID string
		err = tx.QueryRow(ctx, `
			INSERT INTO users (nik, email, full_name, password_hash)
			VALUES ($1, $2, $3, $4) RETURNING id::text`,
			nik, email, fullName, hash).Scan(&userID)
		if err != nil {
			tx.Rollback(ctx)
			fail("NIK atau email sudah terdaftar")
			continue
		}
		rowOK := true
		for _, role := range roles {
			if _, err := tx.Exec(ctx, `
				INSERT INTO user_roles (user_id, role_id)
				SELECT $1, id FROM roles WHERE code = $2`, userID, role); err != nil {
				rowOK = false
				break
			}
		}
		if rowOK && positionID != "" {
			if _, err := tx.Exec(ctx, `
				INSERT INTO user_positions (user_id, position_id, assignment_type)
				VALUES ($1, $2, 'definitive')`, userID, positionID); err != nil {
				rowOK = false
			}
		}
		if !rowOK {
			tx.Rollback(ctx)
			fail("gagal menyimpan role/penempatan")
			continue
		}
		if err := tx.Commit(ctx); err != nil {
			fail("gagal menyimpan baris")
			continue
		}
		imported++
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "user", nil, "import", &actor,
		map[string]any{"file": fileHeader.Filename, "imported": imported, "failed": len(errorsList)},
		c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"imported": imported,
		"failed":   len(errorsList),
		"errors":   errorsList,
	})
}
