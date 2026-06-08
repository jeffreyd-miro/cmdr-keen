package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jeffreyd/cmdr-keen/internal/session"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true)
	hintStyle   = lipgloss.NewStyle().Faint(true)
	activeStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))

	focusBorder   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("12"))
	unfocusBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
)

// statusGlyph returns a single colored cell for a session's status.
func statusGlyph(st session.Status) string {
	switch st {
	case session.StatusCrunching:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render("●") // yellow
	case session.StatusWaiting:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("◐") // red
	case session.StatusDone:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("✓") // green
	case session.StatusExited:
		return hintStyle.Render("✕")
	default:
		return hintStyle.Render("·")
	}
}

// RenderSidebar draws the fixed-order session list, full terminal height.
func RenderSidebar(l Layout, sessions []*session.Session, active int, focused bool) string {
	contentH := l.H - boxBorder
	lines := make([]string, 0, contentH)

	lines = append(lines, titleStyle.Render("sessions"), "")

	for i, s := range sessions {
		marker := "  "
		if i == active {
			marker = "› "
		}
		// One line per session: the Haiku title once we know what the session is
		// about, or a "freshie" placeholder until that title arrives. The working
		// directory is no longer shown — it's the same for every session.
		label := s.Title
		if label == "" {
			label = "freshie"
		}
		label = truncate(label, l.SidebarW-4) // marker(2)+glyph(1)+space(1)
		if i == active {
			label = activeStyle.Render(label)
		}
		lines = append(lines, marker+statusGlyph(s.Status)+" "+label)
	}

	// Pin the hint to the bottom of the box.
	hint := hintStyle.Render("⌃k list · n new · x close")
	used := len(lines) + 1
	for used < contentH {
		lines = append(lines, "")
		used++
	}
	lines = append(lines, hint)

	box := unfocusBorder
	if focused {
		box = focusBorder
	}
	return box.Width(l.SidebarW).Height(contentH).Render(strings.Join(lines, "\n"))
}

// RenderPane draws the active session's emulated screen, or a placeholder.
func RenderPane(l Layout, s *session.Session, focused bool) string {
	box := unfocusBorder
	if focused {
		box = focusBorder
	}
	body := ""
	if s == nil {
		body = hintStyle.Render("no sessions — press ⌃k then n")
	} else {
		body = s.Render()
	}
	return box.Width(l.PaneW).Height(l.PaneH).Render(body)
}

// truncate shortens s to at most w visible columns, adding an ellipsis.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	return string(r[:w-1]) + "…"
}
