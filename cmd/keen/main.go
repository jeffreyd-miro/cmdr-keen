// Command keen is a thin multiplexer for Claude Code: a fixed-order sidebar of
// sessions with status indicators, plus an embedded pane for the active one.
// Ctrl-K toggles focus between the sidebar and the session.
//
//	keen              # one session running `claude` in the current directory
//	keen -- bash      # wrap an arbitrary command instead (handy for testing)
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jeffreyd/cmdr-keen/internal/hooks"
	"github.com/jeffreyd/cmdr-keen/internal/session"
	"github.com/jeffreyd/cmdr-keen/internal/ui"
)

func main() {
	// Command to run per session: everything after `--`, else the default
	// claude invocation.
	args := []string{"claude", "--permission-mode", "auto"}
	for i, a := range os.Args {
		if a == "--" {
			args = os.Args[i+1:]
			break
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	model := ui.NewModel(cwd, args)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Status engine: keen listens on a socket; each claude session reports
	// lifecycle events back to it via the cc-deck-hook helper.
	hookCfg := &hooks.Config{Socket: hooks.DefaultSocketPath(), HookBin: hooks.ResolveHookBin()}
	srv, err := hooks.NewServer(hookCfg.Socket, p.Send)
	if err != nil {
		fmt.Fprintln(os.Stderr, "keen: status socket:", err)
		os.Exit(1)
	}
	defer srv.Close()
	go srv.Serve()

	model.SetManager(session.NewManager(p.Send, hookCfg))

	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "keen:", err)
		os.Exit(1)
	}
}
