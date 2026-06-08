# M0 spike — proving full Claude Code fidelity through a thin wrapper

Two harnesses that bracket the architecture decision in [`../docs/spec.md`](../docs/spec.md).
Both wrap a child process in a PTY and intercept the keen prefix **Ctrl-K**; they
differ in how the session is rendered and — critically — how input reaches Claude.

| | Spike A · `passthrough` | Spike B · `embed-vt` |
|---|---|---|
| Engine | raw byte passthrough, no framework | Bubble Tea + `charmbracelet/x/vt` |
| Input path | **verbatim bytes** → PTY | tea key parse → **reconstructed** bytes → PTY |
| Sidebar next to live session? | no (full-screen; sidebar is a popup on Ctrl-K) | **yes** (sidebar + live pane on one screen) |
| Image paste / dictation | works by construction | **must be verified** (the open question) |
| Maps to spec layout | overlay-switcher (fallback) | always-on sidebar (preferred) |

## Run

```sh
# Spike A — perfect-fidelity passthrough (the overlay engine)
go run ./spike/passthrough              # wraps `claude` in $PWD
go run ./spike/passthrough -- bash      # wrap something else

# Spike B — embedded VT pane with a stub sidebar
go run ./spike/embed-vt                 # wraps `claude` in $PWD
go run ./spike/embed-vt -- bash
```

Press **Ctrl-K** to detach (A) / quit (B) — proving the prefix is caught before
Claude sees it.

## Fidelity checklist (run each with a REAL `claude` in both spikes)

The first four usually pass anywhere; the **last two are the hard requirements**
that decide the architecture.

- [ ] type a prompt, get a reply (basic I/O + truecolor)
- [ ] trigger a permission prompt and approve it
- [ ] edit a file, scroll history
- [ ] resize the terminal — Claude reflows to the new size
- [ ] **paste an image into the prompt (Ctrl-V)**
- [ ] **dictate text via macOS voice-to-text**
- [ ] Ctrl-K caught by keen, never reaches Claude

## Decision rule

- **Spike B passes image paste + dictation** → build the **always-on sidebar**
  (your preferred layout); Spike B's `keyToBytes`/mouse forwarding becomes the
  input layer to harden.
- **Spike B fails either** → ship the **overlay-switcher** on Spike A's engine
  (perfect fidelity guaranteed), with the Charm sidebar as a popup on Ctrl-K.

## Findings (M0 verdict: both viable — embedded path chosen)

- ✅ Go 1.26, `creack/pty`, Bubble Tea v1.3.10, Lipgloss v1.1.0 all wired.
- ✅ Passthrough plumbing verified: child PTY output flows through, `TERM`/
  `COLORTERM` injected, Ctrl-K intercepted, clean exit.
- ✅ **Embedded-vt is interactive** once the emulator's query replies are pumped
  back to the PTY (`go io.Copy(ptmx, emu)`). Without that pump Claude stalls at
  startup waiting for capability/cursor responses and you can't type.
- ✅ **Image input works in both** via drag-and-drop. Passthrough → native
  `[image 1]` attachment; embedded → was a raw file path until we re-wrapped
  pastes in bracketed-paste markers (`ESC[200~…ESC[201~`) — now ingests natively.
- ℹ️ **Ctrl-V image paste blocked by iTerm/corp clipboard permissions** —
  environmental, not a wrapper issue. Drag-and-drop is the working path.
- ⚠️ **`charmbracelet/x/vt` dependency skew:** default `go get` pulled a `cellbuf`
  incompatible with the resolved `ansi`. Fixed by pinning `x/cellbuf@latest` +
  `x/ansi@latest` + `ultraviolet@latest` (see `go.mod`). Package self-describes
  as experimental — a maturity risk to keep watching on the embedded path.

**Decision:** the embedded-vt path (your preferred always-on sidebar) is
interactive and image-capable, so M1 builds on it. Input-layer hardening still
owed: full key/mouse byte coverage in `keyToBytes`, and keep the bracketed-paste
wrap. Passthrough/overlay stays in pocket as the zero-risk fallback.
