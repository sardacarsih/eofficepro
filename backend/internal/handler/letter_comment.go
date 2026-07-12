package handler

// Komentar internal surat (E05-5): diskusi ringan antar pengguna yang berhak
// melihat surat. Append-only, plain text, tidak memengaruhi isi resmi, status,
// versi, maupun alur approval surat.

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

const letterCommentMaxChars = 2000

type letterCommentItem struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	UserName      string    `json:"user_name"`
	PositionTitle *string   `json:"position_title"`
	Body          string    `json:"body"`
	CreatedAt     time.Time `json:"created_at"`
}

// letterCommentHolderSQL memilih jabatan aktif komentator saat ini (boleh
// kosong bila penugasannya sudah berakhir); pola sama dengan holder lookup
// pada disposisi.
const letterCommentHolderSQL = `
	LEFT JOIN LATERAL (
		SELECT p.title
		FROM user_positions up
		JOIN positions p ON p.id = up.position_id
		WHERE up.user_id = lc.user_id
		  AND current_date >= up.valid_from
		  AND (up.valid_to IS NULL OR current_date < up.valid_to)
		  AND p.is_active
		ORDER BY up.valid_from DESC, up.id
		LIMIT 1
	) holder ON true`

// ListLetterComments mengembalikan komentar sebuah surat terurut kronologis
// untuk setiap pengguna yang lolos cek akses surat.
func (h *Handler) ListLetterComments(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	letterID := c.Param("id")
	ctx := c.Request.Context()

	page, pageSize, offset, ok := parsePagination(c.Query("page"), c.Query("page_size"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page atau page_size tidak valid"})
		return
	}

	allowed, err := h.userCanViewLetter(ctx, userID, letterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa akses surat"})
		return
	}
	if !allowed {
		c.JSON(http.StatusNotFound, gin.H{"error": "surat tidak ditemukan"})
		return
	}

	var total int64
	if err := h.DB.QueryRow(ctx,
		`SELECT count(*) FROM letter_comments WHERE letter_id = $1`, letterID).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menghitung komentar surat"})
		return
	}

	rows, err := h.DB.Query(ctx, `
		SELECT lc.id::text, lc.user_id::text, u.full_name, holder.title, lc.body, lc.created_at
		FROM letter_comments lc
		JOIN users u ON u.id = lc.user_id
		`+letterCommentHolderSQL+`
		WHERE lc.letter_id = $1
		ORDER BY lc.created_at, lc.id
		LIMIT $2 OFFSET $3`, letterID, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat komentar surat"})
		return
	}
	defer rows.Close()

	comments := []letterCommentItem{}
	for rows.Next() {
		var item letterCommentItem
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.UserName,
			&item.PositionTitle,
			&item.Body,
			&item.CreatedAt,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca komentar surat"})
			return
		}
		comments = append(comments, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar komentar surat"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": comments, "meta": newPageMeta(page, pageSize, total)})
}

type createLetterCommentRequest struct {
	Body string `json:"body"`
}

// CreateLetterComment menambah komentar baru pada surat yang boleh dilihat
// pengguna; komentar tidak dapat diedit atau dihapus setelah dibuat.
func (h *Handler) CreateLetterComment(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	letterID := c.Param("id")
	ctx := c.Request.Context()

	var req createLetterCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "isi komentar wajib diisi"})
		return
	}
	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "isi komentar wajib diisi"})
		return
	}
	if utf8.RuneCountInString(req.Body) > letterCommentMaxChars {
		c.JSON(http.StatusBadRequest, gin.H{"error": "isi komentar maksimal 2000 karakter"})
		return
	}

	allowed, err := h.userCanViewLetter(ctx, userID, letterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa akses surat"})
		return
	}
	if !allowed {
		c.JSON(http.StatusNotFound, gin.H{"error": "surat tidak ditemukan"})
		return
	}

	var item letterCommentItem
	if err := h.DB.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO letter_comments (letter_id, user_id, body)
			VALUES ($1, $2, $3)
			RETURNING id, user_id, body, created_at
		)
		SELECT lc.id::text, lc.user_id::text, u.full_name, holder.title, lc.body, lc.created_at
		FROM inserted lc
		JOIN users u ON u.id = lc.user_id
		`+letterCommentHolderSQL,
		letterID, userID, req.Body).Scan(
		&item.ID,
		&item.UserID,
		&item.UserName,
		&item.PositionTitle,
		&item.Body,
		&item.CreatedAt,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan komentar surat"})
		return
	}

	h.audit(ctx, "letter_comment", &item.ID, "create", &userID, map[string]any{
		"letter_id": letterID,
	}, c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"comment": item})
}
