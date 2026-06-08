package hooks

import (
	"bufio"
	"encoding/json"
	"net"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// Event is one status update reported by a hook helper over the socket.
type Event struct {
	Session    string `json:"session"`
	Event      string `json:"event"`
	Prompt     string `json:"prompt,omitempty"`     // latest user prompt, for titling
	Tokens     int    `json:"tokens,omitempty"`     // input-side context tokens in use
	Transcript string `json:"transcript,omitempty"` // transcript path, for re-summarizing
}

// StatusEventMsg is delivered into the Bubble Tea program for each hook event.
type StatusEventMsg Event

// Server is keen's unix-socket listener for hook events.
type Server struct {
	ln   net.Listener
	path string
	send func(tea.Msg)
}

// NewServer binds the socket (clearing any stale one first).
func NewServer(path string, send func(tea.Msg)) (*Server, error) {
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	return &Server{ln: ln, path: path, send: send}, nil
}

// Serve accepts connections until Close. Run it in a goroutine.
func (s *Server) Serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return // listener closed
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		var e Event
		if err := json.Unmarshal(sc.Bytes(), &e); err == nil && e.Session != "" {
			s.send(StatusEventMsg(e))
		}
	}
}

// Close stops the listener and removes the socket file.
func (s *Server) Close() {
	_ = s.ln.Close()
	_ = os.Remove(s.path)
}
