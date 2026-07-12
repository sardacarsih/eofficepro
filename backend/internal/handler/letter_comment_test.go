package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kskgroup/eofficepro/internal/auth"
	"github.com/kskgroup/eofficepro/internal/middleware"
)

type letterCommentListResponse struct {
	Data []letterCommentItem `json:"data"`
	Meta pageMeta            `json:"meta"`
}

func newLetterCommentLetter(t *testing.T, fixture *userPositionFixture) (creatorUserID, letterID string) {
	t.Helper()

	orgUnitID := fixture.insertOrgUnit(t, "LCUNIT", "Letter Comment Unit")
	creatorUserID = fixture.insertUser(t, "LCAUTHOR", "Comment Author")
	creatorPositionID := fixture.insertPosition(t, orgUnitID, "Comment Position")
	fixture.insertUserPosition(t, creatorUserID, creatorPositionID, "definitive")
	letterID = fixture.insertDraftLetter(t, creatorUserID, creatorPositionID)

	// Komentar harus dihapus sebelum cleanup fixture menghapus surat (FK).
	t.Cleanup(func() {
		_, _ = fixture.db.Exec(context.Background(),
			`DELETE FROM letter_comments WHERE letter_id = $1`, letterID)
	})
	return creatorUserID, letterID
}

func listLetterComments(t *testing.T, h *Handler, userID, letterID, query string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/letters/view/"+letterID+"/comments"+query, nil)
	c.Params = gin.Params{{Key: "id", Value: letterID}}
	c.Set(middleware.CtxUserID, userID)

	h.ListLetterComments(c)
	return rec
}

func createLetterComment(t *testing.T, h *Handler, userID, letterID, body string) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := json.Marshal(gin.H{"body": body})
	if err != nil {
		t.Fatalf("marshal comment payload error: %v", err)
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/letters/view/"+letterID+"/comments", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: letterID}}
	c.Set(middleware.CtxUserID, userID)

	h.CreateLetterComment(c)
	return rec
}

func countLetterComments(t *testing.T, fixture *userPositionFixture, letterID string) int {
	t.Helper()

	var count int
	if err := fixture.db.QueryRow(context.Background(),
		`SELECT count(*) FROM letter_comments WHERE letter_id = $1`, letterID).Scan(&count); err != nil {
		t.Fatalf("count letter comments error: %v", err)
	}
	return count
}

func TestLetterCommentsCreateAndList_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	creatorUserID, letterID := newLetterCommentLetter(t, fixture)

	rec := createLetterComment(t, h, creatorUserID, letterID, "  Mohon dicek lampiran bagian dua.  ")
	if rec.Code != http.StatusCreated {
		t.Fatalf("CreateLetterComment status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}

	var created struct {
		Comment letterCommentItem `json:"comment"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create comment response) error: %v", err)
	}
	if created.Comment.ID == "" {
		t.Fatal("created comment id is empty")
	}
	if created.Comment.UserID != creatorUserID {
		t.Errorf("created comment user_id = %q, want %q", created.Comment.UserID, creatorUserID)
	}
	if created.Comment.Body != "Mohon dicek lampiran bagian dua." {
		t.Errorf("created comment body = %q, want trimmed text", created.Comment.Body)
	}
	if created.Comment.UserName == "" {
		t.Error("created comment user_name is empty")
	}
	wantTitle := "Comment Position " + fixture.suffix
	if created.Comment.PositionTitle == nil || *created.Comment.PositionTitle != wantTitle {
		t.Errorf("created comment position_title = %v, want %q", created.Comment.PositionTitle, wantTitle)
	}

	second := createLetterComment(t, h, creatorUserID, letterID, "Sudah saya perbaiki.")
	if second.Code != http.StatusCreated {
		t.Fatalf("second CreateLetterComment status = %d, body = %s", second.Code, second.Body.String())
	}

	listRec := listLetterComments(t, h, creatorUserID, letterID, "")
	if listRec.Code != http.StatusOK {
		t.Fatalf("ListLetterComments status = %d, body = %s", listRec.Code, listRec.Body.String())
	}
	var got letterCommentListResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(list comments response) error: %v", err)
	}
	if len(got.Data) != 2 {
		t.Fatalf("comments length = %d, want 2; body = %s", len(got.Data), listRec.Body.String())
	}
	if got.Data[0].ID != created.Comment.ID {
		t.Errorf("first comment id = %q, want oldest %q", got.Data[0].ID, created.Comment.ID)
	}
	if got.Data[1].Body != "Sudah saya perbaiki." {
		t.Errorf("second comment body = %q, want newest", got.Data[1].Body)
	}
	if got.Meta.Total != 2 || got.Meta.Page != 1 {
		t.Errorf("list meta = %+v, want total 2 page 1", got.Meta)
	}

	var audited bool
	if err := fixture.db.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1 FROM audit_logs
			WHERE entity_type = 'letter_comment'
			  AND entity_id = $1
			  AND action = 'create'
			  AND actor_user_id = $2
		)`, created.Comment.ID, creatorUserID).Scan(&audited); err != nil {
		t.Fatalf("query comment audit log error: %v", err)
	}
	if !audited {
		t.Error("audit log for letter_comment create not found")
	}
}

func TestLetterCommentsUnauthenticated_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	_, letterID := newLetterCommentLetter(t, fixture)

	router := gin.New()
	requireAuth := middleware.RequireAuth(auth.NewTokenIssuer("test-secret", 1))
	router.GET("/api/v1/letters/view/:id/comments", requireAuth, h.ListLetterComments)
	router.POST("/api/v1/letters/view/:id/comments", requireAuth, h.CreateLetterComment)

	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/api/v1/letters/view/"+letterID+"/comments", nil))
	if getRec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated list status = %d, want 401; body = %s", getRec.Code, getRec.Body.String())
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/v1/letters/view/"+letterID+"/comments",
		bytes.NewBufferString(`{"body":"tanpa token"}`))
	postReq.Header.Set("Content-Type", "application/json")
	postRec := httptest.NewRecorder()
	router.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated create status = %d, want 401; body = %s", postRec.Code, postRec.Body.String())
	}
	if count := countLetterComments(t, fixture, letterID); count != 0 {
		t.Errorf("comments after unauthenticated create = %d, want 0", count)
	}
}

