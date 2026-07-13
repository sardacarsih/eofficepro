package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

func cancelLetterReq(t *testing.T, h *Handler, userID, letterID, reason string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(gin.H{"reason": reason})
	if err != nil {
		t.Fatalf("marshal cancel payload error: %v", err)
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/letters/view/"+letterID+"/cancel", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: letterID}}
	c.Set(middleware.CtxUserID, userID)
	h.CancelLetter(c)
	return rec
}

type cancelLetterEnv struct {
	creatorUserID      string
	creatorPositionID  string
	approver1UserID    string
	approver1Position  string
	approver2UserID    string
	approver2Position  string
	letterID           string
	step1ID            string
	step2ID            string
	approver1ActionsID string
}

// newCancelLetterEnv membuat surat in_approval cycle 1: step 1 approved oleh
// approver1, step 2 waiting pada posisi approver2. tag menjaga keunikan.
func newCancelLetterEnv(t *testing.T, fixture *userPositionFixture, tag string) cancelLetterEnv {
	t.Helper()
	ctx := context.Background()
	env := cancelLetterEnv{}

	orgUnitID := fixture.insertOrgUnit(t, "CXL"+tag, "Cancel Unit "+tag)
	env.creatorUserID = fixture.insertUser(t, "CXC"+tag, "Cancel Creator "+tag)
	env.creatorPositionID = fixture.insertPosition(t, orgUnitID, "Cancel Creator Position "+tag)
	fixture.insertUserPosition(t, env.creatorUserID, env.creatorPositionID, "definitive")

	env.approver1UserID = fixture.insertUser(t, "CXA"+tag, "Cancel Approver One "+tag)
	env.approver1Position = fixture.insertPosition(t, orgUnitID, "Cancel Approver One Position "+tag)
	fixture.insertUserPosition(t, env.approver1UserID, env.approver1Position, "definitive")

	env.approver2UserID = fixture.insertUser(t, "CXB"+tag, "Cancel Approver Two "+tag)
	env.approver2Position = fixture.insertPosition(t, orgUnitID, "Cancel Approver Two Position "+tag)
	fixture.insertUserPosition(t, env.approver2UserID, env.approver2Position, "definitive")

	env.letterID = fixture.insertDraftLetter(t, env.creatorUserID, env.creatorPositionID)
	if _, err := fixture.db.Exec(ctx, `
		UPDATE letters SET status = 'in_approval', current_step_order = 2 WHERE id = $1`, env.letterID); err != nil {
		t.Fatalf("set letter in_approval error: %v", err)
	}
	if err := fixture.db.QueryRow(ctx, `
		INSERT INTO approval_steps (letter_id, approval_cycle, step_order, approver_position_id, flow_group, status, decided_at)
		VALUES ($1, 1, 1, $2, 1, 'approved', now())
		RETURNING id::text`, env.letterID, env.approver1Position).Scan(&env.step1ID); err != nil {
		t.Fatalf("insert approved step error: %v", err)
	}
	if err := fixture.db.QueryRow(ctx, `
		INSERT INTO approval_actions (approval_step_id, action, acted_by_user_id, note)
		VALUES ($1, 'approve', $2, NULL)
		RETURNING id::text`, env.step1ID, env.approver1UserID).Scan(&env.approver1ActionsID); err != nil {
		t.Fatalf("insert approve action error: %v", err)
	}
	if err := fixture.db.QueryRow(ctx, `
		INSERT INTO approval_steps (letter_id, approval_cycle, step_order, approver_position_id, flow_group, status, sla_deadline)
		VALUES ($1, 1, 2, $2, 2, 'waiting', now() + interval '24 hours')
		RETURNING id::text`, env.letterID, env.approver2Position).Scan(&env.step2ID); err != nil {
		t.Fatalf("insert waiting step error: %v", err)
	}
	return env
}

func TestCancelLetterInApproval_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	cleanupDelegationArtifacts(t, fixture)
	env := newCancelLetterEnv(t, fixture, "A")
	ctx := context.Background()

	// Delegate aktif pada posisi step waiting ikut menerima notifikasi.
	delegateUserID := fixture.insertUser(t, "CXD", "Cancel Delegate")
	delegatePosition := fixture.insertPosition(t, fixture.insertOrgUnit(t, "CXDU", "Cancel Delegate Unit"), "Cancel Delegate Position")
	fixture.insertUserPosition(t, delegateUserID, delegatePosition, "definitive")
	insertDelegationRow(t, fixture, env.approver2Position, delegateUserID, env.approver2UserID,
		time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))

	rec := cancelLetterReq(t, h, env.creatorUserID, env.letterID, "  Data anggaran sudah tidak relevan.  ")
	if rec.Code != http.StatusOK {
		t.Fatalf("CancelLetter status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Letter struct {
			ID              string     `json:"id"`
			Status          string     `json:"status"`
			CancelledAt     *time.Time `json:"cancelled_at"`
			CancelledByName string     `json:"cancelled_by_name"`
			CancelReason    string     `json:"cancel_reason"`
		} `json:"letter"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(cancel response) error: %v", err)
	}
	if got.Letter.Status != "cancelled" || got.Letter.CancelledAt == nil || got.Letter.CancelledByName == "" {
		t.Errorf("cancel response = %+v, want cancelled with trace", got.Letter)
	}
	if got.Letter.CancelReason != "Data anggaran sudah tidak relevan." {
		t.Errorf("cancel_reason = %q, want trimmed reason", got.Letter.CancelReason)
	}

	// Surat: status, jejak, tanpa nomor, current_step_order NULL.
	var (
		status       string
		letterNumber *string
		stepOrder    *int
		cancelledBy  *string
		cancelReason *string
	)
	if err := fixture.db.QueryRow(ctx, `
		SELECT status, letter_number, current_step_order, cancelled_by_user_id::text, cancel_reason
		FROM letters WHERE id = $1`, env.letterID).Scan(&status, &letterNumber, &stepOrder, &cancelledBy, &cancelReason); err != nil {
		t.Fatalf("query cancelled letter error: %v", err)
	}
	if status != "cancelled" || letterNumber != nil || stepOrder != nil {
		t.Errorf("letter after cancel = (%q, %v, %v), want cancelled without number/step", status, letterNumber, stepOrder)
	}
	if cancelledBy == nil || *cancelledBy != env.creatorUserID || cancelReason == nil {
		t.Errorf("cancel trace = (%v, %v), want creator + reason", cancelledBy, cancelReason)
	}

	// Step waiting -> skipped; step approved tetap; action approve utuh.
	var step1Status, step2Status string
	if err := fixture.db.QueryRow(ctx, `SELECT status FROM approval_steps WHERE id = $1`, env.step1ID).Scan(&step1Status); err != nil {
		t.Fatalf("query step1 error: %v", err)
	}
	if err := fixture.db.QueryRow(ctx, `SELECT status FROM approval_steps WHERE id = $1`, env.step2ID).Scan(&step2Status); err != nil {
		t.Fatalf("query step2 error: %v", err)
	}
	if step1Status != "approved" || step2Status != "skipped" {
		t.Errorf("step statuses = (%q, %q), want (approved, skipped)", step1Status, step2Status)
	}
	var actionCount int
	if err := fixture.db.QueryRow(ctx, `
		SELECT count(*) FROM approval_actions WHERE approval_step_id = $1`, env.step1ID).Scan(&actionCount); err != nil {
		t.Fatalf("count approve actions error: %v", err)
	}
	if actionCount != 1 {
		t.Errorf("approve actions after cancel = %d, want 1 (utuh)", actionCount)
	}

	// Notifikasi: approver yang sudah approve, pemegang step di-skip, delegate.
	for _, target := range []string{env.approver1UserID, env.approver2UserID, delegateUserID} {
		var notified bool
		if err := fixture.db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM notifications
				WHERE user_id = $1 AND letter_id = $2 AND event_type = 'letter_cancelled'
			)`, target, env.letterID).Scan(&notified); err != nil {
			t.Fatalf("query letter_cancelled notification error: %v", err)
		}
		if !notified {
			t.Errorf("letter_cancelled notification for %q not found", target)
		}
	}
	var outboxCount int
	if err := fixture.db.QueryRow(ctx, `
		SELECT count(*) FROM notification_outbox
		WHERE letter_id = $1 AND event_type = 'letter_cancelled'`, env.letterID).Scan(&outboxCount); err != nil {
		t.Fatalf("count cancel outbox error: %v", err)
	}
	if outboxCount != 3 {
		t.Errorf("letter_cancelled outbox rows = %d, want 3", outboxCount)
	}

	// Audit: action letter_cancelled + detail reason & previous_status.
	var detail map[string]any
	if err := fixture.db.QueryRow(ctx, `
		SELECT detail FROM audit_logs
		WHERE entity_type = 'letter' AND entity_id = $1 AND action = 'letter_cancelled'
		  AND actor_user_id = $2`, env.letterID, env.creatorUserID).Scan(&detail); err != nil {
		t.Fatalf("query cancel audit error: %v", err)
	}
	if detail["previous_status"] != "in_approval" || detail["reason"] != "Data anggaran sudah tidak relevan." {
		t.Errorf("cancel audit detail = %+v, want previous_status in_approval + reason", detail)
	}

	// Detail surat mengekspos jejak; can_cancel false setelah batal.
	detailRec := httptest.NewRecorder()
	dc, _ := gin.CreateTestContext(detailRec)
	dc.Request = httptest.NewRequest(http.MethodGet, "/letters/view/"+env.letterID, nil)
	dc.Params = gin.Params{{Key: "id", Value: env.letterID}}
	dc.Set(middleware.CtxUserID, env.creatorUserID)
	h.GetLetterDetail(dc)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("GetLetterDetail status = %d, body = %s", detailRec.Code, detailRec.Body.String())
	}
	var detailResp struct {
		Letter LetterDetail `json:"letter"`
	}
	if err := json.Unmarshal(detailRec.Body.Bytes(), &detailResp); err != nil {
		t.Fatalf("json.Unmarshal(letter detail) error: %v", err)
	}
	if detailResp.Letter.CancelledAt == nil || detailResp.Letter.CancelledByName == nil || detailResp.Letter.CancelReason == nil {
		t.Errorf("letter detail cancel trace missing: %+v", detailResp.Letter)
	}
	if detailResp.Letter.CanCancel {
		t.Error("can_cancel = true after cancel, want false")
	}

	// Cancel ulang -> 409 "surat sudah dibatalkan".
	rec = cancelLetterReq(t, h, env.creatorUserID, env.letterID, "coba lagi")
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "surat sudah dibatalkan") {
		t.Errorf("second cancel = (%d, %s), want 409 surat sudah dibatalkan", rec.Code, rec.Body.String())
	}
}

