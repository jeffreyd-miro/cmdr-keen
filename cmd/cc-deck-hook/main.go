// Command cc-deck-hook is a tiny Claude Code hook helper. Claude runs it at
// lifecycle events (e.g. `cc-deck-hook crunching`); it reports that event for
// the current keen session to keen's unix socket, then exits fast. It is a
// no-op when not launched under keen, and never blocks Claude.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		return
	}
	event := os.Args[1]

	// Read Claude's hook payload from stdin (so it never sees a broken pipe).
	// We pull out the prompt text — present on UserPromptSubmit, used to title
	// the tab — and the transcript path, which we read to estimate how much of
	// the context window the session has burned.
	var payload struct {
		Prompt         string `json:"prompt"`
		TranscriptPath string `json:"transcript_path"`
	}
	if in, err := io.ReadAll(os.Stdin); err == nil {
		_ = json.Unmarshal(in, &payload)
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
		Session string `json:"session"`
		Event   string `json:"event"`
		Prompt  string `json:"prompt,omitempty"`
		Tokens  int    `json:"tokens,omitempty"`
	}{Session: sess, Event: event, Prompt: payload.Prompt}
	if payload.TranscriptPath != "" {
		msg.Tokens = contextTokens(payload.TranscriptPath)
	}
	b, _ := json.Marshal(msg)
	_, _ = conn.Write(append(b, '\n'))
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
