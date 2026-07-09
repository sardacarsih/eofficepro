package handler

import (
	"context"
	"testing"
)

func TestUpdateDispositionRecipientBindsStatusParameters_Integration(t *testing.T) {
	_, fixture := newUserPositionFixture(t)
	ctx := context.Background()

	orgUnitID := fixture.insertOrgUnit(t, "DISPUNIT", "Disposition Unit")
	creatorUserID := fixture.insertUser(t, "DISPCREATOR", "Disposition Creator")
	recipientUserID := fixture.insertUser(t, "DISPRECIPIENT", "Disposition Recipient")
	creatorPositionID := fixture.insertPosition(t, orgUnitID, "Disposition Creator Position")
	recipientPositionID := fixture.insertPosition(t, orgUnitID, "Disposition Recipient Position")
	fixture.insertUserPosition(t, recipientUserID, recipientPositionID, "definitive")
	letterID := fixture.insertDraftLetter(t, creatorUserID, creatorPositionID)

	var dispositionID string
	err := fixture.db.QueryRow(ctx, `
		INSERT INTO dispositions
			(letter_id, from_position_id, instruction, created_by)
		VALUES ($1, $2, 'uji tindak lanjut', $3)
		RETURNING id::text`,
		letterID,
		creatorPositionID,
		creatorUserID,
	).Scan(&dispositionID)
	if err != nil {
		t.Fatalf("insert disposition error: %v", err)
	}

	var recipientID string
	err = fixture.db.QueryRow(ctx, `
		INSERT INTO disposition_recipients (disposition_id, position_id)
		VALUES ($1, $2)
		RETURNING id::text`, dispositionID, recipientPositionID).Scan(&recipientID)
	if err != nil {
		t.Fatalf("insert disposition recipient error: %v", err)
	}

	tx, err := fixture.db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin disposition update transaction error: %v", err)
	}
	if err := updateDispositionRecipient(
		ctx,
		tx,
		recipientID,
		"in_progress",
		"sedang diproses",
	); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("updateDispositionRecipient(in_progress) error: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit disposition update transaction error: %v", err)
	}

	var status string
	var followupNote *string
	if err := fixture.db.QueryRow(ctx, `
		SELECT status, followup_note
		FROM disposition_recipients
		WHERE id = $1`, recipientID).Scan(&status, &followupNote); err != nil {
		t.Fatalf("query disposition recipient error: %v", err)
	}
	if status != "in_progress" {
		t.Errorf("disposition status = %q, want in_progress", status)
	}
	if followupNote == nil || *followupNote != "sedang diproses" {
		t.Errorf("disposition followup note = %v, want sedang diproses", followupNote)
	}

	tx, err = fixture.db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin disposition completion transaction error: %v", err)
	}
	if err := updateDispositionRecipient(
		ctx,
		tx,
		recipientID,
		"done",
		"pekerjaan selesai",
	); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("updateDispositionRecipient(done) error: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit disposition completion transaction error: %v", err)
	}

	var completed bool
	if err := fixture.db.QueryRow(ctx, `
		SELECT status = 'done' AND completed_at IS NOT NULL
		FROM disposition_recipients
		WHERE id = $1`, recipientID).Scan(&completed); err != nil {
		t.Fatalf("query completed disposition recipient error: %v", err)
	}
	if !completed {
		t.Error("completed disposition recipient = false, want true")
	}
}
