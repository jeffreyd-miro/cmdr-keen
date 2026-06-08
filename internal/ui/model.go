package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jeffreyd/cmdr-keen/internal/hooks"
	"github.com/jeffreyd/cmdr-keen/internal/session"
	"github.com/jeffreyd/cmdr-keen/internal/titler"
)

// titleMsg carries a generated tab title back to the update loop.
type titleMsg struct {
	ID    string
	Title string
}

// Model is the top-level Bubble Tea model: a fixed-order list of sessions and
// an embedded pane for the active one. Focus is either the pane (keystrokes go
// to Claude — the default) or the sidebar (keystrokes navigate). Ctrl-K toggles.
type Model struct {
	mgr          *session.Manager
	layout       Layout
	sidebarFocus bool
	ready        bool

	initialCwd  string
	initialArgs []string
}

func NewModel(initialCwd string, initialArgs []string) *Model {
	return &Model{initialCwd: initialCwd, initialArgs: initialArgs}
}

// SetManager wires the session manager (created with the program's send func)
// after tea.NewProgram but before Run.
func (m *Model) SetManager(mgr *session.Manager) { m.mgr = mgr }

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.layout = ComputeLayout(msg.Width, msg.Height)
		m.ready = true
		m.mgr.Resize(m.layout.PaneW, m.layout.PaneH)
		if m.mgr.Count() == 0 { // spawn the first session at the real pane size
			_ = m.mgr.Spawn(m.initialCwd, m.initialArgs)
		}
		return m, nil

	case tea.KeyMsg:
		return m, m.handleKey(msg)

	case tea.MouseMsg:
		m.handleMouse(msg)
		return m, nil

	case session.OutputMsg:
		return m, nil // a session painted; Bubble Tea will re-render the view

	case session.ExitMsg:
		m.mgr.MarkStatus(msg.ID, session.StatusExited)
		return m, nil

	case hooks.StatusEventMsg:
		if st, ok := statusForEvent(msg.Event); ok {
			m.mgr.MarkStatus(msg.Session, st)
		}
		m.mgr.SetContextTokens(msg.Session, msg.Tokens)
		return m, m.maybeTitle(msg)

	case titleMsg:
		m.mgr.SetTitle(msg.ID, msg.Title)
		return m, nil
	}
	return m, nil
}

// maybeTitle kicks off (once per session) a Haiku title from the first prompt.
// Runs off the UI thread as a tea.Cmd; falls back to a heuristic on error.
func (m *Model) maybeTitle(msg hooks.StatusEventMsg) tea.Cmd {
	if msg.Prompt == "" {
		return nil
	}
	s := m.mgr.Find(msg.Session)
	if s == nil || s.Title != "" || s.TitleRequested {
		return nil
	}
	s.TitleRequested = true
	id, prompt := msg.Session, msg.Prompt
	return func() tea.Msg {
		title, err := titler.Generate(prompt)
		if err != nil || title == "" {
			title = titler.Heuristic(prompt)
		}
		return titleMsg{ID: id, Title: title}
	}
}

// statusForEvent maps a hook event name to a sidebar status. "start" is
// intentionally unmapped — a freshly launched session stays neutral until the
// first prompt makes it crunch.
func statusForEvent(event string) (session.Status, bool) {
	switch event {
	case "crunching":
		return session.StatusCrunching, true
	case "waiting":
		return session.StatusWaiting, true
	case "done":
		return session.StatusDone, true
	case "exit":
		return session.StatusExited, true
	}
	return 0, false
}

func (m *Model) handleKey(k tea.KeyMsg) tea.Cmd {
	if k.Type == tea.KeyCtrlK { // the keen prefix — never reaches Claude
		m.sidebarFocus = !m.sidebarFocus
		return nil
	}
	if m.sidebarFocus {
		return m.handleSidebarKey(k)
	}
	if s := m.mgr.Active(); s != nil {
		s.Write(KeyToBytes(k))
	}
	return nil
}

func (m *Model) handleSidebarKey(k tea.KeyMsg) tea.Cmd {
	rune1 := ""
	if k.Type == tea.KeyRunes && len(k.Runes) == 1 {
		rune1 = string(k.Runes[0])
	}
	switch {
	case k.Type == tea.KeyEnter:
		m.sidebarFocus = false // hand control back to the session
	case k.Type == tea.KeyUp, rune1 == "k":
		m.mgr.SetActive(m.mgr.ActiveIndex() - 1)
	case k.Type == tea.KeyDown, rune1 == "j":
		m.mgr.SetActive(m.mgr.ActiveIndex() + 1)
	case rune1 == "n":
		_ = m.mgr.Spawn(m.initialCwd, m.initialArgs)
		m.sidebarFocus = false
	case rune1 == "x":
		m.mgr.Close(m.mgr.ActiveIndex())
	case rune1 == "q", k.Type == tea.KeyCtrlC:
		return tea.Quit
	case rune1 >= "1" && rune1 <= "9":
		m.mgr.SetActive(int(k.Runes[0] - '1'))
	}
	return nil
}

func (m *Model) handleMouse(ms tea.MouseMsg) {
	// Click on a sidebar row → switch to it and hand control to the session.
	if idx := m.layout.SessionIndexAt(ms.X, ms.Y, m.mgr.Count()); idx >= 0 {
		if ms.Action == tea.MouseActionPress {
			m.mgr.SetActive(idx)
			m.sidebarFocus = false
		}
		return
	}
	// Otherwise forward mouse (scroll/click/drag) into the active session.
	if m.layout.InPane(ms.X, ms.Y) {
		if s := m.mgr.Active(); s != nil {
			if b := MouseToSGR(ms, m.layout); b != nil {
				s.Write(b)
			}
		}
	}
}

func (m *Model) View() string {
	if !m.ready {
		return "starting keen…"
	}
	sidebar := RenderSidebar(m.layout, m.mgr.Sessions(), m.mgr.ActiveIndex(), m.sidebarFocus)
	pane := RenderPane(m.layout, m.mgr.Active(), !m.sidebarFocus)
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, pane)
}
