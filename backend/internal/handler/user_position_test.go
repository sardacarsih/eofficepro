package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

func TestListUsersIncludesMultipleActiveAssignments_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)

	userID := fixture.insertUser(t, "USR1", "Multi Holder")
	orgUnitID := fixture.insertOrgUnit(t, "UNIT1", "Unit One")
	positionA := fixture.insertPosition(t, orgUnitID, "Head A")
	positionB := fixture.insertPosition(t, orgUnitID, "Acting Head B")
	assignmentA := fixture.insertUserPosition(t, userID, positionA, "definitive")
	assignmentB := fixture.insertUserPosition(t, userID, positionB, "plt")

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/users", nil)
	c.Set(middleware.CtxUserID, fixture.actorUserID)

	h.ListUsers(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("ListUsers status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var got struct {
		Users []struct {
			ID        string                   `json:"id"`
			Positions []userPositionAssignment `json:"positions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(ListUsers response) error: %v", err)
	}

	var positions []userPositionAssignment
	for _, user := range got.Users {
		if user.ID == userID {
			positions = user.Positions
			break
		}
	}
	if len(positions) != 2 {
		t.Fatalf("user positions length = %d, want 2; positions = %+v", len(positions), positions)
	}

	gotAssignments := map[string]bool{}
	for _, position := range positions {
		gotAssignments[position.AssignmentID] = true
		if position.CompanyID != fixture.companyID || position.CompanyCode == "" || position.CompanyName == "" {
			t.Fatalf("position company identity missing: %+v", position)
		}
	}
	if !gotAssignments[assignmentA] || !gotAssignments[assignmentB] {
		t.Fatalf("positions missing assignments %q/%q: %+v", assignmentA, assignmentB, positions)
	}
}

func TestUserCanUsePositionForCompanyRejectsMismatchedOrInactiveScope_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	ctx := context.Background()

	userID := fixture.insertUser(t, "USRSCOPE", "Scoped Holder")
	orgUnitID := fixture.insertOrgUnit(t, "SCOPE", "Scoped Unit")
	positionID := fixture.insertPosition(t, orgUnitID, "Scoped Position")
	fixture.insertUserPosition(t, userID, positionID, "definitive")

	allowed, err := h.userCanUsePositionForCompany(ctx, userID, positionID, fixture.companyID)
	if err != nil || !allowed {
		t.Fatalf("userCanUsePositionForCompany matching scope = (%t, %v), want (true, nil)", allowed, err)
	}

	var otherCompanyID string
	if err := fixture.db.QueryRow(ctx, `
		INSERT INTO companies (code, name)
		VALUES ($1, $2)
		RETURNING id::text`, "TUX"+fixture.suffix, "Other Scope "+fixture.suffix).Scan(&otherCompanyID); err != nil {
		t.Fatalf("insert other company error: %v", err)
	}
	t.Cleanup(func() {
		_, _ = fixture.db.Exec(ctx, `DELETE FROM companies WHERE id = $1`, otherCompanyID)
	})

	allowed, err = h.userCanUsePositionForCompany(ctx, userID, positionID, otherCompanyID)
	if err != nil || allowed {
		t.Fatalf("userCanUsePositionForCompany mismatched company = (%t, %v), want (false, nil)", allowed, err)
	}

	if _, err := fixture.db.Exec(ctx, `UPDATE positions SET is_active = false WHERE id = $1`, positionID); err != nil {
		t.Fatalf("deactivate position error: %v", err)
	}
	allowed, err = h.userCanUsePositionForCompany(ctx, userID, positionID, fixture.companyID)
	if err != nil || allowed {
		t.Fatalf("userCanUsePositionForCompany inactive position = (%t, %v), want (false, nil)", allowed, err)
	}
}

func TestEndUserPositionAssignmentSoftEndsActiveRow_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)

	userID := fixture.insertUser(t, "USR2", "Ended Holder")
	orgUnitID := fixture.insertOrgUnit(t, "UNIT2", "Unit Two")
	positionID := fixture.insertPosition(t, orgUnitID, "Head")
	assignmentID := fixture.insertUserPosition(t, userID, positionID, "definitive")

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodDelete, "/user-positions/"+assignmentID, nil)
	c.Params = gin.Params{{Key: "id", Value: assignmentID}}
	c.Set(middleware.CtxUserID, fixture.actorUserID)

	h.EndUserPositionAssignment(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("EndUserPositionAssignment status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var validTo string
	err := fixture.db.QueryRow(context.Background(), `
		SELECT valid_to::text
		FROM user_positions
		WHERE id = $1`, assignmentID).Scan(&validTo)
	if err != nil {
		t.Fatalf("query ended assignment error: %v", err)
	}
	var databaseToday string
	if err := fixture.db.QueryRow(context.Background(), `SELECT current_date::text`).Scan(&databaseToday); err != nil {
		t.Fatalf("query database current date error: %v", err)
	}
	if validTo != databaseToday {
		t.Fatalf("valid_to = %q, want current date", validTo)
	}

	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodDelete, "/user-positions/"+assignmentID, nil)
	c.Params = gin.Params{{Key: "id", Value: assignmentID}}
	c.Set(middleware.CtxUserID, fixture.actorUserID)

	h.EndUserPositionAssignment(c)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("second EndUserPositionAssignment status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
}

func TestAssignPositionDefinitiveClosesOnlyPreviousDefinitive_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)

	oldUserID := fixture.insertUser(t, "USR3", "Old Holder")
	newUserID := fixture.insertUser(t, "USR4", "New Holder")
	pltUserID := fixture.insertUser(t, "USR5", "Acting Holder")
	orgUnitID := fixture.insertOrgUnit(t, "UNIT3", "Unit Three")
	positionID := fixture.insertPosition(t, orgUnitID, "Department Head")
	oldDefinitiveID := fixture.insertUserPosition(t, oldUserID, positionID, "definitive")
	pltID := fixture.insertUserPosition(t, pltUserID, positionID, "plt")

	body := bytes.NewBufferString(`{"user_id":"` + newUserID + `","assignment_type":"definitive"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/positions/"+positionID+"/assign", body)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: positionID}}
	c.Set(middleware.CtxUserID, fixture.actorUserID)

	h.AssignPosition(c)

	if rec.Code != http.StatusCreated {
		t.Fatalf("AssignPosition status = %d, body = %s", rec.Code, rec.Body.String())
	}

	validToByID := map[string]*string{}
	rows, err := fixture.db.Query(context.Background(), `
		SELECT id::text, valid_to::text
		FROM user_positions
		WHERE id = ANY($1::uuid[])`, []string{oldDefinitiveID, pltID})
	if err != nil {
		t.Fatalf("query assignments error: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var validTo *string
		if err := rows.Scan(&id, &validTo); err != nil {
			t.Fatalf("scan assignment error: %v", err)
		}
		validToByID[id] = validTo
	}
	var databaseToday string
	if err := fixture.db.QueryRow(context.Background(), `SELECT current_date::text`).Scan(&databaseToday); err != nil {
		t.Fatalf("query database current date error: %v", err)
	}
	if validToByID[oldDefinitiveID] == nil || *validToByID[oldDefinitiveID] != databaseToday {
		t.Fatalf("old definitive valid_to = %v, want current date", validToByID[oldDefinitiveID])
	}
	if validToByID[pltID] != nil {
		t.Fatalf("plt valid_to = %v, want nil", *validToByID[pltID])
	}
}

func TestCreatePositionValidatesTypeAgainstOrgUnitLevel_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	officeID := fixture.insertOrgUnitWithLevel(t, nil, "CPROOT", "Create Position Root", "office")
	managerID := fixture.insertPositionWithReportsTo(
		t,
		officeID,
		"Create Position President",
		"president_director",
		nil,
		true,
	)

	tests := []struct {
		name         string
		unitLevel    string
		positionType string
		wantStatus   int
	}{
		{
			name:         "director_in_directorate_allowed",
			unitLevel:    "directorate",
			positionType: "director",
			wantStatus:   http.StatusCreated,
		},
		{
			name:         "director_in_department_rejected",
			unitLevel:    "department",
			positionType: "director",
			wantStatus:   http.StatusBadRequest,
		},
		{
			name:         "gm_in_biro_allowed",
			unitLevel:    "biro",
			positionType: "gm",
			wantStatus:   http.StatusCreated,
		},
		{
			name:         "secretary_in_division_rejected",
			unitLevel:    "division",
			positionType: "secretary",
			wantStatus:   http.StatusBadRequest,
		},
		{
			name:         "division_head_in_division_allowed",
			unitLevel:    "division",
			positionType: "division_head",
			wantStatus:   http.StatusCreated,
		},
		{
			name:         "staff_in_department_allowed",
			unitLevel:    "department",
			positionType: "staff",
			wantStatus:   http.StatusCreated,
		},
		{
			name:         "sub_dept_head_in_department_allowed",
			unitLevel:    "department",
			positionType: "sub_dept_head",
			wantStatus:   http.StatusCreated,
		},
		{
			name:         "sub_dept_head_in_division_rejected",
			unitLevel:    "division",
			positionType: "sub_dept_head",
			wantStatus:   http.StatusBadRequest,
		},
		{
			name:         "section_head_in_department_rejected",
			unitLevel:    "department",
			positionType: "section_head",
			wantStatus:   http.StatusBadRequest,
		},
		{
			name:         "staff_in_division_allowed",
			unitLevel:    "division",
			positionType: "staff",
			wantStatus:   http.StatusCreated,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orgUnitID := fixture.insertOrgUnitWithLevel(t, nil, fmt.Sprintf("CP%02d", i), "Create Position "+tt.name, tt.unitLevel)
			body := bytes.NewBufferString(`{
				"org_unit_id":"` + orgUnitID + `",
				"title":"` + tt.name + ` ` + fixture.suffix + `",
				"position_type":"` + tt.positionType + `",
				"reports_to":"` + managerID + `",
				"is_approver":true
			}`)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/positions", body)
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set(middleware.CtxUserID, fixture.actorUserID)

			h.CreatePosition(c)

			if rec.Code != tt.wantStatus {
				t.Fatalf("CreatePosition(%q, %q) status = %d, want %d; body = %s",
					tt.unitLevel, tt.positionType, rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestPositionMasterRejectsDuplicateCycleAndIdentityChange_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)

	officeID := fixture.insertOrgUnitWithLevel(t, nil, "PMROOT", "Position Master Root", "office")
	departmentID := fixture.insertOrgUnitWithLevel(t, &officeID, "PMDEPT", "Position Master Department", "department")
	otherDepartmentID := fixture.insertOrgUnitWithLevel(t, &officeID, "PMDEPT2", "Other Position Master Department", "department")
	presidentID := fixture.insertPositionWithReportsTo(t, officeID, "Position Master President", "president_director", nil, true)
	deptHeadID := fixture.insertPositionWithReportsTo(t, departmentID, "Position Master Department Head", "dept_head", &presidentID, true)
	subDeptHeadID := fixture.insertPositionWithReportsTo(t, departmentID, "Position Master Sub Department Head", "sub_dept_head", &deptHeadID, true)

	t.Run("duplicate_singleton_rejected", func(t *testing.T) {
		body := bytes.NewBufferString(`{
			"org_unit_id":"` + departmentID + `",
			"title":"Duplicate Department Head",
			"position_type":"dept_head",
			"reports_to":"` + presidentID + `",
			"is_approver":true
		}`)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/positions", body)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(middleware.CtxUserID, fixture.actorUserID)

		h.CreatePosition(c)

		if rec.Code != http.StatusConflict {
			t.Errorf("CreatePosition(duplicate dept_head) status = %d, want %d; body = %s",
				rec.Code, http.StatusConflict, rec.Body.String())
		}
	})

	t.Run("reports_to_cycle_rejected", func(t *testing.T) {
		body := bytes.NewBufferString(`{
			"org_unit_id":"` + departmentID + `",
			"title":"Position Master Department Head",
			"position_type":"dept_head",
			"reports_to":"` + subDeptHeadID + `",
			"is_approver":true
		}`)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPut, "/positions/"+deptHeadID, body)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "id", Value: deptHeadID}}
		c.Set(middleware.CtxUserID, fixture.actorUserID)

		h.UpdatePosition(c)

		if rec.Code != http.StatusConflict {
			t.Errorf("UpdatePosition(cycle) status = %d, want %d; body = %s",
				rec.Code, http.StatusConflict, rec.Body.String())
		}
	})

	t.Run("identity_change_after_history_rejected", func(t *testing.T) {
		body := bytes.NewBufferString(`{
			"org_unit_id":"` + otherDepartmentID + `",
			"title":"Moved Department Head",
			"position_type":"dept_head",
			"reports_to":"` + presidentID + `",
			"is_approver":true
		}`)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPut, "/positions/"+deptHeadID, body)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "id", Value: deptHeadID}}
		c.Set(middleware.CtxUserID, fixture.actorUserID)

		h.UpdatePosition(c)

		if rec.Code != http.StatusConflict {
			t.Errorf("UpdatePosition(identity locked) status = %d, want %d; body = %s",
				rec.Code, http.StatusConflict, rec.Body.String())
		}
	})
}

