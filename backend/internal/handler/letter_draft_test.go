package handler

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestValidateDraftRecipientDirectoratePolicy(t *testing.T) {
	dirA := "dir-a"
	dirB := "dir-b"

	tests := []struct {
		name        string
		creatorType string
		targetType  string
		targetDir   *string
		wantErr     bool
	}{
		{
			name:        "staff_same_directorate_allowed",
			creatorType: "staff",
			targetType:  "position",
			targetDir:   &dirA,
		},
		{
			name:        "division_head_same_directorate_position_allowed",
			creatorType: "division_head",
			targetType:  "position",
			targetDir:   &dirA,
		},
		{
			name:        "division_head_cross_directorate_rejected",
			creatorType: "division_head",
			targetType:  "position",
			targetDir:   &dirB,
			wantErr:     true,
		},
		{
			name:        "division_head_same_directorate_allowed",
			creatorType: "division_head",
			targetType:  "org_unit",
			targetDir:   &dirA,
		},
		{
			name:        "staff_cross_directorate_rejected",
			creatorType: "staff",
			targetType:  "position",
			targetDir:   &dirB,
			wantErr:     true,
		},
		{
			name:        "dept_head_cross_directorate_position_allowed",
			creatorType: "dept_head",
			targetType:  "position",
			targetDir:   &dirB,
		},
		{
			name:        "sub_dept_head_cross_directorate_position_allowed",
			creatorType: "sub_dept_head",
			targetType:  "position",
			targetDir:   &dirB,
		},
		{
			name:        "gm_cross_directorate_unit_rejected",
			creatorType: "gm",
			targetType:  "org_unit",
			targetDir:   &dirB,
			wantErr:     true,
		},
		{
			name:        "dept_head_same_directorate_unit_allowed",
			creatorType: "dept_head",
			targetType:  "org_unit",
			targetDir:   &dirA,
		},
		{
			name:        "missing_target_directorate_allowed",
			creatorType: "staff",
			targetType:  "position",
			targetDir:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDraftRecipientDirectoratePolicy(
				draftPositionScope{
					PositionType:  tt.creatorType,
					DirectorateID: &dirA,
				},
				draftRecipientScope{
					TargetType:    tt.targetType,
					DirectorateID: tt.targetDir,
				},
			)
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("validateDraftRecipientDirectoratePolicy(%q, %q, %v) error = %v, want error presence = %t",
					tt.creatorType, tt.targetType, tt.targetDir, err, tt.wantErr)
			}
		})
	}
}

func TestValidateDraftRecipientTargets_Integration(t *testing.T) {
	databaseURL := os.Getenv("EOFFICE_INTEGRATION_DB_URL")
	if databaseURL == "" {
		t.Skip("set EOFFICE_INTEGRATION_DB_URL to run Postgres-backed recipient validation tests")
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New(%q) error: %v", databaseURL, err)
	}
	t.Cleanup(db.Close)

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("db.Begin() error: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	fixture := insertDraftRecipientPolicyFixture(t, ctx, tx)
	h := &Handler{}

	tests := []struct {
		name              string
		creatorPositionID string
		recipient         draftRecipientRequest
		wantErrContains   string
	}{
		{
			name:              "staff_same_directorate_position_allowed",
			creatorPositionID: fixture.staffDirA,
			recipient: draftRecipientRequest{
				Type:       "to",
				TargetType: "position",
				TargetID:   fixture.managerDirA,
			},
		},
		{
			name:              "staff_cross_directorate_position_rejected",
			creatorPositionID: fixture.staffDirA,
			recipient: draftRecipientRequest{
				Type:       "to",
				TargetType: "position",
				TargetID:   fixture.managerDirB,
			},
			wantErrContains: "level manager ke atas",
		},
		{
			name:              "dept_head_cross_directorate_position_allowed",
			creatorPositionID: fixture.managerDirA,
			recipient: draftRecipientRequest{
				Type:       "to",
				TargetType: "position",
				TargetID:   fixture.managerDirB,
			},
		},
		{
			name:              "gm_cross_directorate_unit_rejected",
			creatorPositionID: fixture.gmDirA,
			recipient: draftRecipientRequest{
				Type:       "to",
				TargetType: "org_unit",
				TargetID:   fixture.deptDirB,
			},
			wantErrContains: "penerima unit lintas direktorat",
		},
		{
			name:              "dept_head_same_directorate_unit_allowed",
			creatorPositionID: fixture.managerDirA,
			recipient: draftRecipientRequest{
				Type:       "to",
				TargetType: "org_unit",
				TargetID:   fixture.deptDirA,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.validateDraftRecipientTargets(ctx, tx, tt.creatorPositionID, []draftRecipientRequest{tt.recipient})
			if tt.wantErrContains == "" {
				if err != nil {
					t.Errorf("validateDraftRecipientTargets(%q, %+v) error = %v, want nil",
						tt.creatorPositionID, tt.recipient, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Errorf("validateDraftRecipientTargets(%q, %+v) error = %v, want containing %q",
					tt.creatorPositionID, tt.recipient, err, tt.wantErrContains)
			}
		})
	}
}

type draftRecipientPolicyFixture struct {
	deptDirA    string
	deptDirB    string
	staffDirA   string
	managerDirA string
	managerDirB string
	gmDirA      string
}

func insertDraftRecipientPolicyFixture(t *testing.T, ctx context.Context, tx pgxTx) draftRecipientPolicyFixture {
	t.Helper()

	var companyID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO companies (code, name)
		VALUES ('TSTXDIR', 'Test Cross Directorate')
		RETURNING id::text`).Scan(&companyID); err != nil {
		t.Fatalf("insert company error: %v", err)
	}

	dirA := insertOrgUnit(t, ctx, tx, companyID, nil, "TDIRA", "Test Directorate A", "directorate")
	dirB := insertOrgUnit(t, ctx, tx, companyID, nil, "TDIRB", "Test Directorate B", "directorate")
	deptDirA := insertOrgUnit(t, ctx, tx, companyID, &dirA, "TDEPA", "Test Department A", "department")
	deptDirB := insertOrgUnit(t, ctx, tx, companyID, &dirB, "TDEPB", "Test Department B", "department")

	return draftRecipientPolicyFixture{
		deptDirA:    deptDirA,
		deptDirB:    deptDirB,
		staffDirA:   insertPosition(t, ctx, tx, deptDirA, "Staff A", "staff"),
		managerDirA: insertPosition(t, ctx, tx, deptDirA, "Department Head A", "dept_head"),
		managerDirB: insertPosition(t, ctx, tx, deptDirB, "Department Head B", "dept_head"),
		gmDirA:      insertPosition(t, ctx, tx, dirA, "GM A", "gm"),
	}
}

type pgxTx interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func insertOrgUnit(t *testing.T, ctx context.Context, tx pgxTx, companyID string, parentID *string, code string, name string, level string) string {
	t.Helper()

	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO org_units (company_id, parent_id, code, name, unit_level)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text`, companyID, parentID, code, name, level).Scan(&id)
	if err != nil {
		t.Fatalf("insert org unit %q error: %v", code, err)
	}
	return id
}

func insertPosition(t *testing.T, ctx context.Context, tx pgxTx, orgUnitID string, title string, positionType string) string {
	t.Helper()

	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO positions (org_unit_id, title, position_type)
		VALUES ($1, $2, $3)
		RETURNING id::text`, orgUnitID, title, positionType).Scan(&id)
	if err != nil {
		t.Fatalf("insert position %q error: %v", title, err)
	}
	return id
}
