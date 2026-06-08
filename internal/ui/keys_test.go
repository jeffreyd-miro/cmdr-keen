package ui

import (
	"bytes"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestKeyToBytes(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyMsg
		want []byte
	}{
		{
			name: "printable rune",
			key:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")},
			want: []byte("a"),
		},
		{
			name: "multibyte rune",
			key:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("é")},
			want: []byte("é"),
		},
		{
			name: "alt+rune prefixes ESC",
			key:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b"), Alt: true},
			want: []byte{0x1b, 'b'},
		},
		{
			name: "space",
			key:  tea.KeyMsg{Type: tea.KeySpace},
			want: []byte(" "),
		},
		{
			name: "enter",
			key:  tea.KeyMsg{Type: tea.KeyEnter},
			want: []byte{'\r'},
		},
		{
			name: "backspace",
			key:  tea.KeyMsg{Type: tea.KeyBackspace},
			want: []byte{0x7f},
		},
		{
			name: "up arrow",
			key:  tea.KeyMsg{Type: tea.KeyUp},
			want: []byte("\x1b[A"),
		},
		{
			name: "alt+up prefixes an extra ESC",
			key:  tea.KeyMsg{Type: tea.KeyUp, Alt: true},
			want: []byte("\x1b\x1b[A"),
		},
		{
			name: "ctrl+c is the C0 byte",
			key:  tea.KeyMsg{Type: tea.KeyCtrlC},
			want: []byte{0x03},
		},
		{
			name: "ctrl+a is the C0 byte",
			key:  tea.KeyMsg{Type: tea.KeyCtrlA},
			want: []byte{0x01},
		},
		{
			name: "alt+ctrl key prefixes ESC",
			key:  tea.KeyMsg{Type: tea.KeyCtrlA, Alt: true},
			want: []byte{0x1b, 0x01},
		},
		{
			name: "F5 (CSI form)",
			key:  tea.KeyMsg{Type: tea.KeyF5},
			want: []byte("\x1b[15~"),
		},
		{
			name: "shift+tab",
			key:  tea.KeyMsg{Type: tea.KeyShiftTab},
			want: []byte("\x1b[Z"),
		},
		{
			name: "unknown key maps to nothing",
			key:  tea.KeyMsg{Type: tea.KeyType(99999)},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KeyToBytes(tt.key)
			if !bytes.Equal(got, tt.want) {
				t.Errorf("KeyToBytes() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestKeyToBytesPaste(t *testing.T) {
	k := tea.KeyMsg{Paste: true, Runes: []rune("hello world")}
	got := KeyToBytes(k)
	want := []byte("\x1b[200~hello world\x1b[201~")
	if !bytes.Equal(got, want) {
		t.Errorf("paste = %q, want %q", got, want)
	}
}

func TestMouseToSGR(t *testing.T) {
	// A pane whose content origin sits at screen (27, 1).
	l := ComputeLayout(120, 40)

	t.Run("left press translates to pane-local 1-based coords", func(t *testing.T) {
		m := tea.MouseMsg{
			X:      l.PaneScreenX, // top-left content cell
			Y:      l.PaneScreenY,
			Button: tea.MouseButtonLeft,
			Action: tea.MouseActionPress,
		}
		got := MouseToSGR(m, l)
		want := []byte("\x1b[<0;1;1M")
		if !bytes.Equal(got, want) {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("release uses lowercase final byte", func(t *testing.T) {
		m := tea.MouseMsg{
			X:      l.PaneScreenX + 2,
			Y:      l.PaneScreenY + 3,
			Button: tea.MouseButtonLeft,
			Action: tea.MouseActionRelease,
		}
		got := MouseToSGR(m, l)
		want := []byte("\x1b[<0;3;4m")
		if !bytes.Equal(got, want) {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("wheel up carries code 64", func(t *testing.T) {
		m := tea.MouseMsg{
			X:      l.PaneScreenX,
			Y:      l.PaneScreenY,
			Button: tea.MouseButtonWheelUp,
			Action: tea.MouseActionPress,
		}
		got := MouseToSGR(m, l)
		want := []byte("\x1b[<64;1;1M")
		if !bytes.Equal(got, want) {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("modifiers add to the button code", func(t *testing.T) {
		m := tea.MouseMsg{
			X:      l.PaneScreenX,
			Y:      l.PaneScreenY,
			Button: tea.MouseButtonLeft,
			Action: tea.MouseActionPress,
			Shift:  true, // +4
			Ctrl:   true, // +16
		}
		got := MouseToSGR(m, l)
		want := []byte("\x1b[<20;1;1M")
		if !bytes.Equal(got, want) {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("outside the pane returns nil", func(t *testing.T) {
		// A click in the sidebar region (left of the pane).
		m := tea.MouseMsg{X: 0, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
		if got := MouseToSGR(m, l); got != nil {
			t.Errorf("expected nil outside pane, got %q", got)
		}
	})

	t.Run("just past the pane edge returns nil", func(t *testing.T) {
		m := tea.MouseMsg{
			X:      l.PaneScreenX + l.PaneW, // one column too far right
			Y:      l.PaneScreenY,
			Button: tea.MouseButtonLeft,
			Action: tea.MouseActionPress,
		}
		if got := MouseToSGR(m, l); got != nil {
			t.Errorf("expected nil at pane edge, got %q", got)
		}
	})
}
