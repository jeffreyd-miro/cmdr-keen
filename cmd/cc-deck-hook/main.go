// Command cc-deck-hook is a tiny Claude Code hook helper. Claude runs it at
// lifecycle events (e.g. `cc-deck-hook crunching`); it reports that event for
// the current keen session to keen's unix socket, then exits fast. It is a
// no-op when not launched under keen, and never blocks Claude.
package main

import (
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

	// Read Claude's hook payload from stdin (so it never sees a broken pipe)
	// and pull out the prompt text — present on UserPromptSubmit — which keen
	// uses to title the tab.
	var payload struct {
		Prompt string `json:"prompt"`
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

	msg := map[string]string{"session": sess, "event": event}
	if payload.Prompt != "" {
		msg["prompt"] = payload.Prompt
	}
	b, _ := json.Marshal(msg)
	_, _ = conn.Write(append(b, '\n'))
}
