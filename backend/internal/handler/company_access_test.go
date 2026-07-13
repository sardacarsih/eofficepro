package handler

import "testing"

func TestNormalizeUserCompanyRoles(t *testing.T) {
	t.Parallel()

	assignments, err := normalizeUserCompanyRoles([]userCompanyRolePayload{{
		CompanyID: " company-id ",
		RoleCode:  "",
	}})
	if err != nil {
		t.Fatalf("normalizeUserCompanyRoles() error = %v", err)
	}
	if len(assignments) != 1 || assignments[0].CompanyID != "company-id" || assignments[0].RoleCode != "admin" {
		t.Fatalf("normalizeUserCompanyRoles() = %#v", assignments)
	}
	if assignments[0].ValidFrom == "" {
		t.Fatal("normalizeUserCompanyRoles() valid_from is empty, want current date default")
	}
}

func TestNormalizeUserCompanyRolesRejectsInvalidAndDuplicateAssignments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []userCompanyRolePayload
	}{
		{name: "missing company", input: []userCompanyRolePayload{{RoleCode: "admin"}}},
		{name: "unknown role", input: []userCompanyRolePayload{{CompanyID: "company-id", RoleCode: "super_admin"}}},
		{name: "invalid start date", input: []userCompanyRolePayload{{CompanyID: "company-id", ValidFrom: "12/07/2026"}}},
		{name: "end before start", input: []userCompanyRolePayload{{CompanyID: "company-id", ValidFrom: "2026-07-12", ValidTo: stringPointer("2026-07-11")}}},
		{name: "same day exclusive end", input: []userCompanyRolePayload{{CompanyID: "company-id", ValidFrom: "2026-07-12", ValidTo: stringPointer("2026-07-12")}}},
		{name: "duplicate", input: []userCompanyRolePayload{{CompanyID: "company-id", RoleCode: "admin"}, {CompanyID: " company-id ", RoleCode: "ADMIN"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := normalizeUserCompanyRoles(tt.input); err == nil {
				t.Fatal("normalizeUserCompanyRoles() error = nil, want error")
			}
		})
	}
}

func stringPointer(value string) *string {
	return &value
}
