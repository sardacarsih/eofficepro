package handler

import (
	"testing"
	"time"

	"github.com/kskgroup/eofficepro/internal/config"
)

func TestRefreshValueRoundTrip(t *testing.T) {
	cases := []struct {
		value        string
		wantUserID   string
		wantRemember bool
	}{
		{refreshValue("u1", true), "u1", true},
		{refreshValue("u1", false), "u1", false},
		// Value lama dari sebelum fitur "ingat saya" (userID polos).
		{"u1", "u1", false},
	}
	for _, tc := range cases {
		userID, remember := parseRefreshValue(tc.value)
		if userID != tc.wantUserID || remember != tc.wantRemember {
			t.Errorf("parseRefreshValue(%q) = (%q, %v), want (%q, %v)",
				tc.value, userID, remember, tc.wantUserID, tc.wantRemember)
		}
	}
}

func TestRefreshTTL(t *testing.T) {
	h := &Handler{Cfg: &config.Config{
		JWTRefreshTTLHours:         24,
		JWTRefreshRememberTTLHours: 720,
	}}
	if got := h.refreshTTL(false); got != 24*time.Hour {
		t.Errorf("refreshTTL(false) = %v, want 24h", got)
	}
	if got := h.refreshTTL(true); got != 720*time.Hour {
		t.Errorf("refreshTTL(true) = %v, want 720h", got)
	}
}
