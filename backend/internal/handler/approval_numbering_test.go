package handler

import (
	"testing"
	"time"
)

func TestRenderLetterNumber(t *testing.T) {
	numbering := numberingContext{
		CompanyCode: "KSK",
		LetterType:  "ND",
		OrgUnitCode: "HRGA",
		CreatedAt:   time.Date(2026, time.July, 8, 10, 0, 0, 0, time.UTC),
	}

	got := renderLetterNumber("{seq:4}/{type}/{unit}/{roman_month}/{year}", numbering, 12)
	want := "0012/ND/HRGA/VII/2026"
	if got != want {
		t.Fatalf("renderLetterNumber() = %q, want %q", got, want)
	}
}

func TestNumberingScopeKey(t *testing.T) {
	numbering := numberingContext{
		CompanyCode: "KSK",
		LetterType:  "ND",
		OrgUnitCode: "HRGA",
		CreatedAt:   time.Date(2026, time.July, 8, 10, 0, 0, 0, time.UTC),
	}

	if got, want := numberingScopeKey(numbering, "yearly"), "KSK|HRGA|ND|2026"; got != want {
		t.Fatalf("yearly numberingScopeKey() = %q, want %q", got, want)
	}
	if got, want := numberingScopeKey(numbering, "monthly"), "KSK|HRGA|ND|2026-07"; got != want {
		t.Fatalf("monthly numberingScopeKey() = %q, want %q", got, want)
	}
}
