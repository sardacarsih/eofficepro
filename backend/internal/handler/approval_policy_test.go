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
