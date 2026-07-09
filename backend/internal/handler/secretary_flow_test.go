package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/config"
	"github.com/kskgroup/eofficepro/internal/middleware"
)

func TestSecretaryCanSubmitDraftToDirectorApproval_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	h.Cfg = &config.Config{WebBaseURL: "http://localhost:3000"}

	ctx := context.Background()
	directorateID := fixture.insertOrgUnitWithLevel(t, nil, "TSDIR", "Secretary Directorate", "directorate")
	departmentID := fixture.insertOrgUnitWithLevel(t, &directorateID, "TSDEP", "Secretary Department", "department")
	directorUserID := fixture.insertUser(t, "DIRSEC", "Director Holder")
	secretaryUserID := fixture.insertUser(t, "SECUSR", "Secretary Holder")
	directorPositionID := fixture.insertPositionWithReportsTo(t, directorateID, "Director Secretary Directorate", "director", nil, true)
	secretaryPositionID := fixture.insertPositionWithReportsTo(t, directorateID, "Secretary Director Secretary Directorate", "secretary", &directorPositionID, false)
	recipientPositionID := fixture.insertPositionWithReportsTo(t, departmentID, "Department Recipient", "dept_head", &directorPositionID, true)
	fixture.insertUserPosition(t, directorUserID, directorPositionID, "definitive")
	fixture.insertUserPosition(t, secretaryUserID, secretaryPositionID, "definitive")

	letterTypeID := fixture.activeLetterTypeID(t)

	router := gin.New()
	asSecretary := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, secretaryUserID)
		c.Set(middleware.CtxRoles, []string{"secretary"})
		c.Next()
	}
	router.POST(
		"/letters/drafts",
		asSecretary,
		middleware.RequireRole("admin", "creator", "secretary"),
		h.CreateDraftLetter,
	)
	router.POST(
		"/letters/drafts/:id/submit",
		asSecretary,
		middleware.RequireRole("admin", "creator", "secretary"),
		h.SubmitDraftLetter,
	)

	draftBody := bytes.NewBufferString(`{
		"company_id":"` + fixture.companyID + `",
		"letter_type_id":"` + letterTypeID + `",
		"creator_position_id":"` + secretaryPositionID + `",
		"on_behalf_of_position_id":"` + directorPositionID + `",
		"subject":"Secretary approval route",
		"priority":"normal",
		"body_html":"<p>Surat dari Secretary Director.</p>",
		"recipients":[{"type":"to","target_type":"position","target_id":"` + recipientPositionID + `"}]
	}`)
	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/letters/drafts", draftBody)
	createReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("CreateDraftLetter as secretary status = %d, body = %s", createRec.Code, createRec.Body.String())
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create response) error: %v", err)
	}

	tx, err := fixture.db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin route check transaction error: %v", err)
	}
	route, err := h.resolveApprovalRoute(ctx, tx, letterTypeID, secretaryPositionID)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("resolveApprovalRoute for secretary error: %v", err)
	}
	if err := validateApprovalRouteHasActiveHolders(ctx, tx, route); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("validateApprovalRouteHasActiveHolders for secretary error: %v", err)
	}
	for _, step := range route.Steps {
		if _, err := tx.Exec(ctx, `
			INSERT INTO approval_steps
				(letter_id, step_order, approver_position_id, flow_group, status, sla_deadline)
			VALUES ($1, $2, $3, $4, 'pending', now() + make_interval(hours => $5::int))`,
			created.ID, step.StepOrder, step.PositionID, step.FlowGroup, route.SLAHours); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("dry-run insert approval step %+v error: %v", step, err)
		}
	}
	_ = tx.Rollback(ctx)

	submitRec := httptest.NewRecorder()
	submitReq := httptest.NewRequest(http.MethodPost, "/letters/drafts/"+created.ID+"/submit", nil)
	router.ServeHTTP(submitRec, submitReq)

	if submitRec.Code != http.StatusOK {
		t.Fatalf("SubmitDraftLetter as secretary status = %d, body = %s", submitRec.Code, submitRec.Body.String())
	}

	var submitted struct {
		ApprovalSteps []struct {
			StepOrder    int    `json:"step_order"`
			PositionID   string `json:"position_id"`
			PositionType string `json:"position_type"`
			Title        string `json:"title"`
		} `json:"approval_steps"`
	}
	if err := json.Unmarshal(submitRec.Body.Bytes(), &submitted); err != nil {
		t.Fatalf("json.Unmarshal(submit response) error: %v", err)
	}
	if len(submitted.ApprovalSteps) != 1 {
		t.Fatalf("approval steps length = %d, want 1; steps = %+v", len(submitted.ApprovalSteps), submitted.ApprovalSteps)
	}
	firstStep := submitted.ApprovalSteps[0]
	if firstStep.PositionID != directorPositionID || firstStep.PositionType != "director" {
		t.Fatalf("first approval step = (%s, %s), want director position %s", firstStep.PositionID, firstStep.PositionType, directorPositionID)
	}
	if firstStep.PositionType == "secretary" {
		t.Fatalf("secretary must not be an approval step: %+v", submitted.ApprovalSteps)
	}

	var onBehalfOfPositionID string
	if err := fixture.db.QueryRow(ctx, `
		SELECT on_behalf_of_position_id::text
		FROM letters
		WHERE id = $1`, created.ID).Scan(&onBehalfOfPositionID); err != nil {
		t.Fatalf("query on_behalf_of_position_id error: %v", err)
	}
	if onBehalfOfPositionID != directorPositionID {
		t.Fatalf("on_behalf_of_position_id = %s, want %s", onBehalfOfPositionID, directorPositionID)
	}
}

