package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
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

// MouseToVT translates a Bubble Tea mouse event into a vt mouse event in the
// active pane's local coordinate space, ready to hand to the emulator's
// SendMouse. The emulator forwards it to Claude using whatever mouse mode and
// encoding Claude negotiated (and silently drops it if Claude isn't tracking
// the mouse), so keen never has to guess the wire format. The bool is false
// when the event falls outside the pane.
func MouseToVT(m tea.MouseMsg, l Layout) (vt.Mouse, bool) {
	// Pane content origin in 0-based screen coords; vt wants 0-based local.
	px := m.X - l.PaneScreenX
	py := m.Y - l.PaneScreenY
	if px < 0 || py < 0 || px >= l.PaneW || py >= l.PaneH {
		return nil, false
	}

	mouse := uv.Mouse{X: px, Y: py, Button: buttonToVT(m.Button)}
	if m.Shift {
		mouse.Mod |= vt.ModShift
	}
	if m.Alt {
		mouse.Mod |= vt.ModAlt
	}
	if m.Ctrl {
		mouse.Mod |= vt.ModCtrl
	}

	switch {
	case isWheel(m.Button):
		return vt.MouseWheel(mouse), true
	case m.Action == tea.MouseActionRelease:
		return vt.MouseRelease(mouse), true
	case m.Action == tea.MouseActionMotion:
		return vt.MouseMotion(mouse), true
	default: // press
		return vt.MouseClick(mouse), true
	}
}

func isWheel(b tea.MouseButton) bool {
	switch b {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown,
		tea.MouseButtonWheelLeft, tea.MouseButtonWheelRight:
		return true
	}
	return false
}

func buttonToVT(b tea.MouseButton) vt.MouseButton {
	switch b {
	case tea.MouseButtonLeft:
		return vt.MouseLeft
	case tea.MouseButtonMiddle:
		return vt.MouseMiddle
	case tea.MouseButtonRight:
		return vt.MouseRight
	case tea.MouseButtonWheelUp:
		return vt.MouseWheelUp
	case tea.MouseButtonWheelDown:
		return vt.MouseWheelDown
	case tea.MouseButtonWheelLeft:
		return vt.MouseWheelLeft
	case tea.MouseButtonWheelRight:
		return vt.MouseWheelRight
	case tea.MouseButtonBackward:
		return vt.MouseBackward
	case tea.MouseButtonForward:
		return vt.MouseForward
	}
	return vt.MouseNone
}
