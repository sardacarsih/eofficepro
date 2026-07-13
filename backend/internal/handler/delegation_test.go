package handler

import (
	"bytes"
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

// cleanupDelegationArtifacts menghapus notifikasi, outbox, dan delegasi milik
// fixture sebelum cleanup fixture menghapus users/letters (urutan FK).
func cleanupDelegationArtifacts(t *testing.T, fixture *userPositionFixture) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = fixture.db.Exec(ctx, `
			DELETE FROM notification_outbox
			WHERE recipient_email LIKE '%' || $1 || '@example.test'
			   OR letter_id IN (SELECT id FROM letters WHERE creator_user_id = ANY($2::uuid[]))`,
			fixture.suffix, fixture.cleanupIDs)
		_, _ = fixture.db.Exec(ctx, `DELETE FROM notifications WHERE user_id = ANY($1::uuid[])`, fixture.cleanupIDs)
		_, _ = fixture.db.Exec(ctx, `
			DELETE FROM approval_actions
			WHERE on_behalf_delegation_id IN (
				SELECT id FROM delegations
				WHERE created_by = ANY($1::uuid[]) OR delegate_user_id = ANY($1::uuid[])
			)`, fixture.cleanupIDs)
		_, _ = fixture.db.Exec(ctx, `
			DELETE FROM delegations
			WHERE created_by = ANY($1::uuid[]) OR delegate_user_id = ANY($1::uuid[])`, fixture.cleanupIDs)
	})
}

func postDelegation(t *testing.T, h *Handler, userID string, body gin.H) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal delegation payload error: %v", err)
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/delegations", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(middleware.CtxUserID, userID)
	h.CreateDelegation(c)
	return rec
}

func listDelegations(t *testing.T, h *Handler, userID string, query string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/delegations"+query, nil)
	c.Set(middleware.CtxUserID, userID)
	h.ListDelegations(c)
	return rec
}

func revokeDelegation(t *testing.T, h *Handler, userID string, delegationID string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodDelete, "/delegations/"+delegationID, nil)
	c.Params = gin.Params{{Key: "id", Value: delegationID}}
	c.Set(middleware.CtxUserID, userID)
	h.RevokeDelegation(c)
	return rec
}

func actApprovalStep(t *testing.T, h *Handler, userID string, stepID string, body gin.H) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal action payload error: %v", err)
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/approvals/steps/"+stepID+"/actions", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: stepID}}
	c.Set(middleware.CtxUserID, userID)
	h.ActApprovalStep(c)
	return rec
}

type delegationEnv struct {
	orgUnitID           string
	delegatorUserID     string
	delegatorPositionID string
	delegateUserID      string
	delegatePositionID  string
}

// newDelegationEnv menyiapkan pemegang posisi delegator dan kandidat delegate
// (pemegang posisi lain pada company yang sama).
func newDelegationEnv(t *testing.T, fixture *userPositionFixture) delegationEnv {
	t.Helper()
	env := delegationEnv{}
	env.orgUnitID = fixture.insertOrgUnit(t, "DGUNIT", "Delegation Unit")
	env.delegatorUserID = fixture.insertUser(t, "DGHOLDER", "Delegation Holder")
	env.delegatorPositionID = fixture.insertPosition(t, env.orgUnitID, "Delegator Head")
	fixture.insertUserPosition(t, env.delegatorUserID, env.delegatorPositionID, "definitive")
	env.delegateUserID = fixture.insertUser(t, "DGDELEG", "Delegation Delegate")
	env.delegatePositionID = fixture.insertPosition(t, env.orgUnitID, "Delegate Staff")
	fixture.insertUserPosition(t, env.delegateUserID, env.delegatePositionID, "definitive")
	return env
}