func TestDivisionUnderDepartmentApprovalRouteIncludesSubDepartmentHead_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)

	ctx := context.Background()
	directorateID := fixture.insertOrgUnitWithLevel(t, nil, "TSDIR2", "Department Directorate", "directorate")
	biroID := fixture.insertOrgUnitWithLevel(t, &directorateID, "TSBIR", "Department Biro", "biro")
	departmentID := fixture.insertOrgUnitWithLevel(t, &biroID, "TSDEP2", "Department Route", "department")
	divisionID := fixture.insertOrgUnitWithLevel(t, &departmentID, "TSDIV2", "Department Division", "division")

	directorUserID := fixture.insertUser(t, "TSDIRU2", "Department Director Holder")
	gmUserID := fixture.insertUser(t, "TSGMU", "Department GM Holder")
	deptUserID := fixture.insertUser(t, "TSDEPU", "Department Head Holder")
	subDeptUserID := fixture.insertUser(t, "TSSDEPU", "Sub Department Head Holder")
	directorPositionID := fixture.insertPositionWithReportsTo(t, directorateID, "Director Department Directorate", "director", nil, true)
	gmPositionID := fixture.insertPositionWithReportsTo(t, biroID, "GM Department Biro", "gm", &directorPositionID, true)
	deptPositionID := fixture.insertPositionWithReportsTo(t, departmentID, "Department Head Route", "dept_head", &gmPositionID, true)
	subDeptPositionID := fixture.insertPositionWithReportsTo(t, departmentID, "Sub Department Head Route", "sub_dept_head", &deptPositionID, true)
	divisionPositionID := fixture.insertPositionWithReportsTo(t, divisionID, "Division Head Department Division", "division_head", &subDeptPositionID, true)
	fixture.insertUserPosition(t, directorUserID, directorPositionID, "definitive")
	fixture.insertUserPosition(t, gmUserID, gmPositionID, "definitive")
	fixture.insertUserPosition(t, deptUserID, deptPositionID, "definitive")
	fixture.insertUserPosition(t, subDeptUserID, subDeptPositionID, "definitive")

	tx, err := fixture.db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin route check transaction error: %v", err)
	}
	defer tx.Rollback(ctx)

	route, err := h.resolveApprovalRoute(ctx, tx, fixture.activeLetterTypeID(t), divisionPositionID)
	if err != nil {
		t.Fatalf("resolveApprovalRoute for division under department error: %v", err)
	}
	if err := validateApprovalRouteHasActiveHolders(ctx, tx, route); err != nil {
		t.Fatalf("validateApprovalRouteHasActiveHolders for division under department error: %v", err)
	}
	if len(route.Steps) != 4 {
		t.Fatalf("approval steps length = %d, want 4; steps = %+v", len(route.Steps), route.Steps)
	}

	want := []struct {
		positionID   string
		positionType string
	}{
		{positionID: subDeptPositionID, positionType: "sub_dept_head"},
		{positionID: deptPositionID, positionType: "dept_head"},
		{positionID: gmPositionID, positionType: "gm"},
		{positionID: directorPositionID, positionType: "director"},
	}
	for i, step := range route.Steps {
		if step.PositionID != want[i].positionID || step.PositionType != want[i].positionType {
			t.Errorf("approval step %d = (%s, %s), want (%s, %s)", i+1, step.PositionID, step.PositionType, want[i].positionID, want[i].positionType)
		}
	}
}

