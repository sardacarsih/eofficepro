package handler

// Probe QA wave-3 untuk E03-5 (delegasi wewenang) dan E03-7 (pembatalan surat).
// Menutup skenario adversarial yang belum dicakup delegation_test.go dan
// letter_cancel_test.go: isolasi lintas company untuk admin company lain,
// delegasi expired pada jalur aksi, aksi delegate terhadap surat yang sudah
// dibatalkan, retry idempotency konkuren dengan client_action_id sama, dan
// delegasi revoked yang tidak memblokir rentang baru.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

// newOtherCompanyAdmin membuat company kedua beserta admin company-nya
// (user_company_roles role 'admin') untuk skenario isolasi lintas company.
func newOtherCompanyAdmin(t *testing.T, fixture *userPositionFixture) (otherCompanyID, adminUserID string) {
	t.Helper()
	ctx := context.Background()

	if err := fixture.db.QueryRow(ctx, `
		INSERT INTO companies (code, name)
		VALUES ($1, $2)
		RETURNING id::text`, "QAX"+fixture.suffix, "QA Probe Other "+fixture.suffix).Scan(&otherCompanyID); err != nil {
		t.Fatalf("insert other company error: %v", err)
	}
	adminUserID = fixture.insertUser(t, "QAADM", "QA Other Company Admin")

	var roleID string
	if err := fixture.db.QueryRow(ctx, `SELECT id::text FROM roles WHERE code = 'admin'`).Scan(&roleID); err != nil {
		t.Fatalf("query admin role error: %v", err)
	}
	if _, err := fixture.db.Exec(ctx, `
		INSERT INTO user_company_roles (user_id, company_id, role_id)
		VALUES ($1, $2, $3)`, adminUserID, otherCompanyID, roleID); err != nil {
		t.Fatalf("insert user company role error: %v", err)
	}
	t.Cleanup(func() {
		_, _ = fixture.db.Exec(ctx, `DELETE FROM user_company_roles WHERE user_id = $1`, adminUserID)
		_, _ = fixture.db.Exec(ctx, `DELETE FROM companies WHERE id = $1`, otherCompanyID)
	})
	return otherCompanyID, adminUserID
}

