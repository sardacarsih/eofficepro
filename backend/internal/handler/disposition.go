package handler

// Disposisi (E05-3/4): pimpinan penerima surat memberi instruksi + tenggat ke
// jabatan lain, bisa berantai (parent_disposition_id). Penerima memperbarui
// status tindak lanjut (open → in_progress → done + laporan).

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

type createDispositionRequest struct {
	FromPositionID       string   `json:"from_position_id" binding:"required"`
	Instruction          string   `json:"instruction" binding:"required"`
	DueDate              *string  `json:"due_date"` // YYYY-MM-DD
	ParentDispositionID  *string  `json:"parent_disposition_id"`
	RecipientPositionIDs []string `json:"recipient_position_ids" binding:"required,min=1"`
}

type dispositionRecipient struct {
	ID            string     `json:"id"`
	PositionID    string     `json:"position_id"`
	PositionTitle string     `json:"position_title"`
	HolderName    string     `json:"holder_name"`
	Status        string     `json:"status"`
	FollowupNote  *string    `json:"followup_note"`
	CompletedAt   *time.Time `json:"completed_at"`
}

type dispositionItem struct {
	ID                  string                 `json:"id"`
	LetterID            string                 `json:"letter_id"`
	ParentDispositionID *string                `json:"parent_disposition_id"`
	FromPositionID      string                 `json:"from_position_id"`
	FromPositionTitle   string                 `json:"from_position_title"`
	CreatorName         string                 `json:"creator_name"`
	Instruction         string                 `json:"instruction"`
	DueDate             *string                `json:"due_date"`
	CreatedAt           time.Time              `json:"created_at"`
	Recipients          []dispositionRecipient `json:"recipients"`
}

// userHoldsPosition memastikan pengguna memegang jabatan tersebut hari ini.
func (h *Handler) userHoldsPosition(ctx context.Context, userID, positionID string) (bool, error) {
	var holds bool
	err := h.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM user_positions up
			WHERE up.user_id = $1
			  AND up.position_id = $2
			  AND current_date >= up.valid_from
			  AND (up.valid_to IS NULL OR current_date < up.valid_to)
		)`, userID, positionID).Scan(&holds)
	return holds, err
}

// userIsDispositionParticipant: pengguna memegang jabatan pengirim atau
// penerima salah satu disposisi surat ini.
func (h *Handler) userIsDispositionParticipant(ctx context.Context, userID, letterID string) (bool, error) {
	var ok bool
	err := h.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM dispositions d
			LEFT JOIN disposition_recipients dr ON dr.disposition_id = d.id
			JOIN user_positions up
			  ON up.position_id IN (d.from_position_id, dr.position_id)
			WHERE d.letter_id = $1
			  AND up.user_id = $2
			  AND current_date >= up.valid_from
			  AND (up.valid_to IS NULL OR current_date < up.valid_to)
		)`, letterID, userID).Scan(&ok)
	return ok, err
}