// newWaitingApprovalLetter membuat surat in_approval dengan satu step waiting
// pada posisi approver. tag menjaga keunikan nik/kode dalam satu fixture.
func newWaitingApprovalLetter(t *testing.T, fixture *userPositionFixture, approverPositionID string, tag string) (creatorUserID, letterID, stepID string) {
	t.Helper()
	ctx := context.Background()

	orgUnitID := fixture.insertOrgUnit(t, "DGL"+tag, "Delegation Letter Unit "+tag)
	creatorUserID = fixture.insertUser(t, "DGC"+tag, "Delegation Letter Creator "+tag)
	creatorPositionID := fixture.insertPosition(t, orgUnitID, "Delegation Letter Creator Position "+tag)
	fixture.insertUserPosition(t, creatorUserID, creatorPositionID, "definitive")
	letterID = fixture.insertDraftLetter(t, creatorUserID, creatorPositionID)

	if _, err := fixture.db.Exec(ctx, `
		UPDATE letters SET status = 'in_approval', current_step_order = 1 WHERE id = $1`, letterID); err != nil {
		t.Fatalf("set letter in_approval error: %v", err)
	}
	if err := fixture.db.QueryRow(ctx, `
		INSERT INTO approval_steps (letter_id, approval_cycle, step_order, approver_position_id, flow_group, status, sla_deadline)
		VALUES ($1, 1, 1, $2, 1, 'waiting', now() + interval '24 hours')
		RETURNING id::text`, letterID, approverPositionID).Scan(&stepID); err != nil {
		t.Fatalf("insert waiting approval step error: %v", err)
	}
	return creatorUserID, letterID, stepID
}

func insertDelegationRow(t *testing.T, fixture *userPositionFixture, positionID, delegateUserID, createdBy string, validFrom, validTo time.Time) string {
	t.Helper()
	var id string
	if err := fixture.db.QueryRow(context.Background(), `
		INSERT INTO delegations (delegator_position_id, delegate_user_id, reason, valid_from, valid_to, created_by)
		VALUES ($1, $2, 'cuti tahunan', $3, $4, $5)
		RETURNING id::text`, positionID, delegateUserID, validFrom, validTo, createdBy).Scan(&id); err != nil {
		t.Fatalf("insert delegation row error: %v", err)
	}
	return id
}

func TestDelegationCreateListRevoke_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	cleanupDelegationArtifacts(t, fixture)
	env := newDelegationEnv(t, fixture)

	validFrom := time.Now().Add(-time.Hour).UTC()
	validTo := time.Now().Add(24 * time.Hour).UTC()
	rec := postDelegation(t, h, env.delegatorUserID, gin.H{
		"delegator_position_id": env.delegatorPositionID,
		"delegate_user_id":      env.delegateUserID,
		"reason":                "cuti tahunan",
		"valid_from":            validFrom.Format(time.RFC3339),
		"valid_to":              validTo.Format(time.RFC3339),
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("CreateDelegation status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Delegation delegationItem `json:"delegation"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create delegation) error: %v", err)
	}
	if created.Delegation.Status != "active" {
		t.Errorf("delegation status = %q, want active", created.Delegation.Status)
	}
	if created.Delegation.DelegateUserID != env.delegateUserID || created.Delegation.DelegateName == "" {
		t.Errorf("delegate identity = (%q, %q), want user %q with name", created.Delegation.DelegateUserID, created.Delegation.DelegateName, env.delegateUserID)
	}
	if created.Delegation.DelegatorPositionTitle == "" || created.Delegation.CreatedByName == "" {
		t.Errorf("delegation titles missing: %+v", created.Delegation)
	}

	// Notifikasi in-app untuk delegate dan pemegang posisi delegator.
	for _, target := range []string{env.delegateUserID, env.delegatorUserID} {
		var notified bool
		if err := fixture.db.QueryRow(context.Background(), `
			SELECT EXISTS (
				SELECT 1 FROM notifications
				WHERE user_id = $1 AND event_type = 'delegation_created'
			)`, target).Scan(&notified); err != nil {
			t.Fatalf("query delegation notification error: %v", err)
		}
		if !notified {
			t.Errorf("delegation_created notification for %q not found", target)
		}
	}

	// Audit create.
	var audited bool
	if err := fixture.db.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1 FROM audit_logs
			WHERE entity_type = 'delegation' AND entity_id = $1 AND action = 'create'
		)`, created.Delegation.ID).Scan(&audited); err != nil {
		t.Fatalf("query delegation audit error: %v", err)
	}
	if !audited {
		t.Error("audit log delegation create not found")
	}

	// List sebagai delegate.
	listRec := listDelegations(t, h, env.delegateUserID, "?scope=delegate")
	if listRec.Code != http.StatusOK {
		t.Fatalf("ListDelegations status = %d, body = %s", listRec.Code, listRec.Body.String())
	}
	var listed struct {
		Data []delegationItem `json:"data"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("json.Unmarshal(list delegations) error: %v", err)
	}
	if len(listed.Data) != 1 || listed.Data[0].ID != created.Delegation.ID {
		t.Errorf("delegate list = %+v, want single created delegation", listed.Data)
	}

	// Scope tidak valid.
	if rec := listDelegations(t, h, env.delegateUserID, "?scope=unknown"); rec.Code != http.StatusBadRequest {
		t.Errorf("ListDelegations invalid scope status = %d, want 400", rec.Code)
	}

	// Revoke oleh pembuat.
	revRec := revokeDelegation(t, h, env.delegatorUserID, created.Delegation.ID)
	if revRec.Code != http.StatusOK {
		t.Fatalf("RevokeDelegation status = %d, body = %s", revRec.Code, revRec.Body.String())
	}
	var revoked struct {
		Delegation delegationItem `json:"delegation"`
	}
	if err := json.Unmarshal(revRec.Body.Bytes(), &revoked); err != nil {
		t.Fatalf("json.Unmarshal(revoke delegation) error: %v", err)
	}
	if revoked.Delegation.Status != "revoked" || revoked.Delegation.RevokedAt == nil {
		t.Errorf("revoked delegation = %+v, want status revoked with revoked_at", revoked.Delegation)
	}

	// Revoke ulang -> 409.
	if rec := revokeDelegation(t, h, env.delegatorUserID, created.Delegation.ID); rec.Code != http.StatusConflict {
		t.Errorf("second revoke status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}

	// include_past default menyembunyikan revoked.
	listRec = listDelegations(t, h, env.delegateUserID, "?scope=delegate")
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("json.Unmarshal(list after revoke) error: %v", err)
	}
	if len(listed.Data) != 0 {
		t.Errorf("delegate list after revoke = %+v, want empty (include_past default false)", listed.Data)
	}
	listRec = listDelegations(t, h, env.delegateUserID, "?scope=delegate&include_past=true")
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("json.Unmarshal(list include_past) error: %v", err)
	}
	if len(listed.Data) != 1 || listed.Data[0].Status != "revoked" {
		t.Errorf("delegate list include_past = %+v, want revoked item", listed.Data)
	}
}

