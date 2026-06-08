package ui

import (
	"testing"

	"github.com/jeffreyd/cmdr-keen/internal/session"
)

func TestStatusForEvent(t *testing.T) {
	tests := []struct {
		event  string
		want   session.Status
		wantOK bool
	}{
		{"crunching", session.StatusCrunching, true},
		{"waiting", session.StatusWaiting, true},
		{"done", session.StatusDone, true},
		{"exit", session.StatusExited, true},
		// "start" is intentionally unmapped: a freshly launched session stays
		// neutral until the first prompt makes it crunch.
		{"start", 0, false},
		{"bogus", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			got, ok := statusForEvent(tt.event)
			if ok != tt.wantOK {
				t.Fatalf("statusForEvent(%q) ok = %v, want %v", tt.event, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("statusForEvent(%q) = %v, want %v", tt.event, got, tt.want)
			}
		})
	}
}
