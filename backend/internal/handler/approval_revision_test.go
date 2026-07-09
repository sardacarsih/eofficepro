package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestInsertApprovalRoutePreservesRevisionHistory_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	ctx := context.Background()

	orgUnitID := fixture.insertOrgUnit(t, "REVUNIT", "Revision Unit")
	creatorUserID := fixture.insertUser(t, "REVCREATOR", "Revision Creator")
	approverUserID := fixture.insertUser(t, "REVAPPROVER", "Revision Approver")
	creatorPositionID := fixture.insertPosition(t, orgUnitID, "Revision Creator Position")
	approverPositionID := fixture.insertPosition(t, orgUnitID, "Revision Approver Position")
	letterID := fixture.insertDraftLetter(t, creatorUserID, creatorPositionID)

	var previousStepID string
	err := fixture.db.QueryRow(ctx, `
		INSERT INTO approval_steps
			(letter_id, approval_cycle, step_order, approver_position_id, flow_group, status)
		VALUES ($1, 1, 1, $2, 1, 'rejected')
		RETURNING id::text`, letterID, approverPositionID).Scan(&previousStepID)
	if err != nil {
		t.Fatalf("insert previous approval step error: %v", err)
	}
	if _, err := fixture.db.Exec(ctx, `
		INSERT INTO approval_actions
			(approval_step_id, action, acted_by_user_id, note)
		VALUES ($1, 'request_revision', $2, 'perbaiki isi surat')`,
		previousStepID, approverUserID); err != nil {
		t.Fatalf("insert revision action error: %v", err)
	}

	tx, err := fixture.db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin approval route transaction error: %v", err)
	}
	route := approvalRoute{
		SLAHours: 24,
		Steps: []approvalRouteStep{
			{
				StepOrder:    1,
				FlowGroup:    1,
				PositionID:   approverPositionID,
				PositionType: "dept_head",
				Title:        "Revision Approver Position",
			},
		},
	}
	approvalCycle, err := insertApprovalRoute(ctx, tx, letterID, route)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("insertApprovalRoute() error: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit approval route transaction error: %v", err)
	}

	if approvalCycle != 2 {
		t.Errorf("insertApprovalRoute() cycle = %d, want 2", approvalCycle)
	}

	var stepCount, actionCount int
	if err := fixture.db.QueryRow(ctx,
		`SELECT count(*) FROM approval_steps WHERE letter_id = $1`,
		letterID,
	).Scan(&stepCount); err != nil {
		t.Fatalf("count approval steps error: %v", err)
	}
	if stepCount != 2 {
		t.Errorf("approval step count = %d, want 2", stepCount)
	}
	if err := fixture.db.QueryRow(ctx, `
		SELECT count(*)
		FROM approval_actions action
		JOIN approval_steps step ON step.id = action.approval_step_id
		WHERE step.letter_id = $1`, letterID).Scan(&actionCount); err != nil {
		t.Fatalf("count approval actions error: %v", err)
	}
	if actionCount != 1 {
		t.Errorf("approval action count = %d, want 1", actionCount)
	}

	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request = httptest.NewRequest(http.MethodGet, "/letters/"+letterID, nil)
	actions, ok := h.loadLetterApprovalActions(ginContext, letterID)
	if !ok {
		t.Fatalf(
			"loadLetterApprovalActions(%q) failed: status = %d, body = %s",
			letterID,
			recorder.Code,
			recorder.Body.String(),
		)
	}
	if len(actions) != 1 {
		t.Fatalf("loadLetterApprovalActions(%q) length = %d, want 1", letterID, len(actions))
	}
	if actions[0].Action != "request_revision" {
		t.Errorf(
			"loadLetterApprovalActions(%q)[0].Action = %q, want request_revision",
			letterID,
			actions[0].Action,
		)
	}

	var currentStatus string
	if err := fixture.db.QueryRow(ctx, `
		SELECT status
		FROM approval_steps
		WHERE letter_id = $1 AND approval_cycle = 2 AND step_order = 1`,
		letterID,
	).Scan(&currentStatus); err != nil {
		t.Fatalf("query current approval step error: %v", err)
	}
	if currentStatus != "waiting" {
		t.Errorf("current approval status = %q, want waiting", currentStatus)
	}
}
