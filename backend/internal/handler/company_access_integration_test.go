package handler

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCompanyAccessTenantIsolation(t *testing.T) {
	databaseURL := os.Getenv("EOFFICE_INTEGRATION_DB_URL")
	if databaseURL == "" {
		t.Skip("set EOFFICE_INTEGRATION_DB_URL to run Postgres-backed company access tests")
	}

	fixture := newCompanyAccessFixture(t, databaseURL)
	h := &Handler{DB: fixture.db}
	ctx := context.Background()

	t.Run("company_admin_is_limited_to_assigned_company", func(t *testing.T) {
		allowed, err := h.canAdminCompany(ctx, fixture.companyAdminID, fixture.assignedCompanyID)
		if err != nil {
			t.Fatalf("canAdminCompany(company admin, assigned company) error: %v", err)
		}
		if !allowed {
			t.Error("canAdminCompany(company admin, assigned company) = false, want true")
		}

		allowed, err = h.canAdminCompany(ctx, fixture.companyAdminID, fixture.otherCompanyID)
		if err != nil {
			t.Fatalf("canAdminCompany(company admin, other company) error: %v", err)
		}
		if allowed {
			t.Error("canAdminCompany(company admin, other company) = true, want false")
		}

		companies, err := h.accessibleCompanies(ctx, fixture.companyAdminID, false)
		if err != nil {
			t.Fatalf("accessibleCompanies(company admin) error: %v", err)
		}
		if got, want := companyIDs(companies), []string{fixture.assignedCompanyID}; !sameStringSet(got, want) {
			t.Errorf("accessibleCompanies(company admin) IDs = %v, want %v", got, want)
		}
	})

	t.Run("super_admin_has_global_access", func(t *testing.T) {
		for _, companyID := range []string{fixture.assignedCompanyID, fixture.otherCompanyID} {
			allowed, err := h.canAdminCompany(ctx, fixture.superAdminID, companyID)
			if err != nil {
				t.Fatalf("canAdminCompany(super admin, %q) error: %v", companyID, err)
			}
			if !allowed {
				t.Errorf("canAdminCompany(super admin, %q) = false, want true", companyID)
			}
		}
	})

	t.Run("expired_company_role_is_denied", func(t *testing.T) {
		if _, err := fixture.db.Exec(ctx, `
			UPDATE user_company_roles
			SET valid_to = current_date
			WHERE user_id = $1 AND company_id = $2`,
			fixture.companyAdminID, fixture.assignedCompanyID); err != nil {
			t.Fatalf("expire company role error: %v", err)
		}

		allowed, err := h.canAdminCompany(ctx, fixture.companyAdminID, fixture.assignedCompanyID)
		if err != nil {
			t.Fatalf("canAdminCompany(expired company admin, assigned company) error: %v", err)
		}
		if allowed {
			t.Error("canAdminCompany(expired company admin, assigned company) = true, want false")
		}
	})
}

type companyAccessFixture struct {
	db                *pgxpool.Pool
	companyAdminID    string
	superAdminID      string
	assignedCompanyID string
	otherCompanyID    string
}

func newCompanyAccessFixture(t *testing.T, databaseURL string) *companyAccessFixture {
	t.Helper()

	ctx := context.Background()
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New(%q) error: %v", databaseURL, err)
	}

	suffix := fmt.Sprintf("%07d", time.Now().UnixNano()%10000000)
	fixture := &companyAccessFixture{db: db}
	t.Cleanup(func() {
		fixture.cleanup(t)
		db.Close()
	})
	fixture.assignedCompanyID = fixture.insertCompany(t, "TCA"+suffix, "Tenant Access "+suffix)
	fixture.otherCompanyID = fixture.insertCompany(t, "TCB"+suffix, "Tenant Other "+suffix)
	fixture.companyAdminID = fixture.insertUser(t, "company-admin-"+suffix)
	fixture.superAdminID = fixture.insertUser(t, "super-admin-"+suffix)

	if _, err := db.Exec(ctx, `
		INSERT INTO user_company_roles (user_id, company_id, role_id)
		SELECT $1, $2, id FROM roles WHERE code = 'admin'`,
		fixture.companyAdminID, fixture.assignedCompanyID); err != nil {
		t.Fatalf("insert company admin role error: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE code = 'super_admin'`, fixture.superAdminID); err != nil {
		t.Fatalf("insert super admin role error: %v", err)
	}

	return fixture
}

func (f *companyAccessFixture) insertCompany(t *testing.T, code string, name string) string {
	t.Helper()

	var id string
	if err := f.db.QueryRow(context.Background(), `
		INSERT INTO companies (code, name)
		VALUES ($1, $2)
		RETURNING id::text`, code, name).Scan(&id); err != nil {
		t.Fatalf("insert company %q error: %v", code, err)
	}
	return id
}

func (f *companyAccessFixture) insertUser(t *testing.T, prefix string) string {
	t.Helper()

	var id string
	if err := f.db.QueryRow(context.Background(), `
		INSERT INTO users (nik, email, full_name, password_hash)
		VALUES ($1, $2, $3, 'test-hash')
		RETURNING id::text`, prefix, prefix+"@example.test", prefix).Scan(&id); err != nil {
		t.Fatalf("insert user %q error: %v", prefix, err)
	}
	return id
}

func (f *companyAccessFixture) cleanup(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	userIDs := []string{f.companyAdminID, f.superAdminID}
	companyIDs := []string{f.assignedCompanyID, f.otherCompanyID}
	for _, statement := range []string{
		`DELETE FROM user_company_roles WHERE user_id = ANY($1::uuid[])`,
		`DELETE FROM user_roles WHERE user_id = ANY($1::uuid[])`,
		`DELETE FROM users WHERE id = ANY($1::uuid[])`,
	} {
		if _, err := f.db.Exec(ctx, statement, userIDs); err != nil {
			t.Errorf("company access cleanup users with %q error: %v", statement, err)
		}
	}
	if _, err := f.db.Exec(ctx, `DELETE FROM companies WHERE id = ANY($1::uuid[])`, companyIDs); err != nil {
		t.Errorf("company access cleanup companies error: %v", err)
	}
}

func companyIDs(companies []accessibleCompany) []string {
	ids := make([]string, 0, len(companies))
	for _, company := range companies {
		ids = append(ids, company.ID)
	}
	return ids
}

func sameStringSet(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	values := make(map[string]int, len(got))
	for _, value := range got {
		values[value]++
	}
	for _, value := range want {
		values[value]--
		if values[value] < 0 {
			return false
		}
	}
	return true
}
