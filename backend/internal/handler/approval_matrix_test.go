package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

func TestApprovalMatrixCRUD_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	letterTypeID := fixture.insertLetterType(t, "AM")

	createBody := bytes.NewBufferString(`{
		"letter_type_id":"` + letterTypeID + `",
		"originator_level":null,
		"final_level":"president_director",
		"flow_mode":"serial",
		"is_active":true
	}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/approval-matrices", createBody)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(middleware.CtxUserID, fixture.actorUserID)

	h.CreateApprovalMatrix(c)

	if rec.Code != http.StatusCreated {
		t.Fatalf("CreateApprovalMatrix(default) status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create default response) error: %v", err)
	}

	t.Run("list includes letter type labels", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/approval-matrices?include_inactive=true&page_size=100", nil)

		h.ListApprovalMatrices(c)

		if rec.Code != http.StatusOK {
			t.Fatalf("ListApprovalMatrices status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var got struct {
			Matrices []ApprovalMatrix `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("json.Unmarshal(list response) error: %v", err)
		}
		for _, matrix := range got.Matrices {
			if matrix.ID == created.ID {
				if matrix.LetterTypeCode == "" || matrix.LetterTypeName == "" {
					t.Fatalf("listed matrix missing letter type label: %+v", matrix)
				}
				return
			}
		}
		t.Fatalf("created matrix %s not found in list: %+v", created.ID, got.Matrices)
	})

	t.Run("duplicate active default is rejected", func(t *testing.T) {
		body := bytes.NewBufferString(`{
			"letter_type_id":"` + letterTypeID + `",
			"originator_level":null,
			"final_level":"director",
			"flow_mode":"serial",
			"is_active":true
		}`)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/approval-matrices", body)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(middleware.CtxUserID, fixture.actorUserID)

		h.CreateApprovalMatrix(c)

		if rec.Code != http.StatusConflict {
			t.Fatalf("CreateApprovalMatrix(duplicate default) status = %d, want 409; body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("originator specific can coexist with default", func(t *testing.T) {
		body := bytes.NewBufferString(`{
			"letter_type_id":"` + letterTypeID + `",
			"originator_level":"secretary",
			"final_level":"director",
			"flow_mode":"serial",
			"is_active":true
		}`)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/approval-matrices", body)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(middleware.CtxUserID, fixture.actorUserID)

		h.CreateApprovalMatrix(c)

		if rec.Code != http.StatusCreated {
			t.Fatalf("CreateApprovalMatrix(secretary) status = %d, want 201; body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid values are rejected", func(t *testing.T) {
		tests := []struct {
			name string
			body string
		}{
			{
				name: "invalid_originator",
				body: `"originator_level":"section_head","final_level":"director","flow_mode":"serial"`,
			},
			{
				name: "invalid_final",
				body: `"originator_level":null,"final_level":"secretary","flow_mode":"serial"`,
			},
			{
				name: "parallel_rejected",
				body: `"originator_level":null,"final_level":"director","flow_mode":"parallel"`,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				body := bytes.NewBufferString(`{"letter_type_id":"` + letterTypeID + `",` + tt.body + `}`)
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/approval-matrices", body)
				c.Request.Header.Set("Content-Type", "application/json")
				c.Set(middleware.CtxUserID, fixture.actorUserID)

				h.CreateApprovalMatrix(c)

				if rec.Code != http.StatusBadRequest {
					t.Fatalf("CreateApprovalMatrix(%s) status = %d, want 400; body = %s", tt.name, rec.Code, rec.Body.String())
				}
			})
		}
	})

	t.Run("delete marks inactive", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodDelete, "/approval-matrices/"+created.ID, nil)
		c.Params = gin.Params{{Key: "id", Value: created.ID}}
		c.Set(middleware.CtxUserID, fixture.actorUserID)

		h.DeactivateApprovalMatrix(c)

		if rec.Code != http.StatusOK {
			t.Fatalf("DeactivateApprovalMatrix status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var isActive bool
		if err := fixture.db.QueryRow(context.Background(), `SELECT is_active FROM approval_matrices WHERE id = $1`, created.ID).Scan(&isActive); err != nil {
			t.Fatalf("query deactivated matrix error: %v", err)
		}
		if isActive {
			t.Fatalf("is_active = true, want false")
		}
	})
}

func TestApprovalRouteUsesApprovalMatrixFinalLevel_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)

	ctx := context.Background()
	officeID := fixture.insertOrgUnitWithLevel(t, nil, "AMROOT", "Approval Matrix Office", "office")
	directorateID := fixture.insertOrgUnitWithLevel(t, &officeID, "AMDIR", "Approval Matrix Directorate", "directorate")
	departmentID := fixture.insertOrgUnitWithLevel(t, &directorateID, "AMDEP", "Approval Matrix Department", "department")

	presidentUserID := fixture.insertUser(t, "AMPRES", "Approval Matrix President Holder")
	directorUserID := fixture.insertUser(t, "AMDIRU", "Approval Matrix Director Holder")
	secretaryUserID := fixture.insertUser(t, "AMSECU", "Approval Matrix Secretary Holder")
	presidentPositionID := fixture.insertPositionWithReportsTo(t, officeID, "Approval Matrix President", "president_director", nil, true)
	directorPositionID := fixture.insertPositionWithReportsTo(t, directorateID, "Approval Matrix Director", "director", &presidentPositionID, true)
	secretaryPositionID := fixture.insertPositionWithReportsTo(t, directorateID, "Approval Matrix Secretary", "secretary", &directorPositionID, false)
	recipientPositionID := fixture.insertPositionWithReportsTo(t, departmentID, "Approval Matrix Recipient", "dept_head", &directorPositionID, true)
	fixture.insertUserPosition(t, presidentUserID, presidentPositionID, "definitive")
	fixture.insertUserPosition(t, directorUserID, directorPositionID, "definitive")
	fixture.insertUserPosition(t, secretaryUserID, secretaryPositionID, "definitive")

	letterTypeID := fixture.insertLetterType(t, "AR")
	if _, err := fixture.db.Exec(ctx, `
		INSERT INTO approval_matrices (letter_type_id, originator_level, final_level, flow_mode, is_active)
		VALUES ($1, 'secretary', 'president_director', 'serial', true)`, letterTypeID); err != nil {
		t.Fatalf("insert approval matrix error: %v", err)
	}

	tx, err := fixture.db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin route check transaction error: %v", err)
	}
	defer tx.Rollback(ctx)

	route, err := h.resolveApprovalRoute(ctx, tx, letterTypeID, secretaryPositionID)
	if err != nil {
		t.Fatalf("resolveApprovalRoute with matrix error: %v", err)
	}
	if err := validateApprovalRouteHasActiveHolders(ctx, tx, route); err != nil {
		t.Fatalf("validateApprovalRouteHasActiveHolders with matrix error: %v", err)
	}
	if len(route.Steps) != 2 {
		t.Fatalf("approval steps length = %d, want 2; steps = %+v; recipient = %s", len(route.Steps), route.Steps, recipientPositionID)
	}
	if route.Steps[0].PositionID != directorPositionID || route.Steps[0].PositionType != "director" {
		t.Fatalf("first step = (%s, %s), want director %s", route.Steps[0].PositionID, route.Steps[0].PositionType, directorPositionID)
	}
	if route.Steps[1].PositionID != presidentPositionID || route.Steps[1].PositionType != "president_director" {
		t.Fatalf("second step = (%s, %s), want president %s", route.Steps[1].PositionID, route.Steps[1].PositionType, presidentPositionID)
	}
}

func (f *userPositionFixture) insertLetterType(t *testing.T, codePrefix string) string {
	t.Helper()

	code := strings.ToUpper(codePrefix) + f.suffix[:3]
	var id string
	err := f.db.QueryRow(context.Background(), `
		INSERT INTO letter_types (code, name, default_classification, default_sla_hours)
		VALUES ($1, $2, 'biasa', 24)
		RETURNING id::text`, code, "Test Matrix "+f.suffix).Scan(&id)
	if err != nil {
		t.Fatalf("insert letter type %q error: %v", code, err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := f.db.Exec(ctx, `DELETE FROM approval_matrices WHERE letter_type_id = $1`, id); err != nil {
			t.Logf("cleanup approval matrices error: %v", err)
		}
		if _, err := f.db.Exec(ctx, `DELETE FROM letter_types WHERE id = $1`, id); err != nil {
			t.Logf("cleanup letter type error: %v", err)
		}
	})
	return id
}
