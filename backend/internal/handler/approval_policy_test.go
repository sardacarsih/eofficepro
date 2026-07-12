package handler

import "testing"

func TestApprovalLevelRankIsStrictlyOrdered(t *testing.T) {
	levels := []string{"division_head", "sub_dept_head", "dept_head", "gm", "director", "vp_director", "president_director"}
	for i := 1; i < len(levels); i++ {
		if approvalLevelRank[levels[i-1]] >= approvalLevelRank[levels[i]] {
			t.Errorf("approvalLevelRank[%q] = %d, want lower than %q (%d)", levels[i-1], approvalLevelRank[levels[i-1]], levels[i], approvalLevelRank[levels[i]])
		}
	}
}

func TestContainsString(t *testing.T) {
	if !containsString([]string{"gm", "director"}, "director") {
		t.Error("containsString([gm director], director) = false, want true")
	}
	if containsString([]string{"gm", "director"}, "vp_director") {
		t.Error("containsString([gm director], vp_director) = true, want false")
	}
}

func TestSameUnitFinalLevel(t *testing.T) {
	tests := []struct{ positionType, unitLevel, want string }{
		{"staff", "division", "division_head"},
		{"assistant", "division", "division_head"},
		{"division_head", "division", "dept_head"},
		{"staff", "department", "dept_head"},
	}
	for _, tt := range tests {
		if got := sameUnitFinalLevel(tt.positionType, tt.unitLevel, "dept_head"); got != tt.want {
			t.Errorf("sameUnitFinalLevel(%q, %q, dept_head) = %q, want %q", tt.positionType, tt.unitLevel, got, tt.want)
		}
	}
}

func TestPromoteCorporateScope(t *testing.T) {
	if got := promoteCorporateScope(3, 10, 10); got != 4 {
		t.Errorf("promoteCorporateScope(3, 10, 10) = %d, want 4", got)
	}
	if got := promoteCorporateScope(3, 2, 10); got != 3 {
		t.Errorf("promoteCorporateScope(3, 2, 10) = %d, want 3", got)
	}
}
