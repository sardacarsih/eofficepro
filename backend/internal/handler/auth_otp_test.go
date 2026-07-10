package handler

import "testing"

func TestGenerateOTP(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		code, err := generateOTP()
		if err != nil {
			t.Fatalf("generateOTP error: %v", err)
		}
		if len(code) != 6 {
			t.Fatalf("panjang kode = %d, harus 6 (%q)", len(code), code)
		}
		for _, r := range code {
			if r < '0' || r > '9' {
				t.Fatalf("kode mengandung non-digit: %q", code)
			}
		}
		seen[code] = true
	}
	if len(seen) == 1 {
		t.Fatal("50 kode identik semua — generator tidak acak")
	}
}

func TestOTPMatches(t *testing.T) {
	hash := hashOTP("123456")
	if !otpMatches(hash, "123456") {
		t.Error("kode benar harus cocok")
	}
	if otpMatches(hash, "654321") {
		t.Error("kode salah tidak boleh cocok")
	}
	if otpMatches("", "123456") {
		t.Error("hash kosong tidak boleh cocok")
	}
}
