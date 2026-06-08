package session

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jeffreyd/cmdr-keen/internal/hooks"
)

// Manager holds the fixed-order list of sessions and which one is active.
// It is only touched from the Bubble Tea update loop (single-threaded), except
// that sessions' own goroutines call the shared send func — so no locking here.
type Manager struct {
	sessions     []*Session
	active       int
	send         func(tea.Msg)
	hookCfg      *hooks.Config
	paneW, paneH int
}

func NewManager(send func(tea.Msg), hookCfg *hooks.Config) *Manager {
	return &Manager{send: send, hookCfg: hookCfg, active: -1, paneW: 80, paneH: 24}
}

// Spawn starts a new session at cwd and makes it active.
func (m *Manager) Spawn(cwd string, args []string) error {
	s, err := New(cwd, args, m.paneW, m.paneH, m.hookCfg, m.send)
	if err != nil {
		return err
	}
	m.sessions = append(m.sessions, s)
	m.setActive(len(m.sessions) - 1)
	return nil
}

func (m *Manager) Sessions() []*Session { return m.sessions }
func (m *Manager) Count() int           { return len(m.sessions) }
func (m *Manager) ActiveIndex() int     { return m.active }

func (m *Manager) Active() *Session {
	if m.active < 0 || m.active >= len(m.sessions) {
		return nil
	}
	return m.sessions[m.active]
}

func (m *Manager) SetActive(i int) { m.setActive(i) }

func (m *Manager) setActive(i int) {
	if i < 0 || i >= len(m.sessions) {
		return
	}
	for j, s := range m.sessions {
		if j == i {
			s.Focus()
		} else {
			s.Blur()
		}
	}
	m.active = i
}

func (m *Manager) Find(id string) *Session {
	for _, s := range m.sessions {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// Resize stores the pane size and applies it to every session.
func (m *Manager) Resize(paneW, paneH int) {
	m.paneW, m.paneH = paneW, paneH
	for _, s := range m.sessions {
		s.Resize(paneW, paneH)
	}
}

// Close kills and removes session i, keeping the active index sane.
func (m *Manager) Close(i int) {
	if i < 0 || i >= len(m.sessions) {
		return
	}
	m.sessions[i].Close()
	m.sessions = append(m.sessions[:i], m.sessions[i+1:]...)
	if m.active >= len(m.sessions) {
		m.active = len(m.sessions) - 1
	}
	if m.active >= 0 {
		m.setActive(m.active)
	}
}

func (m *Manager) MarkStatus(id string, st Status) {
	if s := m.Find(id); s != nil {
		s.Status = st
	}
}

func (m *Manager) SetTitle(id, title string) {
	if s := m.Find(id); s != nil && title != "" {
		s.Title = title
	}
}

// SetContextTokens records a session's reported context-window usage. Zero is
// treated as "no fresh reading" and left as-is, so an event without a token
// count never wipes a good value.
func (m *Manager) SetContextTokens(id string, tokens int) {
	if s := m.Find(id); s != nil && tokens > 0 {
		s.ContextTokens = tokens
	}
}