func TestCancelLetterPerStatusAndValidation_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	cleanupDelegationArtifacts(t, fixture)
	ctx := context.Background()

	orgUnitID := fixture.insertOrgUnit(t, "CXS", "Cancel Status Unit")
	creatorUserID := fixture.insertUser(t, "CXSC", "Cancel Status Creator")
	creatorPositionID := fixture.insertPosition(t, orgUnitID, "Cancel Status Creator Position")
	fixture.insertUserPosition(t, creatorUserID, creatorPositionID, "definitive")

	newLetterWithStatus := func(status string) string {
		letterID := fixture.insertDraftLetter(t, creatorUserID, creatorPositionID)
		if status != "draft" {
			if _, err := fixture.db.Exec(ctx, `UPDATE letters SET status = $2 WHERE id = $1`, letterID, status); err != nil {
				t.Fatalf("set letter status %q error: %v", status, err)
			}
		}
		return letterID
	}

	// Happy path per status cancellable.
	for _, status := range []string{"draft", "revision", "in_approval"} {
		letterID := newLetterWithStatus(status)
		rec := cancelLetterReq(t, h, creatorUserID, letterID, "batal dari status "+status)
		if rec.Code != http.StatusOK {
			t.Errorf("cancel %q status = %d, want 200; body = %s", status, rec.Code, rec.Body.String())
		}
	}

	// 409 untuk status final.
	for _, tt := range []struct {
		status  string
		message string
	}{
		{"approved", "surat sudah disetujui final"},
		{"published", "surat sudah disetujui final"},
		{"cancelled", "surat sudah dibatalkan"},
	} {
		letterID := newLetterWithStatus(tt.status)
		rec := cancelLetterReq(t, h, creatorUserID, letterID, "coba batalkan")
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), tt.message) {
			t.Errorf("cancel %q = (%d, %s), want 409 %q", tt.status, rec.Code, rec.Body.String(), tt.message)
		}
	}

	// 400: alasan kosong / whitespace / kepanjangan (hitung rune).
	letterID := newLetterWithStatus("draft")
	for _, reason := range []string{"", "   \n\t ", strings.Repeat("ä", 1001)} {
		rec := cancelLetterReq(t, h, creatorUserID, letterID, reason)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("cancel invalid reason %q... status = %d, want 400", reason[:min(len(reason), 8)], rec.Code)
		}
	}
	// Tepat 1000 rune sah.
	if rec := cancelLetterReq(t, h, creatorUserID, letterID, strings.Repeat("ä", 1000)); rec.Code != http.StatusOK {
		t.Errorf("cancel 1000-rune reason status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	// 404: bukan pembuat (user lain se-company) dan surat tidak ada.
	otherUserID := fixture.insertUser(t, "CXSO", "Cancel Status Other")
	otherPositionID := fixture.insertPosition(t, orgUnitID, "Cancel Status Other Position")
	fixture.insertUserPosition(t, otherUserID, otherPositionID, "definitive")
	victimLetterID := newLetterWithStatus("in_approval")
	if rec := cancelLetterReq(t, h, otherUserID, victimLetterID, "bukan surat saya"); rec.Code != http.StatusNotFound {
		t.Errorf("cancel by non-owner status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	if rec := cancelLetterReq(t, h, creatorUserID, "00000000-0000-0000-0000-000000000000", "tidak ada"); rec.Code != http.StatusNotFound {
		t.Errorf("cancel missing letter status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}

	// can_cancel true untuk pembuat pada status cancellable.
	detailRec := httptest.NewRecorder()
	dc, _ := gin.CreateTestContext(detailRec)
	dc.Request = httptest.NewRequest(http.MethodGet, "/letters/view/"+victimLetterID, nil)
	dc.Params = gin.Params{{Key: "id", Value: victimLetterID}}
	dc.Set(middleware.CtxUserID, creatorUserID)
	h.GetLetterDetail(dc)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("GetLetterDetail status = %d, body = %s", detailRec.Code, detailRec.Body.String())
	}
	var detailResp struct {
		Letter LetterDetail `json:"letter"`
	}
	if err := json.Unmarshal(detailRec.Body.Bytes(), &detailResp); err != nil {
		t.Fatalf("json.Unmarshal(letter detail) error: %v", err)
	}
	if !detailResp.Letter.CanCancel {
		t.Error("can_cancel = false for creator on in_approval letter, want true")
	}
}

// TestCancelLetterRaceWithApproveFinal_Integration memastikan tepat satu yang
// menang antara pembatalan pembuat dan approve final. Sisi approve dieksekusi
// sebagai transaksi SQL dengan pola lock yang identik dengan ActApprovalStep
// (FOR UPDATE OF s, l) karena jalur approve penuh membutuhkan object storage
// tanda tangan yang tidak tersedia pada test DB-only.
func TestCancelLetterRaceWithApproveFinal_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	cleanupDelegationArtifacts(t, fixture)
	ctx := context.Background()

	for i, tag := range []string{"R1", "R2", "R3"} {
		env := newCancelLetterEnv(t, fixture, tag)
		letterNumber := "RACE-" + fixture.suffix + "-" + tag

		approveFinal := func() (won bool, err error) {
			tx, err := fixture.db.Begin(ctx)
			if err != nil {
				return false, err
			}
			defer tx.Rollback(ctx)
			// Meniru urutan lock ActApprovalStep: letters dulu, baru step —
			// urutan yang sama dengan CancelLetter sehingga bebas deadlock.
			var letterID string
			err = tx.QueryRow(ctx, `
				SELECT l.id::text
				FROM letters l
				WHERE l.id = (SELECT letter_id FROM approval_steps WHERE id = $1)
				  AND l.status = 'in_approval'
				FOR UPDATE`, env.step2ID).Scan(&letterID)
			if err == pgx.ErrNoRows {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			var stepOrder int
			err = tx.QueryRow(ctx, `
				SELECT s.step_order
				FROM approval_steps s
				WHERE s.id = $1 AND s.status = 'waiting'
				FOR UPDATE OF s`, env.step2ID).Scan(&stepOrder)
			if err == pgx.ErrNoRows {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			if _, err := tx.Exec(ctx, `
				UPDATE approval_steps SET status = 'approved', decided_at = now() WHERE id = $1`, env.step2ID); err != nil {
				return false, err
			}
			if _, err := tx.Exec(ctx, `
				UPDATE letters
				SET status = 'approved', letter_number = $2, current_step_order = NULL, updated_at = now()
				WHERE id = $1`, letterID, letterNumber); err != nil {
				return false, err
			}
			return true, tx.Commit(ctx)
		}

		var wg sync.WaitGroup
		start := make(chan struct{})
		var cancelCode int
		var approveWon bool
		var approveErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			cancelCode = cancelLetterReq(t, h, env.creatorUserID, env.letterID, "race dengan approve final").Code
		}()
		go func() {
			defer wg.Done()
			<-start
			approveWon, approveErr = approveFinal()
		}()
		close(start)
		wg.Wait()
		if approveErr != nil {
			t.Fatalf("race %d approve error: %v", i, approveErr)
		}

		var status string
		var gotNumber *string
		if err := fixture.db.QueryRow(ctx, `
			SELECT status, letter_number FROM letters WHERE id = $1`, env.letterID).Scan(&status, &gotNumber); err != nil {
			t.Fatalf("race %d query letter error: %v", i, err)
		}

		cancelWon := cancelCode == http.StatusOK
		if cancelWon == approveWon {
			t.Fatalf("race %d: cancel code = %d, approve won = %t — want exactly one winner", i, cancelCode, approveWon)
		}
		if cancelWon {
			if status != "cancelled" || gotNumber != nil {
				t.Errorf("race %d cancel won but letter = (%q, %v), want cancelled without number", i, status, gotNumber)
			}
		} else {
			if cancelCode != http.StatusConflict {
				t.Errorf("race %d approve won but cancel code = %d, want 409", i, cancelCode)
			}
			if status != "approved" || gotNumber == nil {
				t.Errorf("race %d approve won but letter = (%q, %v), want approved with number", i, status, gotNumber)
			}
		}
	}
}
