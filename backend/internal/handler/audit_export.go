package handler

import (
	"encoding/csv"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

const auditExportLimit = 10000

// ExportAuditLetters returns metadata for published letters inside the
// auditor's assigned scope. Letter content and attachments are intentionally
// excluded from exports.
func (h *Handler) ExportAuditLetters(c *gin.Context) {
	from, to, ok := parseAuditExportRange(c)
	if !ok {
		return
	}
	userID := c.GetString(middleware.CtxUserID)
	ctx := c.Request.Context()
	canExport, err := h.userHasAuditExport(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa otoritas ekspor audit"})
		return
	}
	if !canExport {
		c.JSON(http.StatusForbidden, gin.H{"error": "Anda tidak memiliki izin ekspor audit aktif"})
		return
	}

	rows, err := h.DB.Query(ctx, `
		SELECT l.id::text, COALESCE(l.letter_number, ''), l.subject,
		       l.classification, lt.code, co.code, u.full_name, cp.title,
		       ou.name, l.published_at::date::text
		FROM letters l
		JOIN companies co ON co.id = l.company_id
		JOIN letter_types lt ON lt.id = l.letter_type_id
		JOIN users u ON u.id = l.creator_user_id
		JOIN positions cp ON cp.id = l.creator_position_id
		JOIN org_units ou ON ou.id = cp.org_unit_id
		WHERE l.status = 'published'
		  AND `+auditLetterExportAccessSQL("$1", "l")+`
		  AND l.published_at >= $2::date
		  AND l.published_at < ($3::date + interval '1 day')
		ORDER BY l.published_at DESC, l.updated_at DESC
		LIMIT $4`, userID, from.Format(time.DateOnly), to.Format(time.DateOnly), auditExportLimit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyiapkan ekspor audit"})
		return
	}
	defer rows.Close()

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="audit-letters-`+from.Format("20060102")+`-`+to.Format("20060102")+`.csv"`)
	c.Writer.WriteString("\ufeff") // UTF-8 BOM for spreadsheet applications.
	writer := csv.NewWriter(c.Writer)
	if err := writer.Write([]string{
		"ID Surat", "Nomor Surat", "Perihal", "Klasifikasi", "Jenis Surat",
		"Perusahaan", "Pembuat", "Jabatan Pembuat", "Unit Pembuat", "Tanggal Terbit",
	}); err != nil {
		return
	}

	count := 0
	for rows.Next() {
		values := make([]string, 10)
		if err := rows.Scan(
			&values[0], &values[1], &values[2], &values[3], &values[4],
			&values[5], &values[6], &values[7], &values[8], &values[9],
		); err != nil {
			return
		}
		for index := range values {
			values[index] = csvSafeValue(values[index])
		}
		if err := writer.Write(values); err != nil {
			return
		}
		count++
	}
	if rows.Err() != nil {
		return
	}
	writer.Flush()
	if writer.Error() != nil {
		return
	}

	h.audit(ctx, "audit_export", nil, "export", &userID, map[string]any{
		"from": from.Format(time.DateOnly), "to": to.Format(time.DateOnly), "rows": count,
	}, c.ClientIP())
}

func parseAuditExportRange(c *gin.Context) (time.Time, time.Time, bool) {
	to := time.Now().UTC()
	from := to.AddDate(0, 0, -30)
	if value := strings.TrimSpace(c.Query("from")); value != "" {
		parsed, err := time.Parse(time.DateOnly, value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "tanggal dari harus berformat YYYY-MM-DD"})
			return time.Time{}, time.Time{}, false
		}
		from = parsed
	}
	if value := strings.TrimSpace(c.Query("to")); value != "" {
		parsed, err := time.Parse(time.DateOnly, value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "tanggal sampai harus berformat YYYY-MM-DD"})
			return time.Time{}, time.Time{}, false
		}
		to = parsed
	}
	if from.After(to) || to.Sub(from) > 366*24*time.Hour {
		c.JSON(http.StatusBadRequest, gin.H{"error": "periode ekspor maksimal 366 hari dan tanggal mulai tidak boleh melebihi tanggal selesai"})
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}

func csvSafeValue(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if len(trimmed) > 0 && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}
