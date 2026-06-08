package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// KeyToBytes reconstructs the terminal wire bytes for a Bubble Tea key event so
// the wrapper can forward them to a session's PTY. This is the fidelity layer:
// it must cover everything a user might press inside Claude. Anything not mapped
// falls back to nothing (logged by callers if needed).
func KeyToBytes(k tea.KeyMsg) []byte {
	// Bracketed paste: re-wrap so Claude treats it as a paste (dropped images
	// arrive as "[image N]", not a raw path string).
	if k.Paste {
		return []byte("\x1b[200~" + string(k.Runes) + "\x1b[201~")
	}

	// Printable runes (and space, which v1 reports as a distinct type).
	if k.Type == tea.KeyRunes || k.Type == tea.KeySpace {
		s := string(k.Runes)
		if k.Type == tea.KeySpace && s == "" {
			s = " "
		}
		if k.Alt {
			return append([]byte{0x1b}, []byte(s)...)
		}
		return []byte(s)
	}

	// Named special keys → canonical xterm sequences.
	if seq, ok := specialKeys[k.Type]; ok {
		if k.Alt && len(seq) > 0 && seq[0] == 0x1b {
			// alt + escape-sequence key: prefix an extra ESC (xterm convention).
			return append([]byte{0x1b}, seq...)
		}
		return seq
	}

	// Control keys: in Bubble Tea v1 the KeyType value IS the C0 control byte
	// (ctrl+a==1 … ctrl+_==31). Covers ctrl+c, ctrl+v, tab, enter, esc, etc.
	if k.Type > 0 && k.Type < 32 {
		if k.Alt {
			return []byte{0x1b, byte(k.Type)}
		}
		return []byte{byte(k.Type)}
	}

	return nil
}

// specialKeys maps non-printable, non-C0 keys to their xterm byte sequences.
var specialKeys = map[tea.KeyType][]byte{
	tea.KeyEnter:     {'\r'},
	tea.KeyTab:       {'\t'},
	tea.KeyEsc:       {0x1b},
	tea.KeyBackspace: {0x7f},

	tea.KeyUp:    []byte("\x1b[A"),
	tea.KeyDown:  []byte("\x1b[B"),
	tea.KeyRight: []byte("\x1b[C"),
	tea.KeyLeft:  []byte("\x1b[D"),

	tea.KeyShiftTab: []byte("\x1b[Z"),
	tea.KeyHome:     []byte("\x1b[H"),
	tea.KeyEnd:      []byte("\x1b[F"),
	tea.KeyPgUp:     []byte("\x1b[5~"),
	tea.KeyPgDown:   []byte("\x1b[6~"),
	tea.KeyDelete:   []byte("\x1b[3~"),
	tea.KeyInsert:   []byte("\x1b[2~"),

	tea.KeyCtrlUp:    []byte("\x1b[1;5A"),
	tea.KeyCtrlDown:  []byte("\x1b[1;5B"),
	tea.KeyCtrlRight: []byte("\x1b[1;5C"),
	tea.KeyCtrlLeft:  []byte("\x1b[1;5D"),

	tea.KeyShiftUp:    []byte("\x1b[1;2A"),
	tea.KeyShiftDown:  []byte("\x1b[1;2B"),
	tea.KeyShiftRight: []byte("\x1b[1;2C"),
	tea.KeyShiftLeft:  []byte("\x1b[1;2D"),

	tea.KeyF1:  []byte("\x1bOP"),
	tea.KeyF2:  []byte("\x1bOQ"),
	tea.KeyF3:  []byte("\x1bOR"),
	tea.KeyF4:  []byte("\x1bOS"),
	tea.KeyF5:  []byte("\x1b[15~"),
	tea.KeyF6:  []byte("\x1b[17~"),
	tea.KeyF7:  []byte("\x1b[18~"),
	tea.KeyF8:  []byte("\x1b[19~"),
	tea.KeyF9:  []byte("\x1b[20~"),
	tea.KeyF10: []byte("\x1b[21~"),
	tea.KeyF11: []byte("\x1b[23~"),
	tea.KeyF12: []byte("\x1b[24~"),
}

// MouseToSGR encodes a mouse event into an SGR mouse report, translating screen
// coordinates into the active pane's local coordinate space. Returns nil if the
// event is outside the pane. Lets in-session scroll/click reach Claude.
func MouseToSGR(m tea.MouseMsg, l Layout) []byte {
	// Pane content origin in 0-based screen coords.
	px := m.X - l.PaneScreenX
	py := m.Y - l.PaneScreenY
	if px < 0 || py < 0 || px >= l.PaneW || py >= l.PaneH {
		return nil
	}

	cb, ok := buttonCode(m.Button)
	if !ok {
		return nil
	}
	if m.Action == tea.MouseActionMotion {
		cb += 32
	}
	if m.Shift {
		cb += 4
	}
	if m.Alt {
		cb += 8
	}
	if m.Ctrl {
		cb += 16
	}

	final := byte('M') // press / motion / wheel
	if m.Action == tea.MouseActionRelease {
		final = 'm'
	}
	// SGR coordinates are 1-based.
	return []byte(fmt.Sprintf("\x1b[<%d;%d;%d%c", cb, px+1, py+1, final))
}

func buttonCode(b tea.MouseButton) (int, bool) {
	switch b {
	case tea.MouseButtonLeft:
		return 0, true
	case tea.MouseButtonMiddle:
		return 1, true
	case tea.MouseButtonRight:
		return 2, true
	case tea.MouseButtonNone: // motion with no button held
		return 3, true
	case tea.MouseButtonWheelUp:
		return 64, true
	case tea.MouseButtonWheelDown:
		return 65, true
	case tea.MouseButtonWheelLeft:
		return 66, true
	case tea.MouseButtonWheelRight:
		return 67, true
	case tea.MouseButtonBackward:
		return 128, true
	case tea.MouseButtonForward:
		return 129, true
	}
	return 0, false
}
