package handler

// Test matriks enforcement klasifikasi Biasa/Terbatas/Rahasia (E10-1).
// Mengunci perilaku server-side yang sudah ada (CC pada rahasia = 404,
// disposisi tidak memberi akses rahasia, lintas company = 404) DAN pencatatan
// audit `access_denied` untuk akses URL langsung yang ditolak pada detail,
// unduh PDF final, dan unduh lampiran. Penolakan untuk id surat yang tidak ada
// TIDAK dicatat (anti spam audit).

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

type classificationEnv struct {
	letterID           string
	creatorUserID      string
	approverUserID     string
	approverPositionID string
	toUserID           string
	toPositionID       string
	ccUserID           string
	unrelatedUserID    string
}

// newClassificationLetterEnv membuat surat published dengan klasifikasi
// tertentu: creator, satu approver (step approved), penerima To dan CC
// (position + resolved_user), plus satu user se-company tanpa hubungan.
func newClassificationLetterEnv(t *testing.T, fixture *userPositionFixture, classification string, tag string) classificationEnv {
	t.Helper()
	ctx := context.Background()

	unitID := fixture.insertOrgUnit(t, "CLU"+tag, "Classification Unit "+tag)
	env := classificationEnv{}

	env.creatorUserID = fixture.insertUser(t, "CLC"+tag, "Classification Creator "+tag)
	creatorPositionID := fixture.insertPosition(t, unitID, "Classification Creator Position "+tag)
	fixture.insertUserPosition(t, env.creatorUserID, creatorPositionID, "definitive")

	env.approverUserID = fixture.insertUser(t, "CLA"+tag, "Classification Approver "+tag)
	env.approverPositionID = fixture.insertPosition(t, unitID, "Classification Approver Position "+tag)
	fixture.insertUserPosition(t, env.approverUserID, env.approverPositionID, "definitive")

	env.toUserID = fixture.insertUser(t, "CLT"+tag, "Classification To Recipient "+tag)
	env.toPositionID = fixture.insertPosition(t, unitID, "Classification To Position "+tag)
	fixture.insertUserPosition(t, env.toUserID, env.toPositionID, "definitive")

	env.ccUserID = fixture.insertUser(t, "CLX"+tag, "Classification CC Recipient "+tag)
	ccPositionID := fixture.insertPosition(t, unitID, "Classification CC Position "+tag)
	fixture.insertUserPosition(t, env.ccUserID, ccPositionID, "definitive")

	env.unrelatedUserID = fixture.insertUser(t, "CLN"+tag, "Classification Unrelated "+tag)
	unrelatedPositionID := fixture.insertPosition(t, unitID, "Classification Unrelated Position "+tag)
	fixture.insertUserPosition(t, env.unrelatedUserID, unrelatedPositionID, "definitive")

	env.letterID = fixture.insertDraftLetter(t, env.creatorUserID, creatorPositionID)
	if _, err := fixture.db.Exec(ctx, `
		UPDATE letters
		SET status = 'published', published_at = now(), classification = $2
		WHERE id = $1`, env.letterID, classification); err != nil {
		t.Fatalf("publish letter (%s) error: %v", classification, err)
	}
	if _, err := fixture.db.Exec(ctx, `
		INSERT INTO approval_steps (letter_id, approval_cycle, step_order, approver_position_id, flow_group, status)
		VALUES ($1, 1, 1, $2, 1, 'approved')`, env.letterID, env.approverPositionID); err != nil {
		t.Fatalf("insert approved step error: %v", err)
	}
	for _, recipient := range []struct {
		recipientType string
		positionID    string
		userID        string
	}{
		{"to", env.toPositionID, env.toUserID},
		{"cc", ccPositionID, env.ccUserID},
	} {
		if _, err := fixture.db.Exec(ctx, `
			INSERT INTO letter_recipients (letter_id, recipient_type, position_id, resolved_user_id, delivered_at)
			VALUES ($1, $2, $3, $4, now())`,
			env.letterID, recipient.recipientType, recipient.positionID, recipient.userID); err != nil {
			t.Fatalf("insert %s recipient error: %v", recipient.recipientType, err)
		}
	}

	// read_receipts (tanda baca saat detail 200) tidak dibersihkan fixture;
	// hapus sebelum fixture menghapus letters/users (t.Cleanup LIFO).
	t.Cleanup(func() {
		_, _ = fixture.db.Exec(context.Background(),
			`DELETE FROM read_receipts WHERE letter_id = $1`, env.letterID)
	})
	return env
}

