package titler

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
)

// TranscriptTail reads a Claude Code JSONL transcript and returns a compact,
// human-readable digest of the conversation — user prompts and assistant prose,
// with tool calls, tool results, and other noise dropped — capped to the last
// maxChars. This is what tells the titler where the work is *now*, so a session
// that has changed tracks gets relabeled. Best-effort: returns "" if the file
// can't be read or holds no prose yet.
func TranscriptTail(path string, maxChars int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	var turns []string
	sc := bufio.NewScanner(f)
	// Transcript lines hold whole messages and can be large (default cap 64 KiB).
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		var e struct {
			Type    string `json:"type"`
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		var who string
		switch e.Type {
		case "user":
			who = "user"
		case "assistant":
			who = "assistant"
		default:
			continue
		}
		if text := strings.TrimSpace(extractText(e.Message.Content)); text != "" {
			turns = append(turns, who+": "+text)
		}
	}

	joined := strings.Join(turns, "\n")
	if maxChars > 0 && len(joined) > maxChars {
		joined = joined[len(joined)-maxChars:]
	}
	return joined
}

// extractText pulls plain text out of a transcript message's content, which is
// either a JSON string (a typed user prompt) or an array of typed blocks (an
// assistant turn, or a tool_result-bearing user turn). Only text blocks are
// kept, so tool calls and results contribute nothing.
func extractText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		var b strings.Builder
		for _, bl := range blocks {
			if bl.Type == "text" && bl.Text != "" {
				if b.Len() > 0 {
					b.WriteByte(' ')
				}
				b.WriteString(bl.Text)
			}
		}
		return b.String()
	}
	return ""
}
