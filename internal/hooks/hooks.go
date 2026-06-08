// Package hooks bridges Claude Code's lifecycle hooks to keen's sidebar. keen
// listens on a unix socket; each session launches `claude --settings <file>`
// with a generated settings file that registers hooks pointing at the tiny
// cc-deck-hook helper. The helper reports the event (with the session's
// KEEN_SESSION) back over the socket, and keen turns it into a status color.
//
// This keeps the user's global ~/.claude untouched — the hooks live only in the
// per-session settings file keen writes.
package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config carries what a session needs to wire its hooks back to keen.
type Config struct {
	Socket  string // unix socket keen listens on
	HookBin string // path to the cc-deck-hook helper
}

// DefaultSocketPath returns a short, per-process socket path. We use /tmp to
// stay under the ~104-char unix socket path limit on macOS.
func DefaultSocketPath() string {
	return filepath.Join("/tmp", fmt.Sprintf("keen-%d.sock", os.Getpid()))
}

// SettingsPath is where this session's generated settings file lives.
func (c *Config) SettingsPath(sessionID string) string {
	return filepath.Join("/tmp", fmt.Sprintf("keen-%d-%s-settings.json", os.Getpid(), sessionID))
}

// ResolveHookBin returns the command prefix Claude should invoke at each
// lifecycle event (keen appends the event name). keen is a multi-call binary,
// so by default it points hooks back at the running keen executable via its
// hidden `__hook` subcommand — this is what makes a single `go install` of keen
// self-contained, with no sibling cc-deck-hook to locate.
//
// Resolution order: an explicit KEEN_HOOK_BIN override (used as-is), then
// `<keen> __hook` (path quoted so spaces survive the shell), then a bare
// `cc-deck-hook` on PATH as a last resort.
func ResolveHookBin() string {
	if p := os.Getenv("KEEN_HOOK_BIN"); p != "" {
		return p
	}
	if exe, err := os.Executable(); err == nil {
		return fmt.Sprintf("%q __hook", exe)
	}
	return "cc-deck-hook"
}

// WriteSettings writes a Claude Code settings file registering keen's hooks.
// Event names match the strings keen maps to statuses (see ui/model.go).
func WriteSettings(path, hookBin string) error {
	hook := func(event string) map[string]any {
		return map[string]any{
			"hooks": []map[string]any{
				{"type": "command", "command": fmt.Sprintf("%s %s", hookBin, event)},
			},
		}
	}
	hookMatch := func(event string) map[string]any {
		m := hook(event)
		m["matcher"] = "*"
		return m
	}
	settings := map[string]any{
		"hooks": map[string]any{
			"SessionStart":     []any{hook("start")},
			"UserPromptSubmit": []any{hook("crunching")},
			"PreToolUse":       []any{hookMatch("crunching")},
			"Notification":     []any{hook("waiting")},
			"Stop":             []any{hook("done")},
			"SessionEnd":       []any{hook("exit")},
		},
	}
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}
