package handler

// Pencarian surat (awal E07-2): ILIKE atas nomor, perihal, dan isi surat.
// Cakupan akses: surat yang dibuat pengguna sendiri (semua status) dan surat
// terbit yang diterimanya sebagai To/CC — klasifikasi akses tetap terjaga.

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

const searchResultLimit = 30

type letterSearchResult struct {
	ID             string     `json:"id"`
	CompanyCode    string     `json:"company_code"`
	LetterTypeCode string     `json:"letter_type_code"`
	LetterNumber   *string    `json:"letter_number"`
	Subject        string     `json:"subject"`
	Status         string     `json:"status"`
	Classification string     `json:"classification"`
	CreatorName    string     `json:"creator_name"`
	Origin         string     `json:"origin"` // mine | received | audit
	Snippet        string     `json:"snippet"`
	PublishedAt    *time.Time `json:"published_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (h *Handler) SearchLetters(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	query := strings.TrimSpace(c.Query("q"))
	if len(query) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kata kunci minimal 2 karakter"})
		return
	}
	pattern := "%" + escapeLike(query) + "%"

	rows, err := h.DB.Query(c.Request.Context(), `
		WITH candidates AS (
			SELECT l.id, 'mine' AS origin
			FROM letters l
			WHERE l.creator_user_id = $1
			UNION ALL
			SELECT DISTINCT l.id, 'received' AS origin
			FROM letter_recipients lr
			JOIN letters l ON l.id = lr.letter_id
			WHERE l.status = 'published'
			  AND l.creator_user_id <> $1
			  AND `+publishedRecipientAccessSQL("$1")+`
			UNION ALL
			SELECT l.id, 'audit' AS origin
			FROM letters l
			WHERE `+auditLetterAccessSQL("$1", "l")+`
		), accessible AS (
			SELECT DISTINCT ON (id) id, origin
			FROM candidates
			ORDER BY id,
				CASE origin WHEN 'mine' THEN 1 WHEN 'received' THEN 2 ELSE 3 END
		)
		SELECT l.id::text, co.code, lt.code, l.letter_number, l.subject, l.status,
		       l.classification, u.full_name, a.origin,
		       COALESCE(v.body_plain, ''), l.published_at, l.updated_at
		FROM accessible a
		JOIN letters l ON l.id = a.id
		JOIN companies co ON co.id = l.company_id
		JOIN letter_types lt ON lt.id = l.letter_type_id
		JOIN users u ON u.id = l.creator_user_id
		LEFT JOIN LATERAL (
			SELECT body_plain
			FROM letter_versions
			WHERE letter_id = l.id
			ORDER BY version DESC
			LIMIT 1
		) v ON true
		WHERE l.subject ILIKE $2
		   OR l.letter_number ILIKE $2
		   OR COALESCE(v.body_plain, '') ILIKE $2
		ORDER BY l.updated_at DESC
		LIMIT $3`, userID, pattern, searchResultLimit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mencari surat"})
		return
	}
	defer rows.Close()

	results := []letterSearchResult{}
	for rows.Next() {
		var item letterSearchResult
		var bodyPlain string
		if err := rows.Scan(
			&item.ID,
			&item.CompanyCode,
			&item.LetterTypeCode,
			&item.LetterNumber,
			&item.Subject,
			&item.Status,
			&item.Classification,
			&item.CreatorName,
			&item.Origin,
			&bodyPlain,
			&item.PublishedAt,
			&item.UpdatedAt,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca hasil pencarian"})
			return
		}
		item.Snippet = searchSnippet(bodyPlain, query)
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar hasil pencarian"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"results": results, "query": query})
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

// searchSnippet memotong isi surat di sekitar kemunculan pertama kata kunci.
// Bekerja per-rune agar aman untuk karakter multibyte.
func searchSnippet(body string, query string) string {
	body = strings.Join(strings.Fields(body), " ")
	if body == "" {
		return ""
	}
	const window = 160
	runes := []rune(body)

	idx := -1
	if byteIdx := strings.Index(strings.ToLower(body), strings.ToLower(query)); byteIdx >= 0 {
		idx = utf8.RuneCountInString(body[:byteIdx])
	}
	if idx < 0 {
		if len(runes) > window {
			return string(runes[:window]) + "..."
		}
		return body
	}

	start := max(idx-window/3, 0)
	end := min(start+window, len(runes))
	snippet := string(runes[start:end])
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(runes) {
		snippet += "..."
	}
	return snippet
}