func TestPositionDeactivateBlocksUsageAndSupportsReactivation_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)

	officeID := fixture.insertOrgUnitWithLevel(t, nil, "PDROOT", "Position Deactivation Root", "office")
	departmentID := fixture.insertOrgUnitWithLevel(t, &officeID, "PDDEPT", "Position Deactivation Department", "department")
	presidentID := fixture.insertPositionWithReportsTo(t, officeID, "Position Deactivation President", "president_director", nil, true)
	deptHeadID := fixture.insertPositionWithReportsTo(t, departmentID, "Position Deactivation Head", "dept_head", &presidentID, true)
	staffID := fixture.insertPositionWithReportsTo(t, departmentID, "Position Deactivation Staff", "staff", &deptHeadID, false)

	holderID := fixture.insertUser(t, "PDHOLDER", "Position Deactivation Holder")
	fixture.insertUserPosition(t, holderID, deptHeadID, "definitive")

	t.Run("active_assignment_blocks_deactivation", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodDelete, "/positions/"+deptHeadID, nil)
		c.Params = gin.Params{{Key: "id", Value: deptHeadID}}
		c.Set(middleware.CtxUserID, fixture.actorUserID)

		h.DeactivatePosition(c)

		if rec.Code != http.StatusConflict {
			t.Fatalf("DeactivatePosition(used) status = %d, want %d; body = %s",
				rec.Code, http.StatusConflict, rec.Body.String())
		}
		var got struct {
			Impact positionDeactivationImpact `json:"impact"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("json.Unmarshal(DeactivatePosition response) error: %v", err)
		}
		if got.Impact.ActiveAssignments != 1 || got.Impact.CanDeactivate {
			t.Errorf("DeactivatePosition(used) impact = %+v, want one assignment and blocked", got.Impact)
		}
	})

	t.Run("unused_position_can_deactivate_and_reactivate", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodDelete, "/positions/"+staffID, nil)
		c.Params = gin.Params{{Key: "id", Value: staffID}}
		c.Set(middleware.CtxUserID, fixture.actorUserID)

		h.DeactivatePosition(c)

		if rec.Code != http.StatusOK {
			t.Fatalf("DeactivatePosition(unused) status = %d, want %d; body = %s",
				rec.Code, http.StatusOK, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/positions/"+staffID+"/activate", nil)
		c.Params = gin.Params{{Key: "id", Value: staffID}}
		c.Set(middleware.CtxUserID, fixture.actorUserID)

		h.ActivatePosition(c)

		if rec.Code != http.StatusOK {
			t.Errorf("ActivatePosition(unused) status = %d, want %d; body = %s",
				rec.Code, http.StatusOK, rec.Body.String())
		}
	})
}

func TestCreateUserCreatorRequiresPosition_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)

	body := bytes.NewBufferString(`{
		"nik":"NEW` + fixture.suffix + `",
		"email":"new` + fixture.suffix + `@example.test",
		"full_name":"New Creator",
		"password":"password1234",
		"status":"active",
		"roles":["creator"],
		"positions":[]
	}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/users", body)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(middleware.CtxUserID, fixture.actorUserID)

	h.CreateUser(c)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("CreateUser creator without position status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUserCanReactivateSamePositionEndedToday_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)

	userID := fixture.insertUser(t, "USR6", "Same Day Holder")
	orgUnitID := fixture.insertOrgUnit(t, "UNIT4", "Unit Four")
	positionID := fixture.insertPosition(t, orgUnitID, "Same Day Head")
	assignmentID := fixture.insertUserPosition(t, userID, positionID, "definitive")
	if _, err := fixture.db.Exec(context.Background(), `
		UPDATE user_positions SET valid_to = current_date WHERE id = $1`, assignmentID); err != nil {
		t.Fatalf("end assignment setup error: %v", err)
	}

	body := bytes.NewBufferString(`{
		"nik":"UPD` + fixture.suffix + `",
		"email":"upd` + fixture.suffix + `@example.test",
		"full_name":"Updated Holder",
		"status":"active",
		"roles":["creator"],
		"positions":[{"position_id":"` + positionID + `","assignment_type":"definitive"}]
	}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/users/"+userID, body)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: userID}}
	c.Set(middleware.CtxUserID, fixture.actorUserID)

	h.UpdateUser(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("UpdateUser same-day position status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var validTo *string
	err := fixture.db.QueryRow(context.Background(), `
		SELECT valid_to::text
		FROM user_positions
		WHERE user_id = $1 AND position_id = $2 AND valid_from = current_date`,
		userID, positionID).Scan(&validTo)
	if err != nil {
		t.Fatalf("query same-day assignment error: %v", err)
	}
	if validTo != nil {
		t.Fatalf("valid_to = %v, want nil", *validTo)
	}
}

func TestDeactivateUserRequiresReplacementForImpact_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)

	userID := fixture.insertUser(t, "USR7", "Leaving Holder")
	orgUnitID := fixture.insertOrgUnit(t, "UNIT5", "Unit Five")
	positionID := fixture.insertPosition(t, orgUnitID, "Vacant Head")
	fixture.insertUserPosition(t, userID, positionID, "definitive")

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodDelete, "/users/"+userID, nil)
	c.Params = gin.Params{{Key: "id", Value: userID}}
	c.Set(middleware.CtxUserID, fixture.actorUserID)

	h.DeactivateUser(c)

	if rec.Code != http.StatusConflict {
		t.Fatalf("DeactivateUser without replacement status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}
}

func TestDeactivateUserWithReplacementTransfersDraft_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)

	oldUserID := fixture.insertUser(t, "USR8", "Draft Owner")
	newUserID := fixture.insertUser(t, "USR9", "Draft Replacement")
	orgUnitID := fixture.insertOrgUnit(t, "UNIT6", "Unit Six")
	positionID := fixture.insertPosition(t, orgUnitID, "Draft Head")
	fixture.insertUserPosition(t, oldUserID, positionID, "definitive")
	letterID := fixture.insertDraftLetter(t, oldUserID, positionID)

	body := bytes.NewBufferString(`{
		"position_replacements":[{"position_id":"` + positionID + `","replacement_user_id":"` + newUserID + `","assignment_type":"definitive"}],
		"draft_transfers":[{"letter_id":"` + letterID + `","replacement_user_id":"` + newUserID + `","replacement_position_id":"` + positionID + `"}]
	}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/users/"+oldUserID+"/deactivate", body)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: oldUserID}}
	c.Set(middleware.CtxUserID, fixture.actorUserID)

	h.DeactivateUser(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("DeactivateUser with replacement status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var creatorUserID, creatorPositionID, oldStatus string
	err := fixture.db.QueryRow(context.Background(), `
		SELECT creator_user_id::text, creator_position_id::text, u.status
		FROM letters l
		CROSS JOIN users u
		WHERE l.id = $1 AND u.id = $2`, letterID, oldUserID).Scan(&creatorUserID, &creatorPositionID, &oldStatus)
	if err != nil {
		t.Fatalf("query transferred draft error: %v", err)
	}
	if creatorUserID != newUserID || creatorPositionID != positionID {
		t.Fatalf("draft owner = (%s, %s), want (%s, %s)", creatorUserID, creatorPositionID, newUserID, positionID)
	}
	if oldStatus != "inactive" {
		t.Fatalf("old user status = %q, want inactive", oldStatus)
	}
}

var userPositionFixtureSeq atomic.Int64

type userPositionFixture struct {
	db          *pgxpool.Pool
	actorUserID string
	companyID   string
	cleanupIDs  []string
	suffix      string
}

func newUserPositionFixture(t *testing.T) (*Handler, *userPositionFixture) {
	t.Helper()

	databaseURL := os.Getenv("EOFFICE_INTEGRATION_DB_URL")
	if databaseURL == "" {
		t.Skip("set EOFFICE_INTEGRATION_DB_URL to run Postgres-backed user position tests")
	}

	gin.SetMode(gin.TestMode)

	ctx := context.Background()
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New(%q) error: %v", databaseURL, err)
	}

	// Millisecond komponen + counter atomic: unik dalam satu proses test,
	// menghindari tabrakan kode fixture (mis. companies.code) antar test.
	suffix := fmt.Sprintf("%03d%03d", time.Now().UnixMilli()%1000, userPositionFixtureSeq.Add(1)%1000)
	fixture := &userPositionFixture{db: db, suffix: suffix}
	fixture.companyID = fixture.insertCompany(t)
	fixture.actorUserID = fixture.insertUser(t, "ACTOR", "Position Admin")
	if _, err := db.Exec(ctx, `
		INSERT INTO user_company_roles (user_id, company_id, role_id)
		SELECT $1, $2, id FROM roles WHERE code = 'admin'`,
		fixture.actorUserID, fixture.companyID); err != nil {
		t.Fatalf("grant company admin role error: %v", err)
	}

	t.Cleanup(func() {
		fixture.cleanup(t)
		db.Close()
	})

	return &Handler{DB: db}, fixture
}

func (f *userPositionFixture) insertCompany(t *testing.T) string {
	t.Helper()

	var id string
	err := f.db.QueryRow(context.Background(), `
		INSERT INTO companies (code, name)
		VALUES ($1, $2)
		RETURNING id::text`, "TUP"+f.suffix, "Test User Positions "+f.suffix).Scan(&id)
	if err != nil {
		t.Fatalf("insert company error: %v", err)
	}
	return id
}

func (f *userPositionFixture) insertUser(t *testing.T, nikPrefix string, fullName string) string {
	t.Helper()

	var id string
	nik := nikPrefix + f.suffix
	email := strings.ToLower(nikPrefix) + f.suffix + "@example.test"
	err := f.db.QueryRow(context.Background(), `
		INSERT INTO users (nik, email, full_name, password_hash)
		VALUES ($1, $2, $3, 'test-hash')
		RETURNING id::text`, nik, email, fullName+" "+f.suffix).Scan(&id)
	if err != nil {
		t.Fatalf("insert user %q error: %v", nikPrefix, err)
	}
	f.cleanupIDs = append(f.cleanupIDs, id)
	return id
}

func (f *userPositionFixture) insertOrgUnit(t *testing.T, codePrefix string, name string) string {
	t.Helper()

	return f.insertOrgUnitWithLevel(t, nil, codePrefix, name, "department")
}

func (f *userPositionFixture) insertPosition(t *testing.T, orgUnitID string, title string) string {
	t.Helper()

	var id string
	err := f.db.QueryRow(context.Background(), `
		INSERT INTO positions (org_unit_id, title, position_type)
		VALUES ($1, $2, 'dept_head')
		RETURNING id::text`, orgUnitID, title+" "+f.suffix).Scan(&id)
	if err != nil {
		t.Fatalf("insert position %q error: %v", title, err)
	}
	return id
}

func (f *userPositionFixture) insertUserPosition(t *testing.T, userID string, positionID string, assignmentType string) string {
	t.Helper()

	var id string
	err := f.db.QueryRow(context.Background(), `
		INSERT INTO user_positions (user_id, position_id, assignment_type)
		VALUES ($1, $2, $3)
		RETURNING id::text`, userID, positionID, assignmentType).Scan(&id)
	if err != nil {
		t.Fatalf("insert user position %q error: %v", assignmentType, err)
	}
	return id
}

func (f *userPositionFixture) insertDraftLetter(t *testing.T, userID string, positionID string) string {
	t.Helper()

	var letterTypeID string
	if err := f.db.QueryRow(context.Background(), `SELECT id::text FROM letter_types ORDER BY code LIMIT 1`).Scan(&letterTypeID); err != nil {
		t.Fatalf("query letter type error: %v", err)
	}

	var id string
	err := f.db.QueryRow(context.Background(), `
		INSERT INTO letters (company_id, letter_type_id, subject, creator_user_id, creator_position_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text`,
		f.companyID, letterTypeID, "Draft Transfer "+f.suffix, userID, positionID).Scan(&id)
	if err != nil {
		t.Fatalf("insert draft letter error: %v", err)
	}
	return id
}

func (f *userPositionFixture) cleanup(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	if _, err := f.db.Exec(ctx, `
		DELETE FROM letter_versions
		WHERE letter_id IN (SELECT id FROM letters WHERE creator_user_id = ANY($1::uuid[]))`, f.cleanupIDs); err != nil {
		t.Logf("cleanup letter versions error: %v", err)
	}
	if _, err := f.db.Exec(ctx, `
		DELETE FROM letter_recipients
		WHERE letter_id IN (SELECT id FROM letters WHERE creator_user_id = ANY($1::uuid[]))`, f.cleanupIDs); err != nil {
		t.Logf("cleanup letter recipients error: %v", err)
	}
	if _, err := f.db.Exec(ctx, `
		DELETE FROM disposition_recipients
		WHERE disposition_id IN (
			SELECT disposition.id
			FROM dispositions disposition
			JOIN letters letter ON letter.id = disposition.letter_id
			WHERE letter.creator_user_id = ANY($1::uuid[])
		)`, f.cleanupIDs); err != nil {
		t.Logf("cleanup disposition recipients error: %v", err)
	}
	if _, err := f.db.Exec(ctx, `
		DELETE FROM dispositions
		WHERE letter_id IN (
			SELECT id
			FROM letters
			WHERE creator_user_id = ANY($1::uuid[])
		)`, f.cleanupIDs); err != nil {
		t.Logf("cleanup dispositions error: %v", err)
	}
	if _, err := f.db.Exec(ctx, `
		DELETE FROM approval_actions
		WHERE approval_step_id IN (
			SELECT step.id
			FROM approval_steps step
			JOIN letters letter ON letter.id = step.letter_id
			WHERE letter.creator_user_id = ANY($1::uuid[])
		)`, f.cleanupIDs); err != nil {
		t.Logf("cleanup approval actions error: %v", err)
	}
	if _, err := f.db.Exec(ctx, `
		DELETE FROM approval_steps
		WHERE letter_id IN (SELECT id FROM letters WHERE creator_user_id = ANY($1::uuid[]))`, f.cleanupIDs); err != nil {
		t.Logf("cleanup approval steps error: %v", err)
	}
	if _, err := f.db.Exec(ctx, `DELETE FROM letters WHERE creator_user_id = ANY($1::uuid[])`, f.cleanupIDs); err != nil {
		t.Logf("cleanup letters error: %v", err)
	}
	if _, err := f.db.Exec(ctx, `DELETE FROM audit_logs WHERE actor_user_id = ANY($1::uuid[])`, f.cleanupIDs); err != nil {
		t.Logf("cleanup audit logs error: %v", err)
	}
	if _, err := f.db.Exec(ctx, `
		DELETE FROM user_positions
		WHERE user_id = ANY($1::uuid[])`, f.cleanupIDs); err != nil {
		t.Logf("cleanup user positions error: %v", err)
	}
	if _, err := f.db.Exec(ctx, `
		DELETE FROM positions
		WHERE org_unit_id IN (SELECT id FROM org_units WHERE company_id = $1)`, f.companyID); err != nil {
		t.Logf("cleanup positions error: %v", err)
	}
	if _, err := f.db.Exec(ctx, `DELETE FROM org_units WHERE company_id = $1`, f.companyID); err != nil {
		t.Logf("cleanup org units error: %v", err)
	}
	if _, err := f.db.Exec(ctx, `
		DELETE FROM user_company_roles
		WHERE user_id = ANY($1::uuid[]) OR company_id = $2`, f.cleanupIDs, f.companyID); err != nil {
		t.Logf("cleanup user company roles error: %v", err)
	}
	if _, err := f.db.Exec(ctx, `DELETE FROM user_roles WHERE user_id = ANY($1::uuid[])`, f.cleanupIDs); err != nil {
		t.Logf("cleanup user roles error: %v", err)
	}
	if _, err := f.db.Exec(ctx, `DELETE FROM users WHERE id = ANY($1::uuid[])`, f.cleanupIDs); err != nil {
		t.Logf("cleanup users error: %v", err)
	}
	if _, err := f.db.Exec(ctx, `DELETE FROM companies WHERE id = $1`, f.companyID); err != nil {
		t.Logf("cleanup company error: %v", err)
	}
}
