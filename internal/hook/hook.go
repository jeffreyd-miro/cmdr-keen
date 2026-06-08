// Package hook implements keen's Claude Code hook helper. Claude runs it at
// lifecycle events (e.g. `keen __hook crunching`, or the standalone
// `cc-deck-hook crunching`); it reports that event for the current keen session
// to keen's unix socket, then exits fast. It is a no-op when not launched under
// keen, and never blocks Claude.
//
// The same code backs two entry points: the hidden `keen __hook` subcommand
// (so a single `go install` of keen is self-contained) and the standalone
// cc-deck-hook binary (kept for anyone wiring hooks by hand).
package hook

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

// Run executes the hook helper. args is the argument list after the event
// selector — args[0] is the lifecycle event (start, crunching, waiting, …).
// It reads Claude's hook payload from stdin and never returns an error: a hook
// must not block or break Claude, so every failure path is a silent return.
func Run(args []string) {
	if len(args) < 1 {
		return
	}
	event := args[0]

	// Read Claude's hook payload from stdin (so it never sees a broken pipe).
	// We pull out the prompt text — present on UserPromptSubmit, used to title
	// the tab — and the transcript path, which we read to estimate how much of
	// the context window the session has burned.
	var payload struct {
		Prompt         string `json:"prompt"`
		TranscriptPath string `json:"transcript_path"`
		Message        string `json:"message"`
	}
	if in, err := io.ReadAll(os.Stdin); err == nil {
		_ = json.Unmarshal(in, &payload)
	}

	// The Notification hook fires for two unrelated situations, which keen shows
	// in different colors: a mid-turn permission/attention request (red) and the
	// ~60s idle timeout "Claude is waiting for your input" (magenta — it pings
	// you after a finished turn sat untouched). Distinguish by the message text
	// and re-tag the idle flavor as its own event. Wording is Claude-version
	// dependent; keen's MarkStatus has a Done-precedence backstop if it drifts.
	if event == "waiting" && isIdleNotification(payload.Message) {
		event = "idle"
	}

	sock := os.Getenv("KEEN_SOCKET")
	sess := os.Getenv("KEEN_SESSION")
	if sock == "" || sess == "" {
		return // not running under keen
	}

	conn, err := net.DialTimeout("unix", sock, 500*time.Millisecond)
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))

	msg := struct {
		Session    string `json:"session"`
		Event      string `json:"event"`
		Prompt     string `json:"prompt,omitempty"`
		Tokens     int    `json:"tokens,omitempty"`
		Transcript string `json:"transcript,omitempty"`
	}{Session: sess, Event: event, Prompt: payload.Prompt, Transcript: payload.TranscriptPath}
	if payload.TranscriptPath != "" {
		msg.Tokens = contextTokens(payload.TranscriptPath)
	}
	b, _ := json.Marshal(msg)
	_, _ = conn.Write(append(b, '\n'))
}

// isIdleNotification reports whether a Notification message is the prompt-idle
// timeout (which keen ignores) rather than a permission/attention request
// (which keen surfaces as red). Claude's idle message is "Claude is waiting for
// your input"; matching the stable "waiting for your input" tail keeps this
// resilient to minor wording changes. Match is case-insensitive.
func isIdleNotification(message string) bool {
	return strings.Contains(strings.ToLower(message), "waiting for your input")
}

// contextTokens reads the session transcript and returns the input-side token
// footprint of the most recent assistant turn — input + cache-creation +
// cache-read — which is what currently occupies the context window (and what
// Claude's own status line reports as total_input_tokens). Returns 0 if the
// transcript can't be read or has no usage yet; the hook never fails over it.
func contextTokens(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	type usage struct {
		InputTokens              int `json:"input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	}
	var latest int

	sc := bufio.NewScanner(f)
	// Transcript lines hold whole messages and can be large; give the scanner
	// room (default cap is 64 KiB).
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		// Cheap pre-filter before the JSON parse.
		if !bytes.Contains(line, []byte(`"assistant"`)) {
			continue
		}
		var e struct {
			Type    string `json:"type"`
			Message struct {
				Usage usage `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &e) != nil || e.Type != "assistant" {
			continue
		}
		u := e.Message.Usage
		if sum := u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens; sum > 0 {
			latest = sum // keep the last assistant turn with usage
		}
	}
	return latest
}
