package main

import "testing"

func TestIsIdleNotification(t *testing.T) {
	cases := []struct {
		message string
		want    bool
	}{
		// Idle timeout — keen ignores these so a done session stays green.
		{"Claude is waiting for your input", true},
		{"claude is WAITING FOR YOUR INPUT", true}, // case-insensitive
		// Permission/attention requests — keen surfaces these as red.
		{"Claude needs your permission to use Bash", false},
		{"Claude needs your permission to run a command", false},
		{"", false}, // empty payload: treat as a real attention event, not idle
	}
	for _, c := range cases {
		if got := isIdleNotification(c.message); got != c.want {
			t.Errorf("isIdleNotification(%q) = %v, want %v", c.message, got, c.want)
		}
	}
}