func (h *Handler) CreateDisposition(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	letterID := c.Param("id")
	ctx := c.Request.Context()

	var req createDispositionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instruksi, jabatan pengirim, dan minimal satu penerima wajib diisi"})
		return
	}
	req.Instruction = strings.TrimSpace(req.Instruction)
	if req.Instruction == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instruksi wajib diisi"})
		return
	}

	var dueDate *time.Time
	if req.DueDate != nil && strings.TrimSpace(*req.DueDate) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*req.DueDate))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "format tenggat harus YYYY-MM-DD"})
			return
		}
		dueDate = &parsed
	}

	// Pengirim harus memegang jabatan pengirim.
	holds, err := h.userHoldsPosition(ctx, userID, req.FromPositionID)
	if err != nil || !holds {
		c.JSON(http.StatusForbidden, gin.H{"error": "Anda tidak memegang jabatan pengirim disposisi ini"})
		return
	}

	// Surat harus terbit.
	var letterStatus, subject string
	if err := h.DB.QueryRow(ctx,
		`SELECT status, subject FROM letters WHERE id = $1`, letterID).
		Scan(&letterStatus, &subject); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "surat tidak ditemukan"})
		return
	}
	if letterStatus != "published" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "disposisi hanya untuk surat yang sudah terbit"})
		return
	}

	// Akses: penerima surat, atau (untuk disposisi berantai) penerima
	// disposisi induk.
	if req.ParentDispositionID != nil && *req.ParentDispositionID != "" {
		var validParent bool
		err := h.DB.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM dispositions d
				JOIN disposition_recipients dr ON dr.disposition_id = d.id
				JOIN user_positions up ON up.position_id = dr.position_id
				WHERE d.id = $1
				  AND d.letter_id = $2
				  AND up.user_id = $3
				  AND current_date >= up.valid_from
				  AND (up.valid_to IS NULL OR current_date < up.valid_to)
			)`, *req.ParentDispositionID, letterID, userID).Scan(&validParent)
		if err != nil || !validParent {
			c.JSON(http.StatusForbidden, gin.H{"error": "disposisi induk tidak valid atau bukan ditujukan ke Anda"})
			return
		}
	} else {
		canAccess, err := h.userCanReceivePublishedLetter(ctx, userID, letterID)
		if err != nil || !canAccess {
			c.JSON(http.StatusForbidden, gin.H{"error": "Anda bukan penerima surat ini"})
			return
		}
	}

	// Validasi penerima: jabatan aktif, unik, bukan jabatan pengirim.
	seen := map[string]bool{}
	recipients := []string{}
	for _, positionID := range req.RecipientPositionIDs {
		positionID = strings.TrimSpace(positionID)
		if positionID == "" || seen[positionID] {
			continue
		}
		if positionID == req.FromPositionID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "penerima disposisi tidak boleh jabatan pengirim sendiri"})
			return
		}
		seen[positionID] = true
		recipients = append(recipients, positionID)
	}
	if len(recipients) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "minimal satu penerima disposisi"})
		return
	}
	var activeCount int
	if err := h.DB.QueryRow(ctx, `
		SELECT count(*) FROM positions
		WHERE id = ANY($1::uuid[]) AND is_active`, recipients).Scan(&activeCount); err != nil || activeCount != len(recipients) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ada jabatan penerima yang tidak valid atau nonaktif"})
		return
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi"})
		return
	}
	defer tx.Rollback(ctx)

	var parentArg any
	if req.ParentDispositionID != nil && *req.ParentDispositionID != "" {
		parentArg = *req.ParentDispositionID
	}
	var dispositionID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO dispositions (letter_id, parent_disposition_id, from_position_id, instruction, due_date, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id::text`,
		letterID, parentArg, req.FromPositionID, req.Instruction, dueDate, userID).Scan(&dispositionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan disposisi"})
		return
	}
	for _, positionID := range recipients {
		if _, err := tx.Exec(ctx, `
			INSERT INTO disposition_recipients (disposition_id, position_id)
			VALUES ($1, $2)`, dispositionID, positionID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan penerima disposisi"})
			return
		}
	}

	// Notifikasi ke seluruh pemegang aktif jabatan penerima.
	emailRows, err := tx.Query(ctx, `
		WITH inserted AS (
			INSERT INTO notifications (user_id, event_type, letter_id, title, body)
			SELECT DISTINCT up.user_id, 'disposition_assigned', $2::uuid,
			       'Disposisi baru: ' || $3::text,
			       'Instruksi dari ' || p.title || ': ' || left($4::text, 140)
			         || CASE WHEN $5::date IS NULL THEN '' ELSE ' (tenggat ' || to_char($5::date, 'DD Mon YYYY') || ')' END
			FROM disposition_recipients dr
			JOIN user_positions up ON up.position_id = dr.position_id
			JOIN users u ON u.id = up.user_id
			JOIN dispositions d ON d.id = dr.disposition_id
			JOIN positions p ON p.id = d.from_position_id
			WHERE dr.disposition_id = $1
			  AND current_date >= up.valid_from
			  AND (up.valid_to IS NULL OR current_date < up.valid_to)
			  AND u.status = 'active'
			RETURNING user_id, event_type, letter_id, title, body
		)
		SELECT u.email, i.event_type, i.letter_id::text, i.title, i.body
		FROM inserted i
		JOIN users u ON u.id = i.user_id`,
		dispositionID, letterID, subject, req.Instruction, dueDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengirim notifikasi disposisi"})
		return
	}
	emails, err := collectNotificationEmails(emailRows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca notifikasi disposisi"})
		return
	}

	if err := enqueueNotificationOutbox(ctx, tx, emails); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengantrikan notifikasi disposisi"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan disposisi"})
		return
	}

	h.audit(ctx, "disposition", &dispositionID, "create", &userID, map[string]any{
		"letter_id":  letterID,
		"recipients": len(recipients),
	}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": dispositionID})
}

