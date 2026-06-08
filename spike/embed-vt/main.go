// Spike B — embedded VT pane inside Bubble Tea.
//
// Purpose: test the always-on-sidebar path. Bubble Tea owns the screen; the
// child `claude` runs in a PTY whose output we feed into a charmbracelet/x/vt
// emulator and render in a bordered pane. This is what lets a sidebar live
// next to a live session — BUT input now passes through Bubble Tea's key
// parser, so we must reconstruct bytes to forward to the PTY. The open fidelity
// question (vs Spike A) is whether image paste + dictation survive that round
// trip. Compare the two spikes side by side, then pick the architecture.
//
//	go run ./spike/embed-vt            # wraps `claude` in $PWD
//	go run ./spike/embed-vt -- bash    # wraps an arbitrary command
//
// Prefix Ctrl-K is intercepted (here it quits). Everything else is forwarded.
//
// Set KEEN_DEBUG=/tmp/keen-embed.log to trace window size, every key event, the
// bytes we forward, and the emulator query-responses we pump back.
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

var dbg *log.Logger

func tracef(format string, a ...any) {
	if dbg != nil {
		dbg.Printf(format, a...)
	}
}

func resizePTY(f *os.File, rows, cols int) {
	_ = pty.Setsize(f, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

const (
	defCols  = 80
	defRows  = 24
	sidebarW = 24 // sidebar CONTENT width; +2 for its border
	border   = 2  // border cols/rows a lipgloss box adds
)

func main() {
	if p := os.Getenv("KEEN_DEBUG"); p != "" {
		f, err := os.Create(p)
		if err == nil {
			dbg = log.New(f, "", log.Ltime|log.Lmicroseconds)
			defer f.Close()
		}
	}

	args := []string{"claude"}
	for i, a := range os.Args {
		if a == "--" {
			args = os.Args[i+1:]
			break
		}
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start pty:", err)
		os.Exit(1)
	}
	resizePTY(ptmx, defRows, defCols)

	// Emulator exists up front (no nil "starting…" window). SafeEmulator is
	// goroutine-safe for concurrent Write (output) + Read (query replies).
	emu := vt.NewSafeEmulator(defCols, defRows)
	emu.Focus()

	m := &model{ptmx: ptmx, emu: emu, args: args}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseAllMotion())

	// (1) PTY output -> emulator, then nudge a repaint.
	go func() {
		buf := make([]byte, 8192)
		for {
			n, rerr := ptmx.Read(buf)
			if n > 0 {
				_, _ = emu.Write(buf[:n])
				p.Send(dirtyMsg{})
			}
			if rerr != nil {
				tracef("pty read EOF: %v", rerr)
				p.Send(exitMsg{})
				return
			}
		}
	}()

	// (2) THE FIX: emulator's generated replies (cursor-position reports, device
	// attributes, etc.) -> back to the PTY, so Claude's startup queries get
	// answered and it actually arms its input. Without this the session looks
	// dead — you can't type.
	go func() {
		n, err := io.Copy(ptmx, emu)
		tracef("response pump ended after %d bytes: %v", n, err)
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
	}
	_ = ptmx.Close()
}

type dirtyMsg struct{}
type exitMsg struct{}
type tickMsg struct{}

type model struct {
	ptmx *os.File
	emu  *vt.SafeEmulator
	w, h int
	args []string
}

func (m *model) Init() tea.Cmd {
	return tea.Tick(time.Second/60, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		paneW := m.w - (sidebarW + border) - border
		paneH := m.h - border
		if paneW < 1 {
			paneW = 1
		}
		if paneH < 1 {
			paneH = 1
		}
		tracef("resize term=%dx%d pane=%dx%d", m.w, m.h, paneW, paneH)
		m.emu.Resize(paneW, paneH)
		resizePTY(m.ptmx, paneH, paneW)
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlK { // the keen prefix — intercepted
			tracef("key: Ctrl-K (prefix) -> quit")
			return m, tea.Quit
		}
		b := keyToBytes(msg)
		tracef("key: type=%v runes=%q paste=%v -> %d bytes %q",
			msg.Type, string(msg.Runes), msg.Paste, len(b), string(b))
		if len(b) > 0 {
			if _, err := m.ptmx.Write(b); err != nil {
				tracef("pty write err: %v", err)
			}
		}
		return m, nil

	case tea.MouseMsg:
		// Fidelity TODO: encode mouse back to SGR and forward. Left for the real
		// build; mouse-scroll is part of the fidelity checklist.
		return m, nil

	case tickMsg:
		return m, tea.Tick(time.Second/60, func(time.Time) tea.Msg { return tickMsg{} })
	case dirtyMsg:
		return m, nil
	case exitMsg:
		return m, tea.Quit
	}
	return m, nil
}

func (m *model) View() string {
	pane := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Render(m.emu.Render())

	rows := []string{
		lipgloss.NewStyle().Bold(true).Render("sessions"),
		"› " + lastArg(m.args) + "  ●",
		"",
		lipgloss.NewStyle().Faint(true).Render("⌃k quit"),
	}
	sidebar := lipgloss.NewStyle().
		Width(sidebarW).
		Border(lipgloss.RoundedBorder()).
		Render(joinLines(rows))

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, pane)
}

// keyToBytes reconstructs the wire bytes for a Bubble Tea key event so we can
// forward them to the PTY. This is the lossy layer Spike A avoids. Covers the
// common cases needed to exercise fidelity (text, Enter, Backspace, Tab, Esc,
// arrows, Ctrl-V for image paste, and bracketed-paste text).
func keyToBytes(k tea.KeyMsg) []byte {
	if k.Paste { // re-wrap in bracketed-paste markers so Claude ingests it as a
		// paste (e.g. a dropped image becomes "[image 1]", not a raw path string)
		return []byte("\x1b[200~" + string(k.Runes) + "\x1b[201~")
	}
	switch k.Type {
	case tea.KeyRunes, tea.KeySpace:
		s := string(k.Runes)
		if k.Alt {
			return append([]byte{0x1b}, []byte(s)...)
		}
		return []byte(s)
	case tea.KeyEnter:
		return []byte{'\r'}
	case tea.KeyBackspace:
		return []byte{0x7f}
	case tea.KeyTab:
		return []byte{'\t'}
	case tea.KeyEsc:
		return []byte{0x1b}
	case tea.KeyCtrlC:
		return []byte{0x03}
	case tea.KeyCtrlV: // image paste trigger — Claude reads the OS clipboard
		return []byte{0x16}
	case tea.KeyCtrlD:
		return []byte{0x04}
	case tea.KeyUp:
		return []byte("\x1b[A")
	case tea.KeyDown:
		return []byte("\x1b[B")
	case tea.KeyRight:
		return []byte("\x1b[C")
	case tea.KeyLeft:
		return []byte("\x1b[D")
	}
	if len(k.Runes) == 1 { // many Ctrl combos map to a single control byte
		return []byte(string(k.Runes))
	}
	return nil
}

func lastArg(a []string) string {
	if len(a) == 0 {
		return "?"
	}
	return a[len(a)-1]
}

func joinLines(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += "\n"
		}
		out += s
	}
	return out
}
