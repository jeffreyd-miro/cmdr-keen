package ui

import (
	"bytes"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/vt"
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

func TestMouseToVT(t *testing.T) {
	// A pane whose content origin sits at screen (27, 1).
	l := ComputeLayout(120, 40)

	t.Run("left press becomes a click at pane-local 0-based coords", func(t *testing.T) {
		m := tea.MouseMsg{
			X:      l.PaneScreenX + 2, // top-left content cell + offset
			Y:      l.PaneScreenY + 3,
			Button: tea.MouseButtonLeft,
			Action: tea.MouseActionPress,
		}
		got, ok := MouseToVT(m, l)
		if !ok {
			t.Fatal("expected ok inside pane")
		}
		click, isClick := got.(vt.MouseClick)
		if !isClick {
			t.Fatalf("expected MouseClick, got %T", got)
		}
		if click.X != 2 || click.Y != 3 {
			t.Errorf("coords = (%d,%d), want (2,3)", click.X, click.Y)
		}
		if click.Button != vt.MouseLeft {
			t.Errorf("button = %v, want left", click.Button)
		}
	})

	t.Run("release becomes a MouseRelease", func(t *testing.T) {
		m := tea.MouseMsg{
			X:      l.PaneScreenX,
			Y:      l.PaneScreenY,
			Button: tea.MouseButtonLeft,
			Action: tea.MouseActionRelease,
		}
		got, _ := MouseToVT(m, l)
		if _, ok := got.(vt.MouseRelease); !ok {
			t.Errorf("expected MouseRelease, got %T", got)
		}
	})

	t.Run("wheel up becomes a MouseWheel regardless of action", func(t *testing.T) {
		m := tea.MouseMsg{
			X:      l.PaneScreenX,
			Y:      l.PaneScreenY,
			Button: tea.MouseButtonWheelUp,
			Action: tea.MouseActionPress,
		}
		got, _ := MouseToVT(m, l)
		wheel, ok := got.(vt.MouseWheel)
		if !ok {
			t.Fatalf("expected MouseWheel, got %T", got)
		}
		if wheel.Button != vt.MouseWheelUp {
			t.Errorf("button = %v, want wheel-up", wheel.Button)
		}
	})

	t.Run("modifiers carry through", func(t *testing.T) {
		m := tea.MouseMsg{
			X:      l.PaneScreenX,
			Y:      l.PaneScreenY,
			Button: tea.MouseButtonLeft,
			Action: tea.MouseActionPress,
			Shift:  true,
			Ctrl:   true,
		}
		got, _ := MouseToVT(m, l)
		click := got.(vt.MouseClick)
		if !click.Mod.Contains(vt.ModShift) || !click.Mod.Contains(vt.ModCtrl) {
			t.Errorf("mods = %v, want shift+ctrl", click.Mod)
		}
	})

	t.Run("outside the pane returns ok=false", func(t *testing.T) {
		// A click in the sidebar region (left of the pane).
		m := tea.MouseMsg{X: 0, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
		if _, ok := MouseToVT(m, l); ok {
			t.Error("expected ok=false outside pane")
		}
	})

	t.Run("just past the pane edge returns ok=false", func(t *testing.T) {
		m := tea.MouseMsg{
			X:      l.PaneScreenX + l.PaneW, // one column too far right
			Y:      l.PaneScreenY,
			Button: tea.MouseButtonLeft,
			Action: tea.MouseActionPress,
		}
		if _, ok := MouseToVT(m, l); ok {
			t.Error("expected ok=false at pane edge")
		}
	})
}