func TestDivisionUnderDepartmentApprovalRouteFallsBackToDepartmentGMDirector_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)

	ctx := context.Background()
	directorateID := fixture.insertOrgUnitWithLevel(t, nil, "TBDIR", "Biro Directorate", "directorate")
	biroID := fixture.insertOrgUnitWithLevel(t, &directorateID, "TBBIR", "Biro Finance", "biro")
	departmentID := fixture.insertOrgUnitWithLevel(t, &biroID, "TBDEP", "Biro Department", "department")
	divisionID := fixture.insertOrgUnitWithLevel(t, &departmentID, "TBDIV", "Biro Division", "division")

	directorUserID := fixture.insertUser(t, "TBDIRU", "Biro Director Holder")
	gmUserID := fixture.insertUser(t, "TBGMU", "Biro GM Holder")
	deptUserID := fixture.insertUser(t, "TBDEPU", "Biro Department Holder")
	directorPositionID := fixture.insertPositionWithReportsTo(t, directorateID, "Director Biro Directorate", "director", nil, true)
	gmPositionID := fixture.insertPositionWithReportsTo(t, biroID, "GM Biro Finance", "gm", &directorPositionID, true)
	deptPositionID := fixture.insertPositionWithReportsTo(t, departmentID, "Department Head Biro Department", "dept_head", &gmPositionID, true)
	subDeptPositionID := fixture.insertPositionWithReportsTo(t, departmentID, "Sub Department Head Biro Department", "sub_dept_head", &deptPositionID, true)
	divisionPositionID := fixture.insertPositionWithReportsTo(t, divisionID, "Division Head Biro Division", "division_head", &subDeptPositionID, true)
	fixture.insertUserPosition(t, directorUserID, directorPositionID, "definitive")
	fixture.insertUserPosition(t, gmUserID, gmPositionID, "definitive")
	fixture.insertUserPosition(t, deptUserID, deptPositionID, "definitive")

	tx, err := fixture.db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin route check transaction error: %v", err)
	}
	defer tx.Rollback(ctx)

	route, err := h.resolveApprovalRoute(ctx, tx, fixture.activeLetterTypeID(t), divisionPositionID)
	if err != nil {
		t.Fatalf("resolveApprovalRoute for division under biro error: %v", err)
	}
	if err := validateApprovalRouteHasActiveHolders(ctx, tx, route); err != nil {
		t.Fatalf("validateApprovalRouteHasActiveHolders for division under biro error: %v", err)
	}
	if len(route.Steps) != 3 {
		t.Fatalf("approval steps length = %d, want 3; steps = %+v", len(route.Steps), route.Steps)
	}

	want := []struct {
		positionID   string
		positionType string
	}{
		{positionID: deptPositionID, positionType: "dept_head"},
		{positionID: gmPositionID, positionType: "gm"},
		{positionID: directorPositionID, positionType: "director"},
	}
	for i, step := range route.Steps {
		if step.PositionID != want[i].positionID || step.PositionType != want[i].positionType {
			t.Errorf("approval step %d = (%s, %s), want (%s, %s)", i+1, step.PositionID, step.PositionType, want[i].positionID, want[i].positionType)
		}
	}
}

