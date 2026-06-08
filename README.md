# keen

A thin multiplexer for Claude Code. Run many `claude` sessions behind one
screen: a fixed-order sidebar showing each session's name and a live status
color (crunching / waiting on you / all done), plus an embedded pane for the
active session. Keystrokes pass straight through to Claude — keen only adds the
list, the names, and the statuses.

See [`docs/spec.md`](docs/spec.md) for the full design and milestones.

## Build

Build **both** binaries into `bin/` — keen locates the hook helper next to
itself:

```sh
go build -o bin/keen ./cmd/keen
go build -o bin/cc-deck-hook ./cmd/cc-deck-hook
```

## Run

```sh
./bin/keen          # one session: `claude --permission-mode auto` in $PWD
./bin/keen -- bash  # wrap an arbitrary command instead (handy for testing)
```

Each session is spawned with hooks injected via `claude --settings <tempfile>`,
so your global `~/.claude` is never modified.

## Controls

| Action | Key |
|---|---|
| Toggle focus: sidebar ⇄ session | **Ctrl-K** (or **Cmd-K**, see setup below) |
| Move selection (sidebar focused) | `j`/`k` or ↑/↓ |
| Jump into the session | `Enter` |
| New session | `n` |
| Close session | `x` |
| Jump to session N | `1`–`9` |
| Quit keen | `q` |
| Switch to a session | click its row |

When the session is focused, everything (typing, paste, mouse scroll/click) goes
straight to Claude. Only the prefix key is intercepted.

## Status colors

| Glyph | Meaning | Hook event |
|---|---|---|
| `·` grey | starting | `SessionStart` |
| `●` yellow | crunching | `UserPromptSubmit`, `PreToolUse` |
| `◐` red | waiting on you | `Notification` |
| `✓` green | all done (your move) | `Stop` |
| `✕` faint | exited | `SessionEnd` |

## Setup: make Cmd-K work too (macOS / Cursor / VS Code)

keen's prefix is **Ctrl-K**. To also trigger it with **Cmd-K**, the terminal
must be told to send the Ctrl-K byte on Cmd-K — macOS terminals swallow the Cmd
modifier, so a terminal program can't see Cmd-K on its own.

Add this to your Cursor/VS Code `keybindings.json`
(`Cmd-Shift-P` → *Preferences: Open Keyboard Shortcuts (JSON)*):

```jsonc
{
  "key": "cmd+k",
  "command": "workbench.action.terminal.sendSequence",
  "args": { "text": "\u000b" },   // \u000b = Ctrl-K
  "when": "terminalFocus"
}
```

Trade-offs while the terminal is focused: this overrides Cmd-K's default
"clear terminal" and any Cmd-K chord shortcuts, and in a non-keen shell Cmd-K
will send Ctrl-K (kill-to-end-of-line). It only applies when the terminal has
focus.

> On macOS this file lives at
> `~/Library/Application Support/Cursor/User/keybindings.json` (Cursor) or
> `.../Code/User/keybindings.json` (VS Code).

## Layout

```
cmd/keen/            entry point
cmd/cc-deck-hook/    tiny Claude Code hook helper (reports status to keen)
internal/session/    one claude process: PTY + terminal emulator + lifecycle
internal/ui/         Bubble Tea model, sidebar/pane render, key + mouse input
internal/hooks/      unix-socket status server + per-session settings generation
internal/titler/     turns a session's first prompt into a short Haiku tab title
spike/               M0 de-risk spikes (passthrough vs embedded-vt)
```
