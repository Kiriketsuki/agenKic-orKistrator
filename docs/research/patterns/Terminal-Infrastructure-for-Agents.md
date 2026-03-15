---
title: Terminal Infrastructure for AI Agents
tags: [ai, agents, tmux, terminal, tui, bubbletea, ratatui, wezterm]
type: reference
---

# Terminal Infrastructure for AI Agents

## tmux as Agent Substrate

tmux is the most proven tool for running AI agents as terminal processes (as of early 2026). Its server/session/window/pane model maps naturally to an orchestrator managing a fleet of agents.

### Key API

```bash
tmux new-session -d -s "agent-{name}"      # spawn detached agent session
tmux send-keys -t "agent-{name}" "cmd" Enter  # inject command (Enter separate!)
tmux capture-pane -t "agent-{name}" -p -S -500  # read last 500 lines of output
tmux list-sessions                          # enumerate running agents
```

**Critical**: always send `Enter` as a separate argument — appending `\n` is unreliable.

### Why tmux Works
- Sessions survive disconnects → agents run unattended
- Multiple clients can attach to same session → live observability
- Named sessions → natural agent registry
- Complete programmatic API → orchestrators can control everything via shell

### Watch: Zellij
Approaching v1.0 with a plugin system that may surpass tmux's scripting model for agent orchestration use cases.

---

## Window Positioning

### X11 (mature)
- `xdotool` + `wmctrl` — pixel-precise window placement
- `xterm -geometry 100x35+200+20` — position at spawn time
- `i3-msg` — tiling/floating control for i3 WM

### Wayland (fragmented)
- No universal positioning API — each compositor (Sway, Hyprland, River) has its own IPC
- Sway: i3-compatible `swaymsg` — most portable Wayland option
- Hyprland: IPC socket with JSON commands

### Embedded (best for custom UIs)
- **WezTerm** Lua API: `pane:split()`, `window:set_position()` — programmatic layout at startup
- **xterm.js** in a webview (Tauri/Electron) — full control, cross-platform
- For a custom desktop app, embed the terminal inside the app rather than positioning OS windows

---

## TUI Chat Frameworks

For rendering agents as chat panels in a terminal:

| Framework | Language | Model | Best For |
|-----------|---------|-------|---------|
| **Bubbletea** | Go | Elm (unidirectional) | Production chat UIs, proven ecosystem |
| **Ratatui** | Rust | Immediate-mode | High-performance, minimal deps |
| **Textual** | Python | Reactive + async | Rapid prototyping, asyncio-native |

### Anti-Flicker Pattern
Never clear the full screen (`ESC[2J`) on streaming updates — causes 4k-6k scroll events/sec in tmux. Instead:
- Erase line-by-line (`ESC[2K` per line)
- Buffer output with 16ms flush intervals (~60 FPS)
- Enable tmux synchronized output: `set-option -g allow-passthrough on`

### Multi-Pane Agent Layout
Each agent gets its own scroll position, message history, and input buffer. Use constraint-based layout (Ratatui) or focus system (Tab switches panes) for a multi-agent chat view.

---

## Inline Images in Terminal

For agent avatars or status icons:

| Protocol | Support |
|---------|---------|
| Kitty graphics | kitty, WezTerm |
| iTerm2 protocol | WezTerm, iTerm2, many modern terminals |
| Sixel | Wider legacy support |
| Unicode blocks (█ ▓ ▒ ░) | Universal — works everywhere |

Detection: check `$TERM` env and `$ITERM_SESSION_ID`.

---

*Authored by: Clault KiperS 4.6*
