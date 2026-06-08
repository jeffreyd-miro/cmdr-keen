// Package debug is keen's optional file logger, used to trace why input lands
// where it does (keystroke routing, active-session switches). It is gated on
// KEEN_DEBUG so it costs nothing in normal use: unset, every Logf is a no-op;
// set to a path, lines are appended there; set to any other non-empty value,
// they go to $TMPDIR/keen-debug.log. keen runs in the alt-screen, so logging to
// a file is the only safe channel — stdout/stderr would corrupt the TUI.
package debug

import (
	"log"
	"os"
	"path/filepath"
)

var logger *log.Logger

func init() {
	v := os.Getenv("KEEN_DEBUG")
	if v == "" {
		return
	}
	path := v
	if path == "1" || path == "true" || path == "on" {
		path = filepath.Join(os.TempDir(), "keen-debug.log")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return // best-effort: a bad path just leaves logging off
	}
	logger = log.New(f, "", log.Ltime|log.Lmicroseconds)
	logger.Printf("--- keen debug log opened (pid %d) ---", os.Getpid())
}

// Enabled reports whether the debug log is active, for callers that want to
// skip building an expensive log message.
func Enabled() bool { return logger != nil }

// Logf appends a line to the debug log if one is open, else does nothing.
func Logf(format string, args ...any) {
	if logger != nil {
		logger.Printf(format, args...)
	}
}
