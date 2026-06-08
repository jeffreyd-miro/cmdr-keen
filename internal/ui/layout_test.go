package ui

import "testing"

func TestComputeLayout(t *testing.T) {
	l := ComputeLayout(120, 40)

	if l.SidebarW != defaultSidebarW {
		t.Errorf("SidebarW = %d, want %d", l.SidebarW, defaultSidebarW)
	}
	if l.SidebarTotal != defaultSidebarW+boxBorder {
		t.Errorf("SidebarTotal = %d, want %d", l.SidebarTotal, defaultSidebarW+boxBorder)
	}
	// pane = total width - sidebar box - pane border
	if want := 120 - l.SidebarTotal - boxBorder; l.PaneW != want {
		t.Errorf("PaneW = %d, want %d", l.PaneW, want)
	}
	if want := 40 - boxBorder; l.PaneH != want {
		t.Errorf("PaneH = %d, want %d", l.PaneH, want)
	}
	// Pane content begins just past the sidebar box and the pane's own border.
	if l.PaneScreenX != l.SidebarTotal+1 {
		t.Errorf("PaneScreenX = %d, want %d", l.PaneScreenX, l.SidebarTotal+1)
	}
	if l.PaneScreenY != 1 {
		t.Errorf("PaneScreenY = %d, want 1", l.PaneScreenY)
	}
}

func TestComputeLayoutNarrow(t *testing.T) {
	// Very narrow terminal: sidebar shrinks but pane stays at least 1 wide/tall.
	l := ComputeLayout(20, 3)
	if l.SidebarW < 8 {
		t.Errorf("SidebarW = %d, want >= 8 (floor)", l.SidebarW)
	}
	if l.PaneW < 1 {
		t.Errorf("PaneW = %d, want >= 1", l.PaneW)
	}
	if l.PaneH < 1 {
		t.Errorf("PaneH = %d, want >= 1", l.PaneH)
	}
}

func TestSessionIndexAt(t *testing.T) {
	l := ComputeLayout(120, 40)
	const count = 3
	row0 := l.sessionRow0()

	tests := []struct {
		name string
		x, y int
		want int
	}{
		{"first session, topic line", 2, row0, 0},
		{"first session, task line", 2, row0 + 1, 0},
		{"first session, phase line", 2, row0 + 2, 0},
		{"second session, topic line", 2, row0 + 3, 1},
		{"second session, phase line", 2, row0 + 5, 1},
		{"third session", 2, row0 + 6, 2},
		{"above the list (header)", 2, row0 - 1, -1},
		{"below the last session", 2, row0 + linesPerSession*count, -1},
		{"right of the sidebar", l.SidebarTotal, row0, -1},
		{"negative x", -1, row0, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := l.SessionIndexAt(tt.x, tt.y, count); got != tt.want {
				t.Errorf("SessionIndexAt(%d,%d) = %d, want %d", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

func TestInPane(t *testing.T) {
	l := ComputeLayout(120, 40)

	tests := []struct {
		name string
		x, y int
		want bool
	}{
		{"top-left content cell", l.PaneScreenX, l.PaneScreenY, true},
		{"bottom-right content cell", l.PaneScreenX + l.PaneW - 1, l.PaneScreenY + l.PaneH - 1, true},
		{"one column too far left", l.PaneScreenX - 1, l.PaneScreenY, false},
		{"one column too far right", l.PaneScreenX + l.PaneW, l.PaneScreenY, false},
		{"one row too high", l.PaneScreenX, l.PaneScreenY - 1, false},
		{"one row too low", l.PaneScreenX, l.PaneScreenY + l.PaneH, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := l.InPane(tt.x, tt.y); got != tt.want {
				t.Errorf("InPane(%d,%d) = %v, want %v", tt.x, tt.y, got, tt.want)
			}
		})
	}
}
