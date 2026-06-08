// Package session owns a single Claude Code process: its PTY, its terminal
// emulator, and its lifecycle. The wrapper stays thin — output is fed into the
// emulator for rendering and the emulator's query replies are pumped back so
// Claude arms its input (the M0 lesson); keystrokes are written straight to the
// PTY by the UI layer.
package session

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
	"github.com/jeffreyd/cmdr-keen/internal/hooks"
)

// Status is the high-level state shown in the sidebar. In M1 only Starting and
// Exited are driven (by lifecycle); Crunching/Waiting/Done get wired to Claude
// Code hooks in M2.
type Status int

const (
	StatusStarting Status = iota
	StatusCrunching
	StatusWaiting // blocked on you (permission or idle)
	StatusDone
	StatusExited
)

// OutputMsg is sent (via the program) when a session produced output and the
// screen may need a repaint. ExitMsg is sent when the child process ends.
type OutputMsg struct{ ID string }
type ExitMsg struct{ ID string }

var idCounter int64

func nextID() string { return fmt.Sprintf("s%d", atomic.AddInt64(&idCounter, 1)) }

// Session is one Claude Code process and everything needed to render/drive it.
type Session struct {
	ID     string
	Name   string // directory basename
	Branch string // git branch, if any
	Title  string // short LLM-generated description of the task (M3)
	Cwd    string
	Status Status

	TitleRequested bool // guards against re-titling on every prompt

	emu          *vt.SafeEmulator
	ptmx         *os.File
	cmd          *exec.Cmd
	settingsPath string // generated hooks settings file to clean up on Close
}

// New spawns `args` (default: claude) in cwd, wired to a paneW×paneH emulator.
// When hookCfg is non-nil and the command is claude, it injects per-session
// hooks via `--settings` so status events flow back to keen. send is the
// program's message pump, used by the output/exit goroutines.
func New(cwd string, args []string, paneW, paneH int, hookCfg *hooks.Config, send func(tea.Msg)) (*Session, error) {
	if len(args) == 0 {
		args = []string{"claude"}
	}
	if paneW < 1 {
		paneW = 80
	}
	if paneH < 1 {
		paneH = 24
	}

	id := nextID()
	env := append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	finalArgs := args
	settingsPath := ""

	// Wire hooks only for real claude sessions (don't break `keen -- bash`).
	if hookCfg != nil && isClaude(args[0]) {
		sp := hookCfg.SettingsPath(id)
		if err := hooks.WriteSettings(sp, hookCfg.HookBin); err == nil {
			settingsPath = sp
			finalArgs = append([]string{args[0], "--settings", sp}, args[1:]...)
			env = append(env, "KEEN_SOCKET="+hookCfg.Socket, "KEEN_SESSION="+id)
		}
	}

	c := exec.Command(finalArgs[0], finalArgs[1:]...)
	c.Dir = cwd
	c.Env = env

	ptmx, err := pty.Start(c)
	if err != nil {
		return nil, fmt.Errorf("start pty: %w", err)
	}
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(paneH), Cols: uint16(paneW)})

	emu := vt.NewSafeEmulator(paneW, paneH)

	s := &Session{
		ID:           id,
		Name:         filepath.Base(cwd),
		Branch:       gitBranch(cwd),
		Cwd:          cwd,
		Status:       StatusStarting,
		emu:          emu,
		ptmx:         ptmx,
		cmd:          c,
		settingsPath: settingsPath,
	}

	// PTY output -> emulator -> repaint nudge.
	go func() {
		buf := make([]byte, 8192)
		for {
			n, rerr := ptmx.Read(buf)
			if n > 0 {
				_, _ = emu.Write(buf[:n])
				send(OutputMsg{ID: s.ID})
			}
			if rerr != nil {
				send(ExitMsg{ID: s.ID})
				return
			}
		}
	}()

	// Emulator query replies (cursor reports, device attributes, …) -> PTY, so
	// Claude's startup handshake completes and input arms. See spike/README.md.
	go func() { _, _ = io.Copy(ptmx, emu) }()

	return s, nil
}

// Write forwards input bytes to the child.
func (s *Session) Write(b []byte) {
	if len(b) > 0 {
		_, _ = s.ptmx.Write(b)
	}
}

// Render returns the current emulated screen (paneW×paneH) as a styled string.
func (s *Session) Render() string { return s.emu.Render() }

// Resize matches the emulator and PTY to a new pane size.
func (s *Session) Resize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	s.emu.Resize(w, h)
	_ = pty.Setsize(s.ptmx, &pty.Winsize{Rows: uint16(h), Cols: uint16(w)})
}

func (s *Session) Focus() { s.emu.Focus() }
func (s *Session) Blur()  { s.emu.Blur() }

// Close kills the child and releases the PTY.
func (s *Session) Close() {
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	_ = s.ptmx.Close()
	if s.settingsPath != "" {
		_ = os.Remove(s.settingsPath)
	}
}

func isClaude(cmd string) bool { return filepath.Base(cmd) == "claude" }

func gitBranch(cwd string) string {
	c := exec.Command("git", "-C", cwd, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := c.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
