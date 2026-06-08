// Spike A — raw PTY passthrough.
//
// Purpose: prove FULL Claude Code fidelity through a thin wrapper. Because we
// copy bytes verbatim in both directions and never parse input, everything that
// works in a bare terminal works here — including the two hard requirements:
// image paste (Ctrl-V → byte 0x16 reaches Claude, which reads the OS clipboard)
// and macOS voice-to-text (dictation arrives as ordinary input bytes).
//
// The ONLY thing we intercept is the keen prefix, Ctrl-K (byte 0x0B). In the
// real app that pops the sidebar/switcher; here it just detaches cleanly so we
// can confirm the prefix is caught before Claude ever sees it.
//
// This is also the "overlay-switcher" engine from docs/spec.md: maximally
// faithful by construction. Run it, then exercise the fidelity checklist that
// main() prints on exit.
//
//	go run ./spike/passthrough            # wraps `claude` in $PWD
//	go run ./spike/passthrough -- echo hi # wraps an arbitrary command (plumbing test)
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

const prefixByte = 0x0b // Ctrl-K

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "\r\n[keen-spike] error: %v\r\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Command to wrap: everything after `--`, else `claude`.
	args := []string{"claude"}
	for i, a := range os.Args {
		if a == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	if len(args) == 0 {
		args = []string{"claude"}
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		// In the real app: KEEN_SESSION / KEEN_SOCKET injected here.
	)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("start pty: %w", err)
	}
	defer func() { _ = ptmx.Close() }()

	// Keep the child PTY the same size as our terminal, including the first
	// resize right now so Claude lays out correctly on launch.
	resizes := make(chan os.Signal, 1)
	signal.Notify(resizes, syscall.SIGWINCH)
	go func() {
		for range resizes {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	resizes <- syscall.SIGWINCH

	// Raw mode so keystrokes (incl. Ctrl-V, control sequences, paste payloads)
	// reach Claude untouched. Skip gracefully when stdin isn't a TTY (e.g. the
	// `-- echo hi` plumbing test under a pipe).
	stdinFd := int(os.Stdin.Fd())
	var restore *term.State
	if term.IsTerminal(stdinFd) {
		if restore, err = term.MakeRaw(stdinFd); err != nil {
			return fmt.Errorf("make raw: %w", err)
		}
		defer func() { _ = term.Restore(stdinFd, restore) }()
	}

	// PTY output -> our stdout, verbatim. When the child exits, EOF here ends
	// the copy and we fall through to wait+report.
	go func() { _, _ = io.Copy(os.Stdout, ptmx) }()

	// Our stdin -> PTY, verbatim, except we steal the Ctrl-K prefix.
	captured := forwardInput(os.Stdin, ptmx)

	_ = cmd.Wait()

	if restore != nil {
		_ = term.Restore(stdinFd, restore)
	}
	printChecklist(args, captured)
	return nil
}

// forwardInput copies stdin to the PTY byte-for-byte until it sees the prefix
// byte (Ctrl-K), at which point it stops (the real app would instead pop the
// switcher). Returns the number of prefix presses seen. Anything before the
// prefix in the same chunk is still forwarded, preserving fidelity.
func forwardInput(in io.Reader, pty io.Writer) int {
	buf := make([]byte, 4096)
	prefixes := 0
	for {
		n, err := in.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if i := indexByte(chunk, prefixByte); i >= 0 {
				if i > 0 {
					_, _ = pty.Write(chunk[:i]) // forward bytes before the prefix
				}
				prefixes++
				return prefixes
			}
			if _, werr := pty.Write(chunk); werr != nil {
				return prefixes
			}
		}
		if err != nil {
			return prefixes
		}
	}
}

func indexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}

func printChecklist(args []string, prefixes int) {
	fmt.Printf("\r\n[keen-spike] detached (saw %d Ctrl-K prefix press) — wrapped: %v\r\n", prefixes, args)
	fmt.Print("\r\n" + `M0 fidelity checklist — run with real `+"`claude`"+` and confirm each:
  [ ] type a prompt and get a reply (basic I/O + colors)
  [ ] trigger a permission prompt and approve it
  [ ] edit a file, scroll history
  [ ] resize this terminal — Claude reflows to the new size
  [ ] PASTE AN IMAGE into the prompt (Ctrl-V)         <-- hard requirement
  [ ] DICTATE text via macOS voice-to-text            <-- hard requirement
  [ ] Ctrl-K is caught by keen, never reaches Claude  <-- (this exit proves it)
` + "\r\n")
}
