---
tags: [research, terminal, window-management, wayland, x11, orchestration]
date: 2026-03-14
topic: Repositionable & Floating Terminal Windows
---

# Repositionable Terminals & Window Management

## Three Viable Architectures

### 1. Desktop App (Best UX)
Embedded xterm.js in Electron/Tauri — full control over layout, cross-platform, CSS Grid for positioning. Best choice when building a dedicated orchestrator UI.

### 2. Terminal Native (Low Effort)
WezTerm Lua config or Kitty + i3-msg. Can programmatically create a 2×4 grid of panes at startup, launch agents in each. Works X11 + Wayland. No runtime repositioning needed.

### 3. Headless Backend
tmux + remote UI via SSH. Persistent sessions, debuggable, SSH-friendly. Visual management via web (ttyd).

## X11 vs Wayland

### X11 (Mature)
- `wmctrl -r <title> -e "0,X,Y,W,H"` — move/resize by window title
- `xdotool windowmove <id> X Y` — programmatic placement
- `xterm -geometry 100x35+200+20` — launch at specific position (cols×rows+x+y)
- `i3-msg '[title="agent-1"] move to workspace 2'`
- Full EWMH hint ecosystem

### Wayland (Fragmented)
- No universal positioning API — each compositor is isolated
- **Hyprland**: IPC socket at `$HYPRLAND_INSTANCE_SIGNATURE`
- **Sway**: `swaymsg` with i3-compatible rules but smaller tooling
- **River**: `riverctl` for rule-based placement
- Recommendation: stick with Sway, or use embedded terminals instead

## WezTerm Lua API (Strongest for Orchestration)

```lua
-- Create 2x4 agent grid from config
wezterm.on("gui-startup", function()
  local tab, pane, window = mux.spawn_window({})
  for i = 1, 7 do
    pane:split({ direction = i % 2 == 0 and "Right" or "Bottom" })
  end
end)
```

Can programmatically split panes, get pane IDs, send input, capture output — all from Lua. Cross-platform (X11, Wayland, macOS, Windows).

## IPC Sockets for Live Control

- **Kitty**: `--listen-on unix:/tmp/kitty-{pid}` → `kitty @ set-window-title`, `kitty @ new-window`
- **Alacritty**: `alacritty msg create-window` — limited API
- Both allow orchestrators to control running instances without spawning new processes

## Key Insight

> Terminal multiplexers (tmux, WezTerm) divide a single terminal into panes with shared context. Window managers (i3, Sway, Hyprland) position independent windows. An orchestrator likely needs both: multiplexer for layout within the app + window manager for OS-level positioning.

For an agent workspace, prefer **embedded terminals** (desktop app with xterm.js) over trying to position OS windows — you get a single coordinate system, clipboard sharing, and full layout control.

---
*Authored by: Clault KiperS 4.6*