// newOtherCompanyPlainUser membuat user biasa (pemegang posisi) pada company
// lain, tanpa hubungan apa pun dengan surat fixture.
func newOtherCompanyPlainUser(t *testing.T, fixture *userPositionFixture, tag string) string {
	t.Helper()
	ctx := context.Background()

	var companyID string
	if err := fixture.db.QueryRow(ctx, `
		INSERT INTO companies (code, name)
		VALUES ($1, $2)
		RETURNING id::text`, "CO"+tag+fixture.suffix, "Classification Other Co "+tag+fixture.suffix).Scan(&companyID); err != nil {
		t.Fatalf("insert other company error: %v", err)
	}
	var orgUnitID string
	if err := fixture.db.QueryRow(ctx, `
		INSERT INTO org_units (company_id, code, name, unit_level)
		VALUES ($1, $2, $3, 'department')
		RETURNING id::text`, companyID, "CLOU"+tag+fixture.suffix, "Other Co Unit "+tag).Scan(&orgUnitID); err != nil {
		t.Fatalf("insert other company org unit error: %v", err)
	}
	var positionID string
	if err := fixture.db.QueryRow(ctx, `
		INSERT INTO positions (org_unit_id, title, position_type)
		VALUES ($1, $2, 'staff')
		RETURNING id::text`, orgUnitID, "Other Co Staff "+tag+" "+fixture.suffix).Scan(&positionID); err != nil {
		t.Fatalf("insert other company position error: %v", err)
	}
	userID := fixture.insertUser(t, "CLO"+tag, "Other Company User "+tag)
	fixture.insertUserPosition(t, userID, positionID, "definitive")

	t.Cleanup(func() {
		cctx := context.Background()
		_, _ = fixture.db.Exec(cctx, `DELETE FROM user_positions WHERE position_id = $1`, positionID)
		_, _ = fixture.db.Exec(cctx, `DELETE FROM positions WHERE id = $1`, positionID)
		_, _ = fixture.db.Exec(cctx, `DELETE FROM org_units WHERE id = $1`, orgUnitID)
		_, _ = fixture.db.Exec(cctx, `DELETE FROM companies WHERE id = $1`, companyID)
	})
	return userID
}

func classificationDetailReq(t *testing.T, h *Handler, userID string, letterID string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/letters/view/"+letterID, nil)
	c.Params = gin.Params{{Key: "id", Value: letterID}}
	c.Set(middleware.CtxUserID, userID)
	h.GetLetterDetail(c)
	return rec
}

func classificationDownloadPDFReq(t *testing.T, h *Handler, userID string, letterID string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/letters/view/"+letterID+"/final-pdf", nil)
	c.Params = gin.Params{{Key: "id", Value: letterID}}
	c.Set(middleware.CtxUserID, userID)
	h.DownloadFinalLetterPDF(c)
	return rec
}

func classificationDownloadAttachmentReq(t *testing.T, h *Handler, userID string, letterID string, attachmentID string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/letters/view/"+letterID+"/attachments/"+attachmentID+"/download", nil)
	c.Params = gin.Params{{Key: "id", Value: letterID}, {Key: "attachment_id", Value: attachmentID}}
	c.Set(middleware.CtxUserID, userID)
	h.DownloadLetterAttachment(c)
	return rec
}

// countAccessDenied menghitung baris audit access_denied untuk kombinasi
// surat+aktor+via, sekaligus memastikan IP ikut tercatat.
func countAccessDenied(t *testing.T, fixture *userPositionFixture, letterID string, actorUserID string, via string) int {
	t.Helper()
	var count int
	if err := fixture.db.QueryRow(context.Background(), `
		SELECT count(*)
		FROM audit_logs
		WHERE entity_type = 'letter'
		  AND entity_id = $1
		  AND action = 'access_denied'
		  AND actor_user_id = $2
		  AND detail->>'via' = $3
		  AND ip_address IS NOT NULL
		  AND ip_address <> ''`, letterID, actorUserID, via).Scan(&count); err != nil {
		t.Fatalf("count access_denied(%s) error: %v", via, err)
	}
	return count
}

