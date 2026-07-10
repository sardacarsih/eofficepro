package handler

import "testing"

func TestBuildPushMessageMapsTargetSection(t *testing.T) {
	tests := []struct {
		name           string
		eventType      string
		wantSection    string
		inputTitle     string
		wantTitle      string
		classification string
	}{
		{
			name:        "approval_waiting",
			eventType:   "approval_waiting",
			wantSection: "approvals",
			inputTitle:  "Menunggu approval: Surat A",
			wantTitle:   "Menunggu approval: Surat A",
		},
		{
			name:        "letter_incoming",
			eventType:   "letter_incoming",
			wantSection: "inbox",
			inputTitle:  "Surat masuk: Surat A",
			wantTitle:   "Surat masuk: Surat A",
		},
		{
			name:        "disposition_assigned",
			eventType:   "disposition_assigned",
			wantSection: "dispositions",
			inputTitle:  "Disposisi baru: Surat A",
			wantTitle:   "Disposisi baru: Surat A",
		},
		{
			name:           "classified_secret",
			eventType:      "letter_incoming",
			wantSection:    "inbox",
			inputTitle:     "Surat masuk: Akuisisi Rahasia",
			wantTitle:      "Notifikasi eOffice Pro",
			classification: "rahasia",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPushMessage(notificationEmail{
				EventType: tt.eventType,
				LetterID:  "letter-1",
				Title:     tt.inputTitle,
				Body:      "Body notifikasi surat",
			}, tt.classification)

			if got.TargetSection != tt.wantSection {
				t.Fatalf("TargetSection = %q, want %q", got.TargetSection, tt.wantSection)
			}
			if got.Title != tt.wantTitle {
				t.Fatalf("Title = %q, want %q", got.Title, tt.wantTitle)
			}
			if tt.classification == "rahasia" && got.Body == "Body notifikasi surat" {
				t.Fatalf("secret notification leaked original body")
			}
		})
	}
}

func TestTruncatePushBody(t *testing.T) {
	got := truncatePushBody("abcdefghijklmnopqrstuvwxyz")
	if got != "abcdefghijklmnopqrstuvwxyz" {
		t.Fatalf("short body changed to %q", got)
	}

	longBody := ""
	for i := 0; i < 200; i++ {
		longBody += "x"
	}
	got = truncatePushBody(longBody)
	if len(got) != 180 {
		t.Fatalf("truncated body length = %d, want 180", len(got))
	}
	if got[177:] != "..." {
		t.Fatalf("truncated body suffix = %q, want ...", got[177:])
	}
}
