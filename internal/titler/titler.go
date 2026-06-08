// Package titler turns a session's first prompt into a short, human tab title.
// It shells out to the claude CLI in print mode with the Haiku model, which
// reuses the user's existing Claude auth — no API key to configure. Callers
// should treat it as best-effort and fall back to a heuristic on error.
package titler

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

const instruction = "Summarize this coding task as a terminal tab title. " +
	"Reply with ONLY the title: 2 to 4 words, lowercase, no punctuation, no quotes.\n\nTask:\n"

// Generate returns a short title for the given task prompt via `claude -p`.
func Generate(prompt string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if len(prompt) > 600 { // a title only needs the gist
		prompt = prompt[:600]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "-p", instruction+prompt, "--model", "haiku")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return Clean(string(out)), nil
}

// Clean normalizes model (or heuristic) output into a tidy one-line title.
func Clean(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(strings.Trim(s, "\"'`."))
	if len(s) > 40 {
		s = strings.TrimSpace(s[:40])
	}
	return s
}

// Heuristic is the no-LLM fallback: the first line of the prompt, trimmed.
func Heuristic(prompt string) string {
	return Clean(prompt)
}
