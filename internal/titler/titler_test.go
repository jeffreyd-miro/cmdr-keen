package titler

import (
	"strings"
	"testing"
)

func TestClean(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"trims surrounding whitespace", "  fix login bug  ", "fix login bug"},
		{"keeps only the first line", "add vt parsing\nblah blah", "add vt parsing"},
		{"strips wrapping quotes", "\"refactor auth\"", "refactor auth"},
		{"strips backticks and trailing period", "`update deps`.", "update deps"},
		{"collapses to empty on whitespace only", "   \n  ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Clean(tt.in); got != tt.want {
				t.Errorf("Clean(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCleanTruncates(t *testing.T) {
	long := strings.Repeat("x", 100)
	got := Clean(long)
	if len([]rune(got)) > 40 {
		t.Errorf("Clean did not cap length: got %d runes", len([]rune(got)))
	}
}

func TestHeuristic(t *testing.T) {
	// Heuristic is the no-LLM fallback: first line of the prompt, cleaned.
	in := "Implement the parser\nand then write tests"
	if got, want := Heuristic(in), "Implement the parser"; got != want {
		t.Errorf("Heuristic() = %q, want %q", got, want)
	}
}
