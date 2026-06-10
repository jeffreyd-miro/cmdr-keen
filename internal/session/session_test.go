package session

import (
	"testing"
	"time"
)

func TestFormatElapsed(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "1m"},                       // sub-minute rounds up so an active row is never blank
		{30 * time.Second, "1m"},        // still under a minute
		{time.Minute, "1m"},             // exactly a minute
		{90 * time.Second, "1m"},        // truncated, not rounded
		{5 * time.Minute, "5m"},         // whole minutes
		{59 * time.Minute, "59m"},       // last minute before flipping to hours
		{time.Hour, "1h"},               // the hour boundary
		{90 * time.Minute, "1h"},        // hours truncate too
		{2*time.Hour + 30*time.Minute, "2h"},
	}
	for _, c := range cases {
		if got := formatElapsed(c.d); got != c.want {
			t.Errorf("formatElapsed(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestElapsedLabelOnlyForActiveStates(t *testing.T) {
	since := time.Now().Add(-3 * time.Minute)
	for _, st := range []Status{StatusStarting, StatusDone, StatusExited} {
		s := &Session{Status: st, StatusSince: since}
		if got := s.ElapsedLabel(); got != "" {
			t.Errorf("ElapsedLabel for status %v = %q, want \"\"", st, got)
		}
	}
	for _, st := range []Status{StatusCrunching, StatusWaiting, StatusIdle} {
		s := &Session{Status: st, StatusSince: since}
		if got := s.ElapsedLabel(); got != "3m" {
			t.Errorf("ElapsedLabel for status %v = %q, want \"3m\"", st, got)
		}
	}
}

func TestElapsedLabelZeroTimeIsBlank(t *testing.T) {
	s := &Session{Status: StatusCrunching} // StatusSince left zero
	if got := s.ElapsedLabel(); got != "" {
		t.Errorf("ElapsedLabel with zero StatusSince = %q, want \"\"", got)
	}
}

// A permission "waiting" arriving after Stop is never a real permission prompt
// (those fire mid-turn, while Crunching) — it's the ~60s idle Notification whose
// wording drifted past isIdleNotification (cmdr-keen-o83). It must NOT stay green
// (that hides the ping) and must NOT go red (that's the o83 regression); it gets
// reclassified to idle/magenta "needs you". A genuine permission prompt fires
// while Crunching, so that transition must still go red untouched.
func TestMarkStatusDonePrecedence(t *testing.T) {
	m := &Manager{sessions: []*Session{{ID: "s1", Status: StatusDone}}}

	m.MarkStatus("s1", StatusWaiting)
	if got := m.Find("s1").Status; got != StatusIdle {
		t.Errorf("waiting after done = %v, want StatusIdle (reclassified to magenta)", got)
	}

	// Crunching from a mid-turn permission flow then waiting still goes red.
	m.MarkStatus("s1", StatusCrunching)
	m.MarkStatus("s1", StatusWaiting)
	if got := m.Find("s1").Status; got != StatusWaiting {
		t.Errorf("waiting after crunching = %v, want StatusWaiting (go red)", got)
	}
}

// The idle ping is exempt from the Done backstop: it's meant to repaint a
// finished-but-untouched session magenta, so it must override Done.
func TestMarkStatusIdleOverridesDone(t *testing.T) {
	m := &Manager{sessions: []*Session{{ID: "s1", Status: StatusDone}}}

	m.MarkStatus("s1", StatusIdle)
	if got := m.Find("s1").Status; got != StatusIdle {
		t.Errorf("idle after done = %v, want StatusIdle (go magenta)", got)
	}
}
