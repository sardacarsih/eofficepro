package handler

import "testing"

func TestCSVSafValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain text", input: "Surat evaluasi", want: "Surat evaluasi"},
		{name: "formula", input: "=HYPERLINK(\"https://example.com\")", want: "'=HYPERLINK(\"https://example.com\")"},
		{name: "leading whitespace formula", input: "  +cmd", want: "'  +cmd"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := csvSafeValue(test.input); got != test.want {
				t.Fatalf("csvSafeValue(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}