// ListLetterDispositions mengembalikan seluruh disposisi sebuah surat untuk
// pembuat surat, penerima surat, atau partisipan disposisi.
func (h *Handler) ListLetterDispositions(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	letterID := c.Param("id")
	ctx := c.Request.Context()

	page, pageSize, offset, ok := parsePagination(c.Query("page"), c.Query("page_size"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page atau page_size tidak valid"})
		return
	}

	var isCreator bool
	var letterStatus string
	if err := h.DB.QueryRow(ctx,
		`SELECT creator_user_id = $2, status FROM letters WHERE id = $1`, letterID, userID).
		Scan(&isCreator, &letterStatus); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "surat tidak ditemukan"})
		return
	}
	if !isCreator {
		canAccess, err := h.userCanReceivePublishedLetter(ctx, userID, letterID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa akses"})
			return
		}
		if !canAccess {
			participant, err := h.userIsDispositionParticipant(ctx, userID, letterID)
			if err != nil || !participant {
				// Penerima surat yang belum terbit bukan masalah akses:
				// disposisi memang baru tersedia setelah surat published.
				if letterStatus != "published" {
					recipient, recErr := h.userIsLetterRecipient(ctx, userID, letterID)
					if recErr == nil && recipient {
						c.JSON(http.StatusForbidden, gin.H{"error": "Disposisi tersedia setelah surat terbit"})
						return
					}
				}
				c.JSON(http.StatusForbidden, gin.H{"error": "Anda tidak memiliki akses ke disposisi surat ini"})
				return
			}
		}
	}

	var total int64
	if err := h.DB.QueryRow(ctx, `SELECT count(*) FROM dispositions WHERE letter_id = $1`, letterID).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menghitung disposisi"})
		return
	}

	items, err := h.loadDispositions(ctx, letterID, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat disposisi"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "meta": newPageMeta(page, pageSize, total)})
}