func TestSecretaryGMCanSubmitDraftToGMApproval_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	h.Cfg = &config.Config{WebBaseURL: "http://localhost:3000"}

	ctx := context.Background()
	directorateID := fixture.insertOrgUnitWithLevel(t, nil, "TGDIR", "Secretary GM Directorate", "directorate")
	biroID := fixture.insertOrgUnitWithLevel(t, &directorateID, "TGBIR", "Secretary GM Biro", "biro")
	departmentID := fixture.insertOrgUnitWithLevel(t, &biroID, "TGDEP", "Secretary GM Department", "department")
	directorUserID := fixture.insertUser(t, "TGDIRU", "Secretary GM Director Holder")
	gmUserID := fixture.insertUser(t, "TGGMU", "Secretary GM Holder")
	secretaryUserID := fixture.insertUser(t, "TGSECU", "Secretary GM User")
	directorPositionID := fixture.insertPositionWithReportsTo(t, directorateID, "Director Secretary GM Directorate", "director", nil, true)
	gmPositionID := fixture.insertPositionWithReportsTo(t, biroID, "GM Secretary GM Biro", "gm", &directorPositionID, true)
	secretaryPositionID := fixture.insertPositionWithReportsTo(t, biroID, "Secretary GM Secretary GM Biro", "secretary", &gmPositionID, false)
	recipientPositionID := fixture.insertPositionWithReportsTo(t, departmentID, "Department Recipient GM Flow", "dept_head", &gmPositionID, true)
	fixture.insertUserPosition(t, directorUserID, directorPositionID, "definitive")
	fixture.insertUserPosition(t, gmUserID, gmPositionID, "definitive")
	fixture.insertUserPosition(t, secretaryUserID, secretaryPositionID, "definitive")

	router := gin.New()
	asSecretary := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, secretaryUserID)
		c.Set(middleware.CtxRoles, []string{"secretary"})
		c.Next()
	}
	router.POST(
		"/letters/drafts",
		asSecretary,
		middleware.RequireRole("admin", "creator", "secretary"),
		h.CreateDraftLetter,
	)
	router.POST(
		"/letters/drafts/:id/submit",
		asSecretary,
		middleware.RequireRole("admin", "creator", "secretary"),
		h.SubmitDraftLetter,
	)

	draftBody := bytes.NewBufferString(`{
		"company_id":"` + fixture.companyID + `",
		"letter_type_id":"` + fixture.activeLetterTypeID(t) + `",
		"creator_position_id":"` + secretaryPositionID + `",
		"on_behalf_of_position_id":"` + gmPositionID + `",
		"subject":"Secretary GM approval route",
		"priority":"normal",
		"body_html":"<p>Surat dari Secretary GM.</p>",
		"recipients":[{"type":"to","target_type":"position","target_id":"` + recipientPositionID + `"}]
	}`)
	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/letters/drafts", draftBody)
	createReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("CreateDraftLetter as secretary gm status = %d, body = %s", createRec.Code, createRec.Body.String())
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create response) error: %v", err)
	}

	submitRec := httptest.NewRecorder()
	submitReq := httptest.NewRequest(http.MethodPost, "/letters/drafts/"+created.ID+"/submit", nil)
	router.ServeHTTP(submitRec, submitReq)

	if submitRec.Code != http.StatusOK {
		t.Fatalf("SubmitDraftLetter as secretary gm status = %d, body = %s", submitRec.Code, submitRec.Body.String())
	}

	var submitted struct {
		ApprovalSteps []struct {
			StepOrder    int    `json:"step_order"`
			PositionID   string `json:"position_id"`
			PositionType string `json:"position_type"`
			Title        string `json:"title"`
		} `json:"approval_steps"`
	}
	if err := json.Unmarshal(submitRec.Body.Bytes(), &submitted); err != nil {
		t.Fatalf("json.Unmarshal(submit response) error: %v", err)
	}
	if len(submitted.ApprovalSteps) < 1 {
		t.Fatalf("approval steps length = %d, want at least 1", len(submitted.ApprovalSteps))
	}
	firstStep := submitted.ApprovalSteps[0]
	if firstStep.PositionID != gmPositionID || firstStep.PositionType != "gm" {
		t.Fatalf("first approval step = (%s, %s), want gm position %s", firstStep.PositionID, firstStep.PositionType, gmPositionID)
	}
	for _, step := range submitted.ApprovalSteps {
		if step.PositionType == "secretary" {
			t.Fatalf("secretary must not be an approval step: %+v", submitted.ApprovalSteps)
		}
	}

	var onBehalfOfPositionID string
	if err := fixture.db.QueryRow(ctx, `
		SELECT on_behalf_of_position_id::text
		FROM letters
		WHERE id = $1`, created.ID).Scan(&onBehalfOfPositionID); err != nil {
		t.Fatalf("query on_behalf_of_position_id error: %v", err)
	}
	if onBehalfOfPositionID != gmPositionID {
		t.Fatalf("on_behalf_of_position_id = %s, want %s", onBehalfOfPositionID, gmPositionID)
	}
}

func (f *userPositionFixture) activeLetterTypeID(t *testing.T) string {
	t.Helper()

	var letterTypeID string
	if err := f.db.QueryRow(context.Background(), `
		SELECT id::text
		FROM letter_types
		WHERE is_active
		ORDER BY code
		LIMIT 1`).Scan(&letterTypeID); err != nil {
		t.Fatalf("query active letter type error: %v", err)
	}
	return letterTypeID
}

func (f *userPositionFixture) insertOrgUnitWithLevel(t *testing.T, parentID *string, codePrefix string, name string, level string) string {
	t.Helper()

	var id string
	err := f.db.QueryRow(context.Background(), `
		INSERT INTO org_units (company_id, parent_id, code, name, unit_level)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text`, f.companyID, parentID, codePrefix+f.suffix, name+" "+f.suffix, level).Scan(&id)
	if err != nil {
		t.Fatalf("insert org unit %q error: %v", codePrefix, err)
	}
	return id
}

func (f *userPositionFixture) insertPositionWithReportsTo(t *testing.T, orgUnitID string, title string, positionType string, reportsTo *string, isApprover bool) string {
	t.Helper()

	var id string
	err := f.db.QueryRow(context.Background(), `
		INSERT INTO positions (org_unit_id, title, position_type, reports_to, is_approver)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text`, orgUnitID, title+" "+f.suffix, positionType, reportsTo, isApprover).Scan(&id)
	if err != nil {
		t.Fatalf("insert position %q error: %v", title, err)
	}
	return id
}