func TestClassificationRahasiaMatrix_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	env := newClassificationLetterEnv(t, fixture, "rahasia", "RH")
	ctx := context.Background()

	// Penerima To dan approver: detail 200.
	if rec := classificationDetailReq(t, h, env.toUserID, env.letterID); rec.Code != http.StatusOK {
		t.Errorf("detail rahasia (penerima to) status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if rec := classificationDetailReq(t, h, env.approverUserID, env.letterID); rec.Code != http.StatusOK {
		t.Errorf("detail rahasia (approver) status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	// Penerima CC pada rahasia: detail 404 + tercatat access_denied.
	if rec := classificationDetailReq(t, h, env.ccUserID, env.letterID); rec.Code != http.StatusNotFound {
		t.Errorf("detail rahasia (penerima cc) status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	if got := countAccessDenied(t, fixture, env.letterID, env.ccUserID, "detail"); got != 1 {
		t.Errorf("audit access_denied via=detail (cc) = %d baris, want 1", got)
	}

	// Unduhan CC pada rahasia: 404 + tercatat per jalur.
	if rec := classificationDownloadPDFReq(t, h, env.ccUserID, env.letterID); rec.Code != http.StatusNotFound {
		t.Errorf("download pdf rahasia (cc) status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	if got := countAccessDenied(t, fixture, env.letterID, env.ccUserID, "download_pdf"); got != 1 {
		t.Errorf("audit access_denied via=download_pdf (cc) = %d baris, want 1", got)
	}
	if rec := classificationDownloadAttachmentReq(t, h, env.ccUserID, env.letterID,
		"00000000-0000-0000-0000-000000000001"); rec.Code != http.StatusNotFound {
		t.Errorf("download lampiran rahasia (cc) status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	if got := countAccessDenied(t, fixture, env.letterID, env.ccUserID, "download_attachment"); got != 1 {
		t.Errorf("audit access_denied via=download_attachment (cc) = %d baris, want 1", got)
	}

	// Disposisi TIDAK memberi akses view pada rahasia. Baris disposisi dibuat
	// langsung di DB (adversarial) dari posisi penerima To ke posisi lain.
	dispositionUserID := fixture.insertUser(t, "CLD"+"RH", "Classification Disposition Target RH")
	dispositionUnitID := fixture.insertOrgUnit(t, "CLDU"+"RH", "Classification Disposition Unit RH")
	dispositionPositionID := fixture.insertPosition(t, dispositionUnitID, "Classification Disposition Position RH")
	fixture.insertUserPosition(t, dispositionUserID, dispositionPositionID, "definitive")
	var dispositionID string
	if err := fixture.db.QueryRow(ctx, `
		INSERT INTO dispositions (letter_id, from_position_id, instruction, created_by)
		VALUES ($1, $2, 'tindak lanjuti (probe rahasia)', $3)
		RETURNING id::text`, env.letterID, env.toPositionID, env.toUserID).Scan(&dispositionID); err != nil {
		t.Fatalf("insert disposition error: %v", err)
	}
	if _, err := fixture.db.Exec(ctx, `
		INSERT INTO disposition_recipients (disposition_id, position_id)
		VALUES ($1, $2)`, dispositionID, dispositionPositionID); err != nil {
		t.Fatalf("insert disposition recipient error: %v", err)
	}
	if rec := classificationDetailReq(t, h, dispositionUserID, env.letterID); rec.Code != http.StatusNotFound {
		t.Errorf("detail rahasia (penerima disposisi) status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	if got := countAccessDenied(t, fixture, env.letterID, dispositionUserID, "detail"); got != 1 {
		t.Errorf("audit access_denied via=detail (disposisi) = %d baris, want 1", got)
	}

	// Aturan unduh rahasia: To/approver boleh, CC tidak.
	for _, tc := range []struct {
		name    string
		userID  string
		allowed bool
	}{
		{"to", env.toUserID, true},
		{"approver", env.approverUserID, true},
		{"cc", env.ccUserID, false},
		{"disposisi", dispositionUserID, false},
	} {
		allowed, err := h.userCanDownloadLetter(ctx, tc.userID, env.letterID)
		if err != nil {
			t.Fatalf("userCanDownloadLetter(%s) error: %v", tc.name, err)
		}
		if allowed != tc.allowed {
			t.Errorf("userCanDownloadLetter(rahasia, %s) = %t, want %t", tc.name, allowed, tc.allowed)
		}
	}
}

func TestClassificationTerbatasDanBiasaMatrix_Integration(t *testing.T) {
	tags := map[string]string{"terbatas": "TB", "biasa": "BS"}
	for classification, tag := range tags {
		t.Run(classification, func(t *testing.T) {
			h, fixture := newUserPositionFixture(t)
			env := newClassificationLetterEnv(t, fixture, classification, tag)
			otherCompanyUserID := newOtherCompanyPlainUser(t, fixture, tag)

			// Penerima To dan CC: 200.
			if rec := classificationDetailReq(t, h, env.toUserID, env.letterID); rec.Code != http.StatusOK {
				t.Errorf("detail %s (to) status = %d, want 200; body = %s", classification, rec.Code, rec.Body.String())
			}
			if rec := classificationDetailReq(t, h, env.ccUserID, env.letterID); rec.Code != http.StatusOK {
				t.Errorf("detail %s (cc) status = %d, want 200; body = %s", classification, rec.Code, rec.Body.String())
			}

			// User se-company tanpa hubungan: 404 + tercatat.
			if rec := classificationDetailReq(t, h, env.unrelatedUserID, env.letterID); rec.Code != http.StatusNotFound {
				t.Errorf("detail %s (unrelated se-company) status = %d, want 404; body = %s", classification, rec.Code, rec.Body.String())
			}
			if got := countAccessDenied(t, fixture, env.letterID, env.unrelatedUserID, "detail"); got != 1 {
				t.Errorf("audit access_denied via=detail (unrelated, %s) = %d baris, want 1", classification, got)
			}
			if rec := classificationDownloadPDFReq(t, h, env.unrelatedUserID, env.letterID); rec.Code != http.StatusNotFound {
				t.Errorf("download pdf %s (unrelated) status = %d, want 404; body = %s", classification, rec.Code, rec.Body.String())
			}
			if got := countAccessDenied(t, fixture, env.letterID, env.unrelatedUserID, "download_pdf"); got != 1 {
				t.Errorf("audit access_denied via=download_pdf (unrelated, %s) = %d baris, want 1", classification, got)
			}

			// User company lain: 404 + tercatat.
			if rec := classificationDetailReq(t, h, otherCompanyUserID, env.letterID); rec.Code != http.StatusNotFound {
				t.Errorf("detail %s (company lain) status = %d, want 404; body = %s", classification, rec.Code, rec.Body.String())
			}
			if got := countAccessDenied(t, fixture, env.letterID, otherCompanyUserID, "detail"); got != 1 {
				t.Errorf("audit access_denied via=detail (company lain, %s) = %d baris, want 1", classification, got)
			}
		})
	}
}

// TestClassificationMissingLetterNotAudited_Integration: 404 untuk id surat
// yang tidak ada TIDAK menghasilkan baris audit (anti spam untuk id acak).
func TestClassificationMissingLetterNotAudited_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	env := newClassificationLetterEnv(t, fixture, "biasa", "MS")

	const missingLetterID = "00000000-0000-0000-0000-0000000000aa"
	if rec := classificationDetailReq(t, h, env.unrelatedUserID, missingLetterID); rec.Code != http.StatusNotFound {
		t.Fatalf("detail surat tidak ada status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	if rec := classificationDownloadPDFReq(t, h, env.unrelatedUserID, missingLetterID); rec.Code != http.StatusNotFound {
		t.Fatalf("download pdf surat tidak ada status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}

	var count int
	if err := fixture.db.QueryRow(context.Background(), `
		SELECT count(*)
		FROM audit_logs
		WHERE entity_type = 'letter'
		  AND entity_id = $1
		  AND action = 'access_denied'`, missingLetterID).Scan(&count); err != nil {
		t.Fatalf("count access_denied surat tidak ada error: %v", err)
	}
	if count != 0 {
		t.Errorf("audit access_denied untuk surat tidak ada = %d baris, want 0", count)
	}
}