func (h *Handler) loadDispositions(ctx context.Context, letterID string, pageSize, offset int) ([]dispositionItem, error) {
	rows, err := h.DB.Query(ctx, `
		SELECT d.id::text, d.letter_id::text, d.parent_disposition_id::text,
		       d.from_position_id::text, p.title, u.full_name,
		       d.instruction, d.due_date::text, d.created_at
		FROM dispositions d
		JOIN positions p ON p.id = d.from_position_id
		JOIN users u ON u.id = d.created_by
		WHERE d.letter_id = $1
		ORDER BY d.created_at
		LIMIT $2 OFFSET $3`, letterID, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []dispositionItem{}
	index := map[string]int{}
	for rows.Next() {
		var item dispositionItem
		if err := rows.Scan(
			&item.ID,
			&item.LetterID,
			&item.ParentDispositionID,
			&item.FromPositionID,
			&item.FromPositionTitle,
			&item.CreatorName,
			&item.Instruction,
			&item.DueDate,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.Recipients = []dispositionRecipient{}
		index[item.ID] = len(items)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return items, nil
	}

	recipientRows, err := h.DB.Query(ctx, `
		SELECT dr.id::text, dr.disposition_id::text, dr.position_id::text, p.title,
		       COALESCE(holder.full_name, ''), dr.status, dr.followup_note, dr.completed_at
		FROM disposition_recipients dr
		JOIN dispositions d ON d.id = dr.disposition_id
		JOIN positions p ON p.id = dr.position_id
		LEFT JOIN LATERAL (
			SELECT u.full_name
			FROM user_positions up
			JOIN users u ON u.id = up.user_id
			WHERE up.position_id = dr.position_id
			  AND current_date >= up.valid_from
			  AND (up.valid_to IS NULL OR current_date < up.valid_to)
			  AND u.status = 'active'
			ORDER BY up.valid_from DESC, up.id
			LIMIT 1
		) holder ON true
		WHERE d.letter_id = $1
		ORDER BY p.title`, letterID)
	if err != nil {
		return nil, err
	}
	defer recipientRows.Close()
	for recipientRows.Next() {
		var recipient dispositionRecipient
		var dispositionID string
		if err := recipientRows.Scan(
			&recipient.ID,
			&dispositionID,
			&recipient.PositionID,
			&recipient.PositionTitle,
			&recipient.HolderName,
			&recipient.Status,
			&recipient.FollowupNote,
			&recipient.CompletedAt,
		); err != nil {
			return nil, err
		}
		if i, ok := index[dispositionID]; ok {
			items[i].Recipients = append(items[i].Recipients, recipient)
		}
	}
	return items, recipientRows.Err()
}

type dispositionInboxItem struct {
	RecipientID       string     `json:"recipient_id"`
	DispositionID     string     `json:"disposition_id"`
	LetterID          string     `json:"letter_id"`
	LetterSubject     string     `json:"letter_subject"`
	LetterNumber      *string    `json:"letter_number"`
	FromPositionTitle string     `json:"from_position_title"`
	CreatorName       string     `json:"creator_name"`
	MyPositionTitle   string     `json:"my_position_title"`
	Instruction       string     `json:"instruction"`
	DueDate           *string    `json:"due_date"`
	Status            string     `json:"status"`
	FollowupNote      *string    `json:"followup_note"`
	CompletedAt       *time.Time `json:"completed_at"`
	CreatedAt         time.Time  `json:"created_at"`
}

// ListDispositionInbox — disposisi yang ditujukan ke jabatan aktif pengguna.
func (h *Handler) ListDispositionInbox(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	ctx := c.Request.Context()

	page, pageSize, offset, ok := parsePagination(c.Query("page"), c.Query("page_size"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page atau page_size tidak valid"})
		return
	}

	inboxCTE := `
		SELECT DISTINCT ON (dr.id)
		       dr.id::text, d.id::text, l.id::text, l.subject, l.letter_number,
		       fp.title, cu.full_name, mp.title,
		       d.instruction, d.due_date::text, dr.status, dr.followup_note,
		       dr.completed_at, d.created_at
		FROM disposition_recipients dr
		JOIN dispositions d ON d.id = dr.disposition_id
		JOIN letters l ON l.id = d.letter_id
		JOIN positions fp ON fp.id = d.from_position_id
		JOIN positions mp ON mp.id = dr.position_id
		JOIN users cu ON cu.id = d.created_by
		JOIN user_positions up ON up.position_id = dr.position_id
		WHERE up.user_id = $1
		  AND current_date >= up.valid_from
		  AND (up.valid_to IS NULL OR current_date < up.valid_to)
		ORDER BY dr.id, d.created_at DESC`

	var total int64
	if err := h.DB.QueryRow(ctx, `SELECT count(*) FROM (`+inboxCTE+`) inbox`, userID).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menghitung inbox disposisi"})
		return
	}

	rows, err := h.DB.Query(ctx, `
		SELECT *
		FROM (`+inboxCTE+`) inbox
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, userID, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat inbox disposisi"})
		return
	}
	defer rows.Close()

	items := []dispositionInboxItem{}
	for rows.Next() {
		var item dispositionInboxItem
		if err := rows.Scan(
			&item.RecipientID,
			&item.DispositionID,
			&item.LetterID,
			&item.LetterSubject,
			&item.LetterNumber,
			&item.FromPositionTitle,
			&item.CreatorName,
			&item.MyPositionTitle,
			&item.Instruction,
			&item.DueDate,
			&item.Status,
			&item.FollowupNote,
			&item.CompletedAt,
			&item.CreatedAt,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca inbox disposisi"})
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar disposisi"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": items, "meta": newPageMeta(page, pageSize, total)})
}

type updateDispositionStatusRequest struct {
	Status       string `json:"status" binding:"required"`
	FollowupNote string `json:"followup_note"`
}

func updateDispositionRecipient(
	ctx context.Context,
	tx pgx.Tx,
	recipientID string,
	status string,
	followupNote string,
) error {
	var noteArg any
	if followupNote != "" {
		noteArg = followupNote
	}
	tag, err := tx.Exec(ctx, `
		UPDATE disposition_recipients
		SET status = $2::varchar,
		    followup_note = COALESCE($3::text, followup_note),
		    completed_at = CASE
		        WHEN $2::varchar = 'done' THEN now()
		        ELSE completed_at
		    END
		WHERE id = $1`, recipientID, status, noteArg)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (h *Handler) UpdateDispositionStatus(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserID)
	recipientID := c.Param("id")
	ctx := c.Request.Context()

	var req updateDispositionStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status wajib diisi"})
		return
	}
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	req.FollowupNote = strings.TrimSpace(req.FollowupNote)
	if req.Status != "in_progress" && req.Status != "done" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status harus in_progress atau done"})
		return
	}
	if req.Status == "done" && req.FollowupNote == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "laporan tindak lanjut wajib diisi saat menyelesaikan disposisi"})
		return
	}

	// Hanya pemegang aktif jabatan penerima yang boleh memperbarui.
	var dispositionID, letterID, positionID, currentStatus string
	err := h.DB.QueryRow(ctx, `
		SELECT dr.disposition_id::text, d.letter_id::text, dr.position_id::text, dr.status
		FROM disposition_recipients dr
		JOIN dispositions d ON d.id = dr.disposition_id
		JOIN user_positions up ON up.position_id = dr.position_id
		WHERE dr.id = $1
		  AND up.user_id = $2
		  AND current_date >= up.valid_from
		  AND (up.valid_to IS NULL OR current_date < up.valid_to)`, recipientID, userID).
		Scan(&dispositionID, &letterID, &positionID, &currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "disposisi tidak ditemukan atau bukan ditujukan ke Anda"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca disposisi"})
		return
	}
	if currentStatus == "done" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "disposisi sudah selesai"})
		return
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai transaksi"})
		return
	}
	defer tx.Rollback(ctx)

	if err := updateDispositionRecipient(
		ctx,
		tx,
		recipientID,
		req.Status,
		req.FollowupNote,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memperbarui status disposisi"})
		return
	}

	// Beri tahu pembuat disposisi.
	statusLabel := "diproses"
	if req.Status == "done" {
		statusLabel = "selesai"
	}
	emailRows, err := tx.Query(ctx, `
		WITH inserted AS (
			INSERT INTO notifications (user_id, event_type, letter_id, title, body)
			SELECT d.created_by, 'disposition_updated', d.letter_id,
			       'Disposisi ' || $2::text || ': ' || l.subject,
			       'Tindak lanjut oleh ' || p.title
			         || CASE WHEN $3::text = '' THEN '' ELSE ' — ' || left($3::text, 140) END
			FROM dispositions d
			JOIN letters l ON l.id = d.letter_id
			JOIN positions p ON p.id = $4::uuid
			WHERE d.id = $1
			RETURNING user_id, event_type, letter_id, title, body
		)
		SELECT u.email, i.event_type, i.letter_id::text, i.title, i.body
		FROM inserted i
		JOIN users u ON u.id = i.user_id`,
		dispositionID, statusLabel, req.FollowupNote, positionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengirim notifikasi tindak lanjut"})
		return
	}
	emails, err := collectNotificationEmails(emailRows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca notifikasi tindak lanjut"})
		return
	}

	if err := enqueueNotificationOutbox(ctx, tx, emails); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengantrikan notifikasi tindak lanjut"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan status disposisi"})
		return
	}

	h.audit(ctx, "disposition", &dispositionID, "followup_"+req.Status, &userID, map[string]any{
		"letter_id":    letterID,
		"recipient_id": recipientID,
	}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": recipientID, "status": req.Status})
}