func TestDelegationCreateValidationAndAuthorization_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	cleanupDelegationArtifacts(t, fixture)
	env := newDelegationEnv(t, fixture)
	ctx := context.Background()

	validBody := func(mutate func(gin.H)) gin.H {
		body := gin.H{
			"delegator_position_id": env.delegatorPositionID,
			"delegate_user_id":      env.delegateUserID,
			"reason":                "dinas luar",
			"valid_from":            time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
			"valid_to":              time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
		}
		if mutate != nil {
			mutate(body)
		}
		return body
	}

	// 400: alasan kosong, format tanggal salah, rentang terbalik, valid_to lampau.
	badRequests := []struct {
		name   string
		mutate func(gin.H)
	}{
		{"empty_reason", func(b gin.H) { b["reason"] = "   " }},
		{"bad_valid_from", func(b gin.H) { b["valid_from"] = "2026-13-99" }},
		{"from_after_to", func(b gin.H) {
			b["valid_from"] = time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
		}},
		{"to_in_past", func(b gin.H) {
			b["valid_from"] = time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
			b["valid_to"] = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
		}},
		{"self_delegation", func(b gin.H) { b["delegate_user_id"] = env.delegatorUserID }},
	}
	for _, tt := range badRequests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := postDelegation(t, h, env.delegatorUserID, validBody(tt.mutate)); rec.Code != http.StatusBadRequest {
				t.Errorf("CreateDelegation(%s) status = %d, want 400; body = %s", tt.name, rec.Code, rec.Body.String())
			}
		})
	}

	// 403: bukan pemegang posisi delegator dan bukan admin company.
	if rec := postDelegation(t, h, env.delegateUserID, validBody(nil)); rec.Code != http.StatusForbidden {
		t.Errorf("CreateDelegation(non-holder) status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}

	// 404: posisi delegator tidak ada.
	if rec := postDelegation(t, h, env.delegatorUserID, validBody(func(b gin.H) {
		b["delegator_position_id"] = "00000000-0000-0000-0000-000000000000"
	})); rec.Code != http.StatusNotFound {
		t.Errorf("CreateDelegation(missing position) status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}

	// 400: delegate lintas company.
	var otherCompanyID string
	if err := fixture.db.QueryRow(ctx, `
		INSERT INTO companies (code, name)
		VALUES ($1, $2)
		RETURNING id::text`, "TDG"+fixture.suffix, "Delegation Other "+fixture.suffix).Scan(&otherCompanyID); err != nil {
		t.Fatalf("insert other company error: %v", err)
	}
	var otherOrgUnitID string
	if err := fixture.db.QueryRow(ctx, `
		INSERT INTO org_units (company_id, code, name, unit_level)
		VALUES ($1, $2, $3, 'department')
		RETURNING id::text`, otherCompanyID, "DGX"+fixture.suffix, "Delegation Other Unit "+fixture.suffix).Scan(&otherOrgUnitID); err != nil {
		t.Fatalf("insert other org unit error: %v", err)
	}
	var otherPositionID string
	if err := fixture.db.QueryRow(ctx, `
		INSERT INTO positions (org_unit_id, title, position_type)
		VALUES ($1, $2, 'dept_head')
		RETURNING id::text`, otherOrgUnitID, "Delegation Other Head "+fixture.suffix).Scan(&otherPositionID); err != nil {
		t.Fatalf("insert other position error: %v", err)
	}
	outsiderUserID := fixture.insertUser(t, "DGOUT", "Delegation Outsider")
	fixture.insertUserPosition(t, outsiderUserID, otherPositionID, "definitive")
	t.Cleanup(func() {
		_, _ = fixture.db.Exec(ctx, `DELETE FROM user_positions WHERE position_id = $1`, otherPositionID)
		_, _ = fixture.db.Exec(ctx, `DELETE FROM positions WHERE id = $1`, otherPositionID)
		_, _ = fixture.db.Exec(ctx, `DELETE FROM org_units WHERE id = $1`, otherOrgUnitID)
		_, _ = fixture.db.Exec(ctx, `DELETE FROM companies WHERE id = $1`, otherCompanyID)
	})

	if rec := postDelegation(t, h, env.delegatorUserID, validBody(func(b gin.H) {
		b["delegate_user_id"] = outsiderUserID
	})); rec.Code != http.StatusBadRequest {
		t.Errorf("CreateDelegation(cross-company delegate) status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}

	// Buat satu delegasi sah lalu uji revoke lintas company -> 404.
	rec := postDelegation(t, h, env.delegatorUserID, validBody(nil))
	if rec.Code != http.StatusCreated {
		t.Fatalf("CreateDelegation(valid) status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Delegation delegationItem `json:"delegation"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create delegation) error: %v", err)
	}
	if rec := revokeDelegation(t, h, outsiderUserID, created.Delegation.ID); rec.Code != http.StatusNotFound {
		t.Errorf("RevokeDelegation(cross-company) status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	// User se-company tanpa hak -> 403.
	bystanderUserID := fixture.insertUser(t, "DGBYSTD", "Delegation Bystander")
	bystanderPositionID := fixture.insertPosition(t, env.orgUnitID, "Delegation Bystander Position")
	fixture.insertUserPosition(t, bystanderUserID, bystanderPositionID, "definitive")
	if rec := revokeDelegation(t, h, bystanderUserID, created.Delegation.ID); rec.Code != http.StatusForbidden {
		t.Errorf("RevokeDelegation(bystander) status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
	// 404: id tidak ada.
	if rec := revokeDelegation(t, h, env.delegatorUserID, "00000000-0000-0000-0000-000000000000"); rec.Code != http.StatusNotFound {
		t.Errorf("RevokeDelegation(missing id) status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
}

func TestDelegationOverlapConcurrent_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	cleanupDelegationArtifacts(t, fixture)
	env := newDelegationEnv(t, fixture)

	secondDelegateID := fixture.insertUser(t, "DGDELEG2", "Delegation Second Delegate")
	secondDelegatePos := fixture.insertPosition(t, env.orgUnitID, "Second Delegate Staff")
	fixture.insertUserPosition(t, secondDelegateID, secondDelegatePos, "definitive")

	makeBody := func(delegateID string) gin.H {
		return gin.H{
			"delegator_position_id": env.delegatorPositionID,
			"delegate_user_id":      delegateID,
			"reason":                "overlap konkuren",
			"valid_from":            time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
			"valid_to":              time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
		}
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	codes := make([]int, 2)
	delegates := []string{env.delegateUserID, secondDelegateID}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			codes[i] = postDelegation(t, h, env.delegatorUserID, makeBody(delegates[i])).Code
		}(i)
	}
	close(start)
	wg.Wait()

	created, conflicted := 0, 0
	for _, code := range codes {
		switch code {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicted++
		}
	}
	if created != 1 || conflicted != 1 {
		t.Fatalf("concurrent overlap codes = %v, want exactly one 201 and one 409", codes)
	}

	// Overlap sekuensial juga 409.
	if rec := postDelegation(t, h, env.delegatorUserID, makeBody(secondDelegateID)); rec.Code != http.StatusConflict {
		t.Errorf("sequential overlap status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}
}

func TestDelegationActOnBehalfAndBoundaries_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	cleanupDelegationArtifacts(t, fixture)
	env := newDelegationEnv(t, fixture)
	ctx := context.Background()

	_, letterID, stepID := newWaitingApprovalLetter(t, fixture, env.delegatorPositionID, "A")

	// Boundary: delegasi masih scheduled -> tidak ada akses.
	scheduledID := insertDelegationRow(t, fixture, env.delegatorPositionID, env.delegateUserID, env.delegatorUserID,
		time.Now().Add(2*time.Hour), time.Now().Add(24*time.Hour))
	if allowed, err := h.userCanViewLetter(ctx, env.delegateUserID, letterID); err != nil || allowed {
		t.Errorf("userCanViewLetter(scheduled delegation) = (%t, %v), want (false, nil)", allowed, err)
	}
	if rec := actApprovalStep(t, h, env.delegateUserID, stepID, gin.H{"action": "request_revision", "note": "belum berlaku"}); rec.Code != http.StatusNotFound {
		t.Errorf("act with scheduled delegation status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	if _, err := fixture.db.Exec(ctx, `DELETE FROM delegations WHERE id = $1`, scheduledID); err != nil {
		t.Fatalf("delete scheduled delegation error: %v", err)
	}

	// Boundary: delegasi sudah lewat -> tidak ada akses.
	expiredID := insertDelegationRow(t, fixture, env.delegatorPositionID, env.delegateUserID, env.delegatorUserID,
		time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour))
	if allowed, err := h.userCanViewLetter(ctx, env.delegateUserID, letterID); err != nil || allowed {
		t.Errorf("userCanViewLetter(expired delegation) = (%t, %v), want (false, nil)", allowed, err)
	}
	if _, err := fixture.db.Exec(ctx, `DELETE FROM delegations WHERE id = $1`, expiredID); err != nil {
		t.Fatalf("delete expired delegation error: %v", err)
	}

	// Delegasi aktif: inbox + view + act "a.n.".
	delegationID := insertDelegationRow(t, fixture, env.delegatorPositionID, env.delegateUserID, env.delegatorUserID,
		time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))

	if allowed, err := h.userCanViewLetter(ctx, env.delegateUserID, letterID); err != nil || !allowed {
		t.Errorf("userCanViewLetter(active delegation) = (%t, %v), want (true, nil)", allowed, err)
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
		Data []struct {
			StepID             string  `json:"step_id"`
			IsDelegated        bool    `json:"is_delegated"`
			DelegatedFromTitle *string `json:"delegated_from_title"`
		} `json:"data"`
	}
	if err := json.Unmarshal(inboxRec.Body.Bytes(), &inbox); err != nil {
		t.Fatalf("json.Unmarshal(inbox) error: %v", err)
	}
	if len(inbox.Data) != 1 || inbox.Data[0].StepID != stepID {
		t.Fatalf("delegate inbox = %+v, want single step %q", inbox.Data, stepID)
	}
	if !inbox.Data[0].IsDelegated || inbox.Data[0].DelegatedFromTitle == nil {
		t.Errorf("delegate inbox item = %+v, want is_delegated + delegated_from_title", inbox.Data[0])
	}

	// Act oleh delegate -> on_behalf_delegation_id terisi.
	actRec := actApprovalStep(t, h, env.delegateUserID, stepID, gin.H{"action": "request_revision", "note": "mohon perbaiki lampiran"})
	if actRec.Code != http.StatusOK {
		t.Fatalf("delegate act status = %d, body = %s", actRec.Code, actRec.Body.String())
	}
	var onBehalfID *string
	if err := fixture.db.QueryRow(ctx, `
		SELECT on_behalf_delegation_id::text
		FROM approval_actions
		WHERE approval_step_id = $1`, stepID).Scan(&onBehalfID); err != nil {
		t.Fatalf("query on_behalf_delegation_id error: %v", err)
	}
	if onBehalfID == nil || *onBehalfID != delegationID {
		t.Errorf("on_behalf_delegation_id = %v, want %q", onBehalfID, delegationID)
	}

	// Detail actions mengekspos on_behalf_of + judul posisi delegator.
	detailRec := httptest.NewRecorder()
	dc, _ := gin.CreateTestContext(detailRec)
	dc.Request = httptest.NewRequest(http.MethodGet, "/letters/view/"+letterID, nil)
	actions, ok := h.loadLetterApprovalActions(dc, letterID)
	if !ok {
		t.Fatalf("loadLetterApprovalActions failed: %s", detailRec.Body.String())
	}
	if len(actions) != 1 || !actions[0].OnBehalfOf || actions[0].OnBehalfOfPositionTitle == nil {
		t.Errorf("approval actions = %+v, want on_behalf_of with delegator title", actions)
	}
}

func TestDelegationDirectCapacityWinsAndRevokeImmediate_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	cleanupDelegationArtifacts(t, fixture)
	env := newDelegationEnv(t, fixture)
	ctx := context.Background()

	_, _, stepID := newWaitingApprovalLetter(t, fixture, env.delegatorPositionID, "B")

	delegationID := insertDelegationRow(t, fixture, env.delegatorPositionID, env.delegateUserID, env.delegatorUserID,
		time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))

	// Delegate lalu diangkat jadi pemegang langsung posisi step:
	// kapasitas langsung menang -> on_behalf_delegation_id NULL.
	fixture.insertUserPosition(t, env.delegateUserID, env.delegatorPositionID, "plt")
	actRec := actApprovalStep(t, h, env.delegateUserID, stepID, gin.H{"action": "request_revision", "note": "sebagai pemegang langsung"})
	if actRec.Code != http.StatusOK {
		t.Fatalf("direct-capacity act status = %d, body = %s", actRec.Code, actRec.Body.String())
	}
	var onBehalfID *string
	if err := fixture.db.QueryRow(ctx, `
		SELECT on_behalf_delegation_id::text
		FROM approval_actions
		WHERE approval_step_id = $1`, stepID).Scan(&onBehalfID); err != nil {
		t.Fatalf("query on_behalf_delegation_id error: %v", err)
	}
	if onBehalfID != nil {
		t.Errorf("on_behalf_delegation_id = %v, want NULL (kapasitas langsung menang)", *onBehalfID)
	}

	// Revoke berlaku seketika: siapkan surat kedua, cabut, akses hilang.
	if _, err := fixture.db.Exec(ctx, `
		DELETE FROM user_positions WHERE user_id = $1 AND position_id = $2`,
		env.delegateUserID, env.delegatorPositionID); err != nil {
		t.Fatalf("remove direct assignment error: %v", err)
	}
	_, letter2ID, step2ID := newWaitingApprovalLetter(t, fixture, env.delegatorPositionID, "C")
	if allowed, err := h.userCanViewLetter(ctx, env.delegateUserID, letter2ID); err != nil || !allowed {
		t.Fatalf("userCanViewLetter(before revoke) = (%t, %v), want (true, nil)", allowed, err)
	}
	if rec := revokeDelegation(t, h, env.delegatorUserID, delegationID); rec.Code != http.StatusOK {
		t.Fatalf("RevokeDelegation status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if allowed, err := h.userCanViewLetter(ctx, env.delegateUserID, letter2ID); err != nil || allowed {
		t.Errorf("userCanViewLetter(after revoke) = (%t, %v), want (false, nil)", allowed, err)
	}
	if rec := actApprovalStep(t, h, env.delegateUserID, step2ID, gin.H{"action": "request_revision", "note": "sudah dicabut"}); rec.Code != http.StatusNotFound {
		t.Errorf("act after revoke status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
}

func TestDelegationNotificationsAndSLAReachDelegate_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	cleanupDelegationArtifacts(t, fixture)
	env := newDelegationEnv(t, fixture)
	ctx := context.Background()

	_, letterID, stepID := newWaitingApprovalLetter(t, fixture, env.delegatorPositionID, "D")
	insertDelegationRow(t, fixture, env.delegatorPositionID, env.delegateUserID, env.delegatorUserID,
		time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))

	// notifyWaitingApprovers menjangkau pemegang langsung + delegate (dedup).
	tx, err := fixture.db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin notify transaction error: %v", err)
	}
	emails, err := notifyWaitingApprovers(ctx, tx, letterID)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("notifyWaitingApprovers error: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit notify transaction error: %v", err)
	}
	if len(emails) != 2 {
		t.Errorf("notifyWaitingApprovers emails = %d penerima, want 2 (holder + delegate); %+v", len(emails), emails)
	}
	for _, target := range []string{env.delegatorUserID, env.delegateUserID} {
		var count int
		if err := fixture.db.QueryRow(ctx, `
			SELECT count(*) FROM notifications
			WHERE user_id = $1 AND letter_id = $2 AND event_type = 'approval_waiting'`,
			target, letterID).Scan(&count); err != nil {
			t.Fatalf("count approval_waiting notification error: %v", err)
		}
		if count != 1 {
			t.Errorf("approval_waiting notifications for %q = %d, want 1", target, count)
		}
	}

	// SLA reminder & eskalasi menjangkau delegate.
	if _, err := fixture.db.Exec(ctx, `
		UPDATE approval_steps SET sla_deadline = now() + interval '1 minute' WHERE id = $1`, stepID); err != nil {
		t.Fatalf("set near sla_deadline error: %v", err)
	}
	if _, err := h.insertSLAReminders(ctx); err != nil {
		t.Fatalf("insertSLAReminders error: %v", err)
	}
	var reminded bool
	if err := fixture.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM notifications
			WHERE user_id = $1 AND letter_id = $2 AND event_type = 'sla_reminder'
		)`, env.delegateUserID, letterID).Scan(&reminded); err != nil {
		t.Fatalf("query sla_reminder error: %v", err)
	}
	if !reminded {
		t.Error("sla_reminder for delegate not found")
	}

	if _, err := fixture.db.Exec(ctx, `
		UPDATE approval_steps SET sla_deadline = now() - interval '1 minute' WHERE id = $1`, stepID); err != nil {
		t.Fatalf("set past sla_deadline error: %v", err)
	}
	if _, err := h.insertSLAEscalations(ctx); err != nil {
		t.Fatalf("insertSLAEscalations error: %v", err)
	}
	var escalated bool
	if err := fixture.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM notifications
			WHERE user_id = $1 AND letter_id = $2 AND event_type = 'sla_escalation'
		)`, env.delegateUserID, letterID).Scan(&escalated); err != nil {
		t.Fatalf("query sla_escalation error: %v", err)
	}
	if !escalated {
		t.Error("sla_escalation for delegate not found")
	}
}

func TestDelegationDelegateOptions_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	cleanupDelegationArtifacts(t, fixture)
	env := newDelegationEnv(t, fixture)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/delegations/delegate-options?position_id="+env.delegatorPositionID, nil)
	c.Set(middleware.CtxUserID, env.delegatorUserID)
	h.ListDelegateOptions(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("ListDelegateOptions status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Data []struct {
			UserID         string   `json:"user_id"`
			FullName       string   `json:"full_name"`
			PositionTitles []string `json:"position_titles"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(delegate options) error: %v", err)
	}
	foundDelegate := false
	for _, option := range got.Data {
		if option.UserID == env.delegatorUserID {
			t.Errorf("delegate options must exclude holder of delegator position: %+v", option)
		}
		if option.UserID == env.delegateUserID {
			foundDelegate = true
			if len(option.PositionTitles) == 0 {
				t.Errorf("delegate option position_titles empty: %+v", option)
			}
		}
	}
	if !foundDelegate {
		t.Errorf("delegate options = %+v, want delegate user included", got.Data)
	}

	// 403 untuk non-pemegang non-admin.
	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/delegations/delegate-options?position_id="+env.delegatorPositionID, nil)
	c.Set(middleware.CtxUserID, env.delegateUserID)
	h.ListDelegateOptions(c)
	if rec.Code != http.StatusForbidden {
		t.Errorf("ListDelegateOptions(non-holder) status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
}