// TestQADelegationCrossCompanyAdminIsolation_Integration memastikan admin
// company LAIN tidak dapat melihat, mencabut, membuat, atau memuat kandidat
// delegasi milik company fixture (tenant isolation E03-5).
func TestQADelegationCrossCompanyAdminIsolation_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	cleanupDelegationArtifacts(t, fixture)
	env := newDelegationEnv(t, fixture)

	delegationID := insertDelegationRow(t, fixture, env.delegatorPositionID, env.delegateUserID, env.delegatorUserID,
		time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	_, adminUserID := newOtherCompanyAdmin(t, fixture)

	// List gabungan (tanpa scope) sebagai admin company lain: delegasi company
	// fixture tidak boleh bocor.
	rec := listDelegations(t, h, adminUserID, "?include_past=true")
	if rec.Code != http.StatusOK {
		t.Fatalf("ListDelegations(other-company admin) status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Data []delegationItem `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("json.Unmarshal(list) error: %v", err)
	}
	for _, item := range listed.Data {
		if item.ID == delegationID {
			t.Fatalf("delegasi company lain bocor ke admin company lain: %+v", item)
		}
	}

	// Revoke lintas company -> 404 (menyembunyikan keberadaan).
	if rec := revokeDelegation(t, h, adminUserID, delegationID); rec.Code != http.StatusNotFound {
		t.Errorf("RevokeDelegation(other-company admin) status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}

	// Create untuk posisi company fixture -> 403 (sesuai kontrak POST).
	rec = postDelegation(t, h, adminUserID, gin.H{
		"delegator_position_id": env.delegatorPositionID,
		"delegate_user_id":      env.delegateUserID,
		"reason":                "probe lintas company",
		"valid_from":            time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		"valid_to":              time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
	})
	if rec.Code != http.StatusForbidden {
		t.Errorf("CreateDelegation(other-company admin) status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}

	// Delegate-options untuk posisi company fixture -> 403.
	optRec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(optRec)
	c.Request = httptest.NewRequest(http.MethodGet, "/delegations/delegate-options?position_id="+env.delegatorPositionID, nil)
	c.Set(middleware.CtxUserID, adminUserID)
	h.ListDelegateOptions(c)
	if optRec.Code != http.StatusForbidden {
		t.Errorf("ListDelegateOptions(other-company admin) status = %d, want 403; body = %s", optRec.Code, optRec.Body.String())
	}
}

// TestQADelegateExpiredCannotActAndInboxEmpty_Integration menutup boundary
// yang belum diuji: delegasi kedaluwarsa tidak memberi hak aksi maupun inbox.
func TestQADelegateExpiredCannotActAndInboxEmpty_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	cleanupDelegationArtifacts(t, fixture)
	env := newDelegationEnv(t, fixture)

	_, _, stepID := newWaitingApprovalLetter(t, fixture, env.delegatorPositionID, "QX")
	insertDelegationRow(t, fixture, env.delegatorPositionID, env.delegateUserID, env.delegatorUserID,
		time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour))

	if rec := actApprovalStep(t, h, env.delegateUserID, stepID, gin.H{"action": "request_revision", "note": "delegasi kedaluwarsa"}); rec.Code != http.StatusNotFound {
		t.Errorf("act with expired delegation status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}

	inboxRec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(inboxRec)
	c.Request = httptest.NewRequest(http.MethodGet, "/approvals/inbox", nil)
	c.Set(middleware.CtxUserID, env.delegateUserID)
	h.ListApprovalInbox(c)
	if inboxRec.Code != http.StatusOK {
		t.Fatalf("ListApprovalInbox status = %d, body = %s", inboxRec.Code, inboxRec.Body.String())
	}
	var inbox struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(inboxRec.Body.Bytes(), &inbox); err != nil {
		t.Fatalf("json.Unmarshal(inbox) error: %v", err)
	}
	if len(inbox.Data) != 0 {
		t.Errorf("inbox delegate expired = %d item, want 0", len(inbox.Data))
	}
}

// TestQACancelledLetterBlocksDelegateActAndDelegateCannotCancel_Integration:
// setelah pembuat membatalkan surat, delegate aktif tidak bisa bertindak (404),
// dan delegate (bukan pembuat) tidak pernah bisa membatalkan surat (404).
func TestQACancelledLetterBlocksDelegateActAndDelegateCannotCancel_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	cleanupDelegationArtifacts(t, fixture)
	env := newDelegationEnv(t, fixture)

	creatorUserID, letterID, stepID := newWaitingApprovalLetter(t, fixture, env.delegatorPositionID, "QC")
	insertDelegationRow(t, fixture, env.delegatorPositionID, env.delegateUserID, env.delegatorUserID,
		time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))

	// Delegate bukan pembuat: cancel -> 404 tanpa membocorkan keberadaan.
	if rec := cancelLetterReq(t, h, env.delegateUserID, letterID, "bukan surat saya"); rec.Code != http.StatusNotFound {
		t.Errorf("cancel by delegate status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	// Approver/pemegang posisi delegator juga bukan pembuat -> 404.
	if rec := cancelLetterReq(t, h, env.delegatorUserID, letterID, "bukan surat saya"); rec.Code != http.StatusNotFound {
		t.Errorf("cancel by approver status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}

	// Pembuat membatalkan; delegate aktif tidak bisa lagi bertindak.
	if rec := cancelLetterReq(t, h, creatorUserID, letterID, "dibatalkan sebelum delegate bertindak"); rec.Code != http.StatusOK {
		t.Fatalf("cancel by creator status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if rec := actApprovalStep(t, h, env.delegateUserID, stepID, gin.H{"action": "approve"}); rec.Code == http.StatusOK {
		t.Errorf("act on cancelled letter succeeded, want failure; body = %s", rec.Body.String())
	}
	if rec := actApprovalStep(t, h, env.delegateUserID, stepID, gin.H{"action": "request_revision", "note": "sudah batal"}); rec.Code != http.StatusNotFound {
		t.Errorf("act on cancelled letter status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
}

// TestQAActConcurrentDuplicateClientActionID_Integration: dua aksi konkuren
// dengan client_action_id yang sama menghasilkan tepat satu approval_action
// (idempotency E03 tidak berubah oleh integrasi delegasi).
func TestQAActConcurrentDuplicateClientActionID_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	cleanupDelegationArtifacts(t, fixture)
	env := newDelegationEnv(t, fixture)
	ctx := context.Background()

	_, _, stepID := newWaitingApprovalLetter(t, fixture, env.delegatorPositionID, "QI")
	insertDelegationRow(t, fixture, env.delegatorPositionID, env.delegateUserID, env.delegatorUserID,
		time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))

	var clientActionID string
	if err := fixture.db.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&clientActionID); err != nil {
		t.Fatalf("generate client_action_id error: %v", err)
	}
	body := gin.H{"action": "request_revision", "note": "retry idempotent", "client_action_id": clientActionID}

	var wg sync.WaitGroup
	start := make(chan struct{})
	codes := make([]int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			codes[i] = actApprovalStep(t, h, env.delegateUserID, stepID, body).Code
		}(i)
	}
	close(start)
	wg.Wait()

	success := 0
	for _, code := range codes {
		switch code {
		case http.StatusOK:
			success++
		case http.StatusConflict, http.StatusNotFound:
			// duplikat ditolak (unique client_action_id) atau step sudah diputus
		default:
			t.Errorf("unexpected concurrent duplicate code = %d (codes %v)", code, codes)
		}
	}
	if success != 1 {
		t.Errorf("concurrent duplicate client_action_id codes = %v, want exactly one 200", codes)
	}

	var actionCount int
	if err := fixture.db.QueryRow(ctx, `
		SELECT count(*) FROM approval_actions WHERE approval_step_id = $1`, stepID).Scan(&actionCount); err != nil {
		t.Fatalf("count approval actions error: %v", err)
	}
	if actionCount != 1 {
		t.Errorf("approval_actions rows = %d, want 1 (idempotent)", actionCount)
	}
}

// TestQARevokedDelegationDoesNotBlockNewOverlap_Integration: constraint
// anti-overlap hanya berlaku untuk delegasi non-revoked; setelah revoke,
// rentang yang sama dapat didelegasikan ulang.
func TestQARevokedDelegationDoesNotBlockNewOverlap_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	cleanupDelegationArtifacts(t, fixture)
	env := newDelegationEnv(t, fixture)

	body := gin.H{
		"delegator_position_id": env.delegatorPositionID,
		"delegate_user_id":      env.delegateUserID,
		"reason":                "probe revoked overlap",
		"valid_from":            time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
		"valid_to":              time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
	}
	rec := postDelegation(t, h, env.delegatorUserID, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first create status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Delegation delegationItem `json:"delegation"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create) error: %v", err)
	}
	if rec := revokeDelegation(t, h, env.delegatorUserID, created.Delegation.ID); rec.Code != http.StatusOK {
		t.Fatalf("revoke status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if rec := postDelegation(t, h, env.delegatorUserID, body); rec.Code != http.StatusCreated {
		t.Errorf("create after revoke status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}
}

// TestQANonUUIDPathParams_Integration mendokumentasikan perilaku id path yang
// bukan UUID pada endpoint baru. Harapan minimum: bukan 2xx dan tidak panic.
func TestQANonUUIDPathParams_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	cleanupDelegationArtifacts(t, fixture)
	env := newDelegationEnv(t, fixture)

	rec := revokeDelegation(t, h, env.delegatorUserID, "not-a-uuid")
	t.Logf("DELETE /delegations/not-a-uuid -> %d %s", rec.Code, rec.Body.String())
	if rec.Code >= 200 && rec.Code < 300 {
		t.Errorf("revoke non-uuid id status = %d, want non-2xx", rec.Code)
	}

	rec = cancelLetterReq(t, h, env.delegatorUserID, "not-a-uuid", "probe non uuid")
	t.Logf("POST /letters/view/not-a-uuid/cancel -> %d %s", rec.Code, rec.Body.String())
	if rec.Code >= 200 && rec.Code < 300 {
		t.Errorf("cancel non-uuid id status = %d, want non-2xx", rec.Code)
	}
}
