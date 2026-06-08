package ui

// Layout is the single source of truth for on-screen geometry. Sidebar render,
// pane render, mouse hit-testing, and PTY resize all derive from it, so they
// can never disagree. All screen coordinates are 0-based (matching Bubble Tea
// mouse events).
//
// Horizontal:  [ sidebar box: SidebarW + 2 border ][ pane box: PaneW + 2 border ]
// Vertical:    row 0 = top border, content rows 1..H-2, row H-1 = bottom border
type Layout struct {
	W, H int

	SidebarW     int // sidebar content width (inside its border)
	SidebarTotal int // SidebarW + 2

	PaneW, PaneH             int // pane content size == emulator size
	PaneScreenX, PaneScreenY int // 0-based screen coords of pane content cell (0,0)
}

const (
	defaultSidebarW = 30 // fits a topic, task, and a phase word + context bar
	sidebarHeader   = 2  // "sessions" title + one blank line
	boxBorder       = 2  // a lipgloss border adds 1 cell on each side
)

// ComputeLayout derives geometry from the terminal size.
func ComputeLayout(w, h int) Layout {
	sidebarW := defaultSidebarW
	if w < sidebarW+boxBorder+10 { // shrink the sidebar on very narrow terminals
		sidebarW = max(8, w-boxBorder-10)
	}
	sidebarTotal := sidebarW + boxBorder

	paneW := w - sidebarTotal - boxBorder
	paneH := h - boxBorder
	if paneW < 1 {
		paneW = 1
	}
	if paneH < 1 {
		paneH = 1
	}

	return Layout{
		W: w, H: h,
		SidebarW:     sidebarW,
		SidebarTotal: sidebarTotal,
		PaneW:        paneW,
		PaneH:        paneH,
		PaneScreenX:  sidebarTotal + 1, // past sidebar box + pane's left border
		PaneScreenY:  1,                // past pane's top border
	}
}

// showLegend reports whether the color key fits below the session list without
// pushing any session or the bottom hint out of the box. The legend lives below
// the list, so on short or session-heavy terminals it yields and the list stays
// fully visible. RenderSidebar consults this so what's drawn and where clicks
// land can never disagree.
func (l Layout) showLegend(count int) bool {
	contentH := l.H - boxBorder
	need := sidebarHeader + legendHeight() + count*linesPerSession + 1 // +1 for the hint
	return contentH >= need
}

// sessionRow0 is the 0-based screen row of the first session entry. The legend
// now sits below the list, so the rows always start right after the header.
func (l Layout) sessionRow0() int {
	return 1 + sidebarHeader // past sidebar top border + header lines
}

// linesPerSession is how many sidebar rows one session occupies. RenderSidebar
// draws three rows per entry — topic, current task, and phase+context bar (see
// render.go) — so hit-testing must step by three to stay aligned.
const linesPerSession = 3

// SessionIndexAt returns the session row index a click landed on, or -1 if the
// click wasn't on a session entry in the sidebar.
func (l Layout) SessionIndexAt(x, y, count int) int {
	if x < 0 || x >= l.SidebarTotal {
		return -1
	}
	rel := y - l.sessionRow0()
	if rel < 0 {
		return -1
	}
	idx := rel / linesPerSession
	if idx >= count {
		return -1
	}
	return idx
}

// InPane reports whether a screen coordinate is inside the pane content area.
func (l Layout) InPane(x, y int) bool {
	return x >= l.PaneScreenX && x < l.PaneScreenX+l.PaneW &&
		y >= l.PaneScreenY && y < l.PaneScreenY+l.PaneH
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