func TestLetterCommentsCrossCompanyUserGets404_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	_, letterID := newLetterCommentLetter(t, fixture)
	ctx := context.Background()

	outsiderUserID := fixture.insertUser(t, "LCOUTCO", "Cross Company Outsider")

	var otherCompanyID string
	if err := fixture.db.QueryRow(ctx, `
		INSERT INTO companies (code, name)
		VALUES ($1, $2)
		RETURNING id::text`, "TLC"+fixture.suffix, "Letter Comment Other "+fixture.suffix).Scan(&otherCompanyID); err != nil {
		t.Fatalf("insert other company error: %v", err)
	}
	var otherOrgUnitID string
	if err := fixture.db.QueryRow(ctx, `
		INSERT INTO org_units (company_id, code, name, unit_level)
		VALUES ($1, $2, $3, 'department')
		RETURNING id::text`, otherCompanyID, "LCX"+fixture.suffix, "Other Comment Unit "+fixture.suffix).Scan(&otherOrgUnitID); err != nil {
		t.Fatalf("insert other org unit error: %v", err)
	}
	var otherPositionID string
	if err := fixture.db.QueryRow(ctx, `
		INSERT INTO positions (org_unit_id, title, position_type)
		VALUES ($1, $2, 'dept_head')
		RETURNING id::text`, otherOrgUnitID, "Other Comment Head "+fixture.suffix).Scan(&otherPositionID); err != nil {
		t.Fatalf("insert other position error: %v", err)
	}
	fixture.insertUserPosition(t, outsiderUserID, otherPositionID, "definitive")
	t.Cleanup(func() {
		_, _ = fixture.db.Exec(ctx, `DELETE FROM user_positions WHERE position_id = $1`, otherPositionID)
		_, _ = fixture.db.Exec(ctx, `DELETE FROM positions WHERE id = $1`, otherPositionID)
		_, _ = fixture.db.Exec(ctx, `DELETE FROM org_units WHERE id = $1`, otherOrgUnitID)
		_, _ = fixture.db.Exec(ctx, `DELETE FROM companies WHERE id = $1`, otherCompanyID)
	})

	if rec := listLetterComments(t, h, outsiderUserID, letterID, ""); rec.Code != http.StatusNotFound {
		t.Errorf("cross-company list status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	if rec := createLetterComment(t, h, outsiderUserID, letterID, "coba komentar lintas company"); rec.Code != http.StatusNotFound {
		t.Errorf("cross-company create status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	if count := countLetterComments(t, fixture, letterID); count != 0 {
		t.Errorf("comments after cross-company create = %d, want 0", count)
	}
}

func TestLetterCommentsUnrelatedUserGets404_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	_, letterID := newLetterCommentLetter(t, fixture)

	unrelatedUserID := fixture.insertUser(t, "LCNOREL", "Unrelated Viewer")

	if rec := listLetterComments(t, h, unrelatedUserID, letterID, ""); rec.Code != http.StatusNotFound {
		t.Errorf("unrelated list status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	if rec := createLetterComment(t, h, unrelatedUserID, letterID, "coba komentar tanpa relasi"); rec.Code != http.StatusNotFound {
		t.Errorf("unrelated create status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	if count := countLetterComments(t, fixture, letterID); count != 0 {
		t.Errorf("comments after unrelated create = %d, want 0", count)
	}
}

func TestLetterCommentsBodyValidation_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	creatorUserID, letterID := newLetterCommentLetter(t, fixture)

	tests := []struct {
		name string
		body string
	}{
		{name: "empty_body", body: ""},
		{name: "whitespace_only_body", body: "   \n\t "},
		{name: "over_2000_chars", body: strings.Repeat("a", 2001)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := createLetterComment(t, h, creatorUserID, letterID, tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("CreateLetterComment(%s) status = %d, want 400; body = %s", tt.name, rec.Code, rec.Body.String())
			}
		})
	}
	if count := countLetterComments(t, fixture, letterID); count != 0 {
		t.Errorf("comments after invalid bodies = %d, want 0", count)
	}

	rec := createLetterComment(t, h, creatorUserID, letterID, strings.Repeat("b", 2000))
	if rec.Code != http.StatusCreated {
		t.Errorf("CreateLetterComment(2000 chars) status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}
}

func TestLetterCommentsPagination_Integration(t *testing.T) {
	h, fixture := newUserPositionFixture(t)
	creatorUserID, letterID := newLetterCommentLetter(t, fixture)
	ctx := context.Background()

	base := time.Now().Add(-time.Hour)
	commentIDs := make([]string, 0, 3)
	for i, body := range []string{"komentar pertama", "komentar kedua", "komentar ketiga"} {
		var id string
		if err := fixture.db.QueryRow(ctx, `
			INSERT INTO letter_comments (letter_id, user_id, body, created_at)
			VALUES ($1, $2, $3, $4)
			RETURNING id::text`,
			letterID, creatorUserID, body, base.Add(time.Duration(i)*time.Minute)).Scan(&id); err != nil {
			t.Fatalf("insert comment %d error: %v", i, err)
		}
		commentIDs = append(commentIDs, id)
	}

	rec := listLetterComments(t, h, creatorUserID, letterID, "?page=1&page_size=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("ListLetterComments page 1 status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var pageOne letterCommentListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &pageOne); err != nil {
		t.Fatalf("json.Unmarshal(page 1) error: %v", err)
	}
	if len(pageOne.Data) != 2 || pageOne.Data[0].ID != commentIDs[0] || pageOne.Data[1].ID != commentIDs[1] {
		t.Errorf("page 1 data = %+v, want first two comments in order", pageOne.Data)
	}
	if pageOne.Meta.Total != 3 || pageOne.Meta.TotalPages != 2 || pageOne.Meta.PageSize != 2 {
		t.Errorf("page 1 meta = %+v, want total 3 total_pages 2 page_size 2", pageOne.Meta)
	}

	rec = listLetterComments(t, h, creatorUserID, letterID, "?page=2&page_size=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("ListLetterComments page 2 status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var pageTwo letterCommentListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &pageTwo); err != nil {
		t.Fatalf("json.Unmarshal(page 2) error: %v", err)
	}
	if len(pageTwo.Data) != 1 || pageTwo.Data[0].ID != commentIDs[2] {
		t.Errorf("page 2 data = %+v, want only newest comment", pageTwo.Data)
	}

	if rec := listLetterComments(t, h, creatorUserID, letterID, "?page=0"); rec.Code != http.StatusBadRequest {
		t.Errorf("ListLetterComments invalid page status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
	if rec := listLetterComments(t, h, creatorUserID, letterID, "?page_size=abc"); rec.Code != http.StatusBadRequest {
		t.Errorf("ListLetterComments invalid page_size status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
}
