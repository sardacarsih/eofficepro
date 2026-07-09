package handler

import (
	"strings"
	"testing"
)

func TestNormalizeAndValidateCompanyRequest(t *testing.T) {
	req := companyRequest{
		Code: "  ksk  ",
		Name: "  PT Kalimantan Sawit Kusuma  ",
	}
	if err := normalizeAndValidateCompanyRequest(&req); err != nil {
		t.Fatalf("normalizeAndValidateCompanyRequest() error = %v", err)
	}
	if req.Code != "KSK" {
		t.Errorf("normalizeAndValidateCompanyRequest().Code = %q, want KSK", req.Code)
	}
	if req.Name != "PT Kalimantan Sawit Kusuma" {
		t.Errorf(
			"normalizeAndValidateCompanyRequest().Name = %q, want PT Kalimantan Sawit Kusuma",
			req.Name,
		)
	}
}

func TestNormalizeAndValidateCompanyRequestRejectsInvalidData(t *testing.T) {
	tests := []struct {
		name string
		req  companyRequest
	}{
		{name: "empty_code", req: companyRequest{Name: "Company"}},
		{name: "code_too_long", req: companyRequest{Code: "COMPANYCODE1", Name: "Company"}},
		{name: "empty_name", req: companyRequest{Code: "CMP"}},
		{name: "name_too_long", req: companyRequest{Code: "CMP", Name: strings.Repeat("a", 151)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := normalizeAndValidateCompanyRequest(&tt.req); err == nil {
				t.Errorf("normalizeAndValidateCompanyRequest(%s) error = nil, want error", tt.name)
			}
		})
	}
}
