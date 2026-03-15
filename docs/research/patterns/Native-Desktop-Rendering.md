---
title: Native Desktop Rendering for AI Agent UIs
tags: [ai, agents, godot, tauri, egui, raylib, pixel-art, desktop, rendering]
type: reference
---

# Native Desktop Rendering for AI Agent UIs

## Stack Comparison

| Framework | Language | Terminal Embed | Pixel Art | Time to MVP | Best For |
|-----------|---------|---------------|-----------|------------|---------|
| **Godot 4** | GDScript / C# | godot-xterm (Linux/macOS) | Native | 3-4 weeks | Game-like agent worlds |
| **Tauri** | Rust + JS | xterm.js via webview | CSS pixel art | 2-3 weeks | Cross-platform, light bundle |
| **egui** | Rust | Custom IPC needed | Nearest-neighbor | 3-5 weeks | Pure native, no webview |
| **raylib** | C / Go / etc. | Custom PTY needed | Full control | 4-6 weeks | Maximum rendering control |
| **Bevy** | Rust (ECS) | Custom PTY needed | Sprite sheets | 6-8 weeks | Animated sprite agents |

---

## Pixel Art Rendering Principles

These apply regardless of framework:

1. **Nearest-neighbor scaling** — disable antialiasing; scale integer multiples (1×, 2×, 3×) only
2. **Low-res render target** — render to a small texture (e.g. 320×180), upscale to window
3. **Palette restriction** — limit to 16-64 colours for authentic look
4. **Monospace font** — bitmap fonts (e.g. Press Start 2P) reinforce the aesthetic
5. **CRT effects** — optional scanline/curvature shader (WGSL / GLSL post-process pass)

---

## Godot 4 + godot-xterm

The fastest path to an embedded terminal in a pixel-art native app.

**What it does**: full XTerm emulation (colours, cursor, scrollback) as a Godot node you place in your scene tree.

**Limitation**: The PTY node (which spawns real shells/agent processes) is **Linux and macOS only**. Windows gets display-only terminal rendering, not a live connection. Factor into cross-platform planning.

**Strengths**: visual editor for pixel-art scenes, proven in pixel-art productivity apps (Pixelorama, 8.5k ★).

---

## Tauri + xterm.js

Best when you want:
- Lightest deployment bundle
- Web-first aesthetics (CSS pixel art libraries: snes.css, nes.css)
- Quick cross-platform reach

**IPC**: Tauri's `invoke()` bridge between Rust backend and JS frontend. Use `tauri-plugin-pty` for shell/agent connections.

**Trade-off**: webview rendering is less consistent than native, and resize performance is slower than pure native.

---

## egui

Pure Rust, immediate-mode. No webview. Every frame is re-rendered from scratch.

Good for: debug panels, agent dashboards, tool windows.
Less good for: sprite-heavy pixel-art worlds, tilesets, animated characters. Combine with Bevy or wgpu for those.

**Pixel font caveat**: egui renders fonts via its own rasteriser — bitmap pixel fonts can appear blurry without workarounds.

---

## Bevy (ECS Game Engine)

Appropriate when agent characters need animation — walking, working, idle states as sprite sheet sequences. The ECS (Entity Component System) maps naturally to agent entities with component state.

Heavier investment (6-8 weeks to MVP) but best rendering performance and most flexible long-term.

---

## Terminal Embedding Summary

| Approach | Complexity | Cross-Platform |
|---------|------------|----------------|
| godot-xterm (Godot) | Low | Linux + macOS only (PTY) |
| tauri-plugin-pty + xterm.js | Medium | Full |
| Custom PTY in egui/raylib | High | Full |
| SSH into agents, display output | Medium | Full (indirect) |

---

*Authored by: Clault KiperS 4.6*
