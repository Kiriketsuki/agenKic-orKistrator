# Pixelated / Retro Office UI Desktop App Frameworks
## Comprehensive Research Report
**Date**: 2026-03-14
**Context**: Evaluating native desktop app frameworks for rendering pixel-art/retro office UIs with agent terminal panels

---

## Executive Summary

Building a "pixelated office" world with embedded agent terminal panels requires balancing:
- **Native rendering** (performance, pixel-perfect control)
- **Terminal integration** (XTerm emulation, PTY management)
- **Rapid prototyping** (visual editor vs. code-first)
- **Cross-platform distribution** (single binary preferred)

**Primary Recommendation: Godot 4.x** — mature terminal support via godot-xterm, proven in pixel-art productivity tools (Pixelorama), fast iteration.

**Secondary Path: Tauri + CSS pixel art** — lightweight webview, mature CSS pixel-art libraries (Pixelact, snes.css), straightforward JavaScript/React UI.

**Performance-focused Path: raylib + Go** — proven by MasterPlan project management UI, excellent sprite/tilemap support, minimal dependencies.

---

## 1. Tauri: Rust Backend + Webview

### Architecture
- **Webview**: Platform-native engines (WebView2 on Windows, WKWebView on macOS, WebKit2GTK on Linux)
- **IPC**: Custom URI schemes (`ipc://`, `tauri://`) with ~0.5ms latency per invoke
- **Events**: Pub/sub pattern for multi-producer state changes
- **No bundled browser**: <1MB app bundles, significantly smaller than Electron

### Pixel-Art CSS Aesthetics
**Mature CSS frameworks for retro UI:**
- **Pixelact UI** — shadcn/ui-based, fully Tailwind, individually installable components
- **snes.css** — Super Nintendo 16-bit aesthetic, proven patterns
- **RetroUI** — vintage color palettes, pixel-art borders, classic buttons
- **image-rendering: pixelated** CSS property enables nearest-neighbor scaling for crisp pixel look

### Performance Characteristics
| Metric | Result |
|--------|--------|
| Bundle size | <1MB (platform native webview) |
| Memory footprint | ~50% of Electron's Chromium |
| IPC latency | 0.5ms baseline; large payloads (100KB+) bottleneck on serialization |
| Graphical heaviness | **Not suitable** for WebGL-heavy scenes; Canvas rendering adequate for 2D |
| Resize performance | Slightly slower than pure Rust frameworks (egui, Iced) |

### Terminal Embedding
**Three proven approaches:**
1. **tauri-plugin-pty** — Full PTY spawning + xterm.js integration
2. **Sidecar binaries** — Bundle external executables (shell, tmux, custom CLI)
3. **CLI plugin** — Expose CLI interface in addition to GUI

### Verdict
✅ **Excellent for web-first retro UI** (HTML/CSS pixel art)
✅ **Lightweight distribution** (no browser engine)
✅ **Terminal integration feasible** via tauri-plugin-pty + xterm.js
⚠️ **Not suitable** for heavy sprite/tilemap rendering
⚠️ **Platform variability** — WebGL support differs per platform

---

## 2. egui: Immediate-Mode Rust GUI

### Rendering Model
- **Immediate-mode**: UI redescribed every frame (~60 FPS), no persistent widget objects
- **Smart redraw**: Only paints when interaction/animation occurs; idle apps use zero CPU
- **Multi-pass support** for layout-dependent widgets
- **Rendering-agnostic**: Generates shapes; backend handles GPU (wgpu, glow)

### Performance
- **Expected overhead**: 1-2ms per frame for typical UIs
- **Smooth window resizing** at 60+ FPS without CPU penalty
- **Immediate-mode advantage**: More efficient for highly interactive GUIs than retained-mode

### Achieving Pixel-Art Look
**Critical issue**: Pixel-style fonts (Cozette, Ark) render blurry due to bilinear texture filtering
- Font atlas cached at fixed resolution; zooming reveals pixelation
- Subpixel positioning (4 glyph versions at different offsets) used for crisp rendering
- **Solution needed**: Custom nearest-neighbor filtering for font atlas (not built-in)

### Bitmap Fonts & Retro Aesthetics
- **Custom font loading** via FontDefinitions
- **Font atlas system** dynamically grows (up to 16K textures)
- **Constraint**: Fixed-resolution bitmap fonts don't scale smoothly
- **Best practice**: Use vector fonts at fixed DPI for pixel-perfect output

### Sprite & Tilemap Rendering
**Limitations**: egui is a UI library, not a game engine
- No built-in sprite/tilemap systems
- `Painter::image()` workaround for textured rectangles (UI-oriented, bilinear filtered)
- **Better approach**: Combine egui with dedicated graphics (Bevy + bevy_egui)

### Native Rendering
| Backend | Description |
|---------|-------------|
| **wgpu** | Modern (Vulkan, Metal, D3D12, OpenGL, WebGPU) |
| **glow** | Lightweight (OpenGL 3.2+, faster compiles) |

**Platform support**: Windows, macOS, Linux, Android, Web, Steam Deck, VR

### Verdict
✅ **Excellent for retro-styled game UIs** (menus, HUDs)
✅ **Efficient redraw** (immediate-mode advantage)
✅ **Native rendering** (no webview, direct GPU)
⚠️ **Blurry pixel fonts** without workaround
❌ **Not suitable** for sprite/tilemap games
🔧 **Better as UI layer** over dedicated graphics engine

---

## 3. raylib vs SDL2: Game-Like Desktop Rendering

### raylib: Strengths for Pixelated Office World

**Built-in 2D Support**
- Native sprite sheet animation (TextureAtlas, frame calculation via modulo/column division)
- Tilemap integration via **raylib-tmx** (header-only Tiled loader)
- Pixel-perfect scaling: render to RenderTexture at low resolution, scale with `TEXTURE_FILTER_POINT`
- No external dependencies (batteries-included)

**Real-World Proof: MasterPlan**
- Project management UI built entirely in raylib + Go
- Panel-based interface managing tasks, categories, sidebar
- Local JSON storage, full CRUD operations
- **Demonstrates feasibility** for agent-based panel systems
- Available on itch.io and Steam

**Pixel-Rendering Pipeline**
```
RenderTexture(320x180) → render game at low resolution
↓
SetTextureFilter(TEXTURE_FILTER_POINT) → nearest-neighbor
↓
DrawTexturePro(3x integer scale) → Display at 960x540
```

**Ecosystem**
- 120+ official examples
- 60+ language bindings (Rust, Go, C#, Python, etc.)
- raygui: Simple immediate-mode GUI library built on raylib
- Asset tools: rTexPacker, rIconPacker, MySprite (pixel art), Quill (2D CAD)

### SDL2: Low-Level Alternative

**Strengths**
- 26+ years mature, AAA industry tested
- Fine-grained platform control
- FreeType2 font rendering (superior typography vs raylib's stb_truetype)
- Modular (SDL_image, SDL_ttf, SDL_mixer, SDL_net)

**Pixel-Perfect Scaling (SDL2)**
```c
SDL_SetHint(SDL_HINT_RENDER_SCALE_QUALITY, "nearest");
SDL_Texture* backBuffer = SDL_CreateTexture(..., 320, 180);
SDL_SetRenderTarget(renderer, backBuffer); // render game
SDL_SetRenderTarget(renderer, NULL);       // render to window
SDL_RenderSetScale(renderer, 3.0f, 3.0f); // integer scale
```

**Verdict**
| Use raylib if: | Use SDL2 if: |
|---|---|
| Fast prototyping, game-like UI | Fine rendering control needed |
| Panel-based system central | Performance extreme |
| Single binary deployment | Typography quality critical |
| Minimal setup | Modular architecture preferred |

### Pixel Rendering Techniques

**Nearest-neighbor scaling**: Preserves crisp edges at integer multiples (2x, 3x, 4x)

**CRT Scanlines & Effects** (Common misconception: CRT blur was *not* essential)
- Reality: Pixel art relied on deliberate dithering/anti-aliasing techniques
- Scanline emulation: Custom shader with black lines between pixel rows
- CRT shader elements: Screen curvature, chromatic aberration, RGB offsets, glow, vignette
- **Back-buffer technique**: Render at low resolution, upscale via shader with effects

### Cross-Platform Distribution
- **raylib**: Single binary (no deps), simple Makefile for Windows/macOS/Linux
- **SDL2**: CMake standard, requires SDL2 library or static linking

---

## 4. Game Engines: Bevy vs Godot

### Bevy (Rust ECS)

**2D Rendering**
- **2x faster 2D** than alternatives (GPU-accelerated pipeline)
- **Sprite batching**: Textures automatically batch for efficiency
- **ECS scales** to millions of entities with zero overhead
- Bevy 0.16: ~3x performance improvement over 0.15

**UI Systems**
- **CSS-like Flexbox** (Taffy library implementing CSS Grid/Flexbox)
- **Declarative composition** via Scene format or programmatic spawning
- **Low power mode** for always-on productivity tools
- Recent improvements: Color Plane widgets, ligature support

**Development Speed**
- **Steep learning curve** (Rust + ECS paradigm)
- **Fast compiles**: 0.8-3.0 seconds with "fast compiles" config
- **Strong documentation** and active community

**Terminal Integration**
- ⚠️ **Limited** — No built-in terminal emulator
- Workaround: Custom subprocess + IPC + texture rendering
- bevy_terminal_display renders Bevy *to* terminal, not vice versa

**Verdict**
✅ Excellent for **high-performance 2D** and **scale**
✅ Strong **sprite batching** and rendering
⚠️ Terminal embedding requires custom work
⚠️ Steep Rust/ECS learning curve

---

### Godot 4.x: Proven for Productivity Apps

**2D Rendering & Pixel Art**
- **Nearest-neighbor filtering** built-in for crisp pixels
- **Viewport scaling modes** for authentic retro low-resolution rendering (e.g., 320x240 upscaled)
- **Sprite atlasing** with automatic batching
- **Pixelorama** (open-source pixel editor built in Godot) — 8.5k+ GitHub stars, production-grade

**UI Systems**
- **Node-based hierarchy** with Container layout auto-management
- **Godot's own editor** built with its UI system — proof of scalability
- **Signal system** for event handling
- **Fast prototyping** with visual editor and hot-reloading (4.2+)

**Terminal Integration: UNIQUE ADVANTAGE**
- **godot-xterm** (GDExtension): Full XTerm emulator using libtsm + node-pty
  - Supports XTerm control sequences
  - Can embed full terminal inside scenes
  - Multi-platform: Linux (primary), macOS, partial Windows

- **GDExtension system**: Native C++/Rust integration for performance-critical components
- **Subprocess management**: Full PTY control via node-pty

**Non-Game Apps Built with Godot**
- **Pixelorama**: Pixel art editor (multi-layer, animation, frame tags, onion skinning)
- **Dungeondraft**: Map designer
- **Material Maker**: Procedural materials
- **Lorien**: Infinite canvas drawing
- **Bosca Ceoil Blue**: Music composition
- **GodSVG**: Vector design
- **vpuppr**: VTuber application

**Learning Curve**
- **Accessible**: GDScript is Python-like and intuitive
- **Integrated editor**: Immediate visual feedback
- **Fast iteration**: Hot-reloading (4.2+)
- **Fast prototyping** → production (3-4 weeks vs. 6-8 for Bevy)

**Verdict**
✅ **Strongest overall choice** for "pixelated office"
✅ **Mature terminal integration** via godot-xterm
✅ **Proven productivity apps** (Pixelorama)
✅ **Fast prototyping** with visual editor
✅ **Pixel-perfect rendering** built-in
✅ **GDExtension** for native performance

---

## 5. Terminal Integration Approaches

### Dedicated Solutions

| Solution | Integration | Best For |
|----------|-----------|----------|
| **godot-xterm** | Native terminal inside Godot scene | Godot apps, full XTerm support |
| **tauri-plugin-pty** | PTY spawning + xterm.js in webview | Tauri web-based UI |
| **xterm.js** | Browser-based terminal (Electron/Tauri) | Web-first architecture |
| **VTE (libvte)** | GTK-based terminal widget | GTK applications (not game engines) |
| **Alacritty parser** | Minimal terminal emulator (Rust) | Not designed as library; use via IPC |

### IPC Patterns

**Modern approach**: Spawn external terminal, communicate via IPC
- **tmux/zellij**: Session management + multiplexing
- **SSH**: Remote execution
- **Named pipes / sockets**: Bidirectional communication

**In-app embedding**: More complex but unified interface
- Requires PTY management (node-pty, portable-pty)
- Texture rendering for display
- Keyboard event forwarding

---

## 6. Pixel Rendering Techniques Deep Dive

### Nearest-Neighbor Scaling (Core Technique)

**Why it matters**: Preserves pixel edges, no anti-aliasing blur

```
Original pixel grid (8x8)    Scaled 4x with nearest-neighbor
████                         ████████████████████████████
████                         ████████████████████████████
████                         ████████████████████████████
████        →                ████████████████████████████
████                         ████████████████████████████
████                         ████████████████████████████
████                         ████████████████████████████
████                         ████████████████████████████
```

**Implementation**
- raylib: `SetTextureFilter(TEXTURE_FILTER_POINT)` or `ImageResizeNN()`
- SDL2: `SDL_SetHint(SDL_HINT_RENDER_SCALE_QUALITY, "nearest")`
- Tauri/Canvas: `image-rendering: pixelated` CSS property
- egui: Backend-dependent (glow vs wgpu); custom shader needed

### CRT Scanline & Post-Processing Effects

**Common misconception**: "CRT blur made pixel art look good"
**Reality**: Pixel art was carefully designed with dithering and anti-aliasing techniques that worked on any display

**Scanline emulation shader**:
- Render black horizontal lines between pixel rows
- Typically 1-2 pixels tall per game pixel
- Reduces visible resolution slightly but adds "retro" ambiance

**CRT shader parameters**:
- Screen curvature (barrel distortion)
- Chromatic aberration (RGB channel separation)
- Glow intensity and color offset
- Scanline height and opacity
- Vignette (darkened edges)

**Implementation**:
- Bevy/Godot: Custom fragment shaders
- raylib: Custom shader with GLSL (via `BeginShaderMode()`)
- SDL2: Post-processing via OpenGL

### Pixel-Perfect Layout

**Coordinate system considerations**:
- Round all positions to integers to avoid subpixel rendering
- Texture filtering matters: ensure nearest-neighbor for UI elements
- Font rendering critical: use monospace bitmap fonts or vector fonts at fixed DPI
- Test at multiple screen sizes and DPI settings

---

## 7. Existing Retro/Pixelated Applications

### Desktop Apps Using These Frameworks

**Godot Ecosystem**
- Pixelorama (pixel art editor) — production-grade
- Dungeondraft, Material Maker, Lorien, GodSVG

**raylib Ecosystem**
- MasterPlan (project manager) — panel-based UI proof
- Asset tools (MySprite, Quill, rTexPacker)

**Tauri Ecosystem**
- snes.css community examples (gaming/retro focus, not productivity)
- Retrom, Shard Launcher (community projects)
- **Note**: Pixel-art retro Tauri apps are underexplored (opportunity for novel approach)

**VTE/GTK Ecosystem**
- Ghostty terminal — GTK4 integration, respects system theming
- Alacritty + tmux/zellij — minimal aesthetic via constraint

### Design Philosophy in Your Vault

Your vault emphasizes:
- **Dark, professional aesthetic** (Chrysaki design system)
- **Terminal-based productivity** (no GUI bloat)
- **Wallpaper-driven palette** (pywal16 → CSS variables → components)
- **Inherently retro via constraint** (monospace, 256 colors, keyboard-driven)

---

## 8. Recommended Tech Stacks

### Stack 1: Godot 4.x (PRIMARY)

**Why**: Mature, proven, terminal-integrated

**Components**:
- **Engine**: Godot 4.2+
- **Terminal**: godot-xterm (GDExtension)
- **Rendering**: Built-in 2D with sprite atlases
- **Scripting**: GDScript (Python-like)
- **Distribution**: Single executable per platform

**Development time**: 3-4 weeks to MVP

**Strengths**:
- Visual editor for rapid iteration
- Pixelorama as proof-of-concept
- godot-xterm ready to use
- Pixel-perfect rendering built-in

---

### Stack 2: Tauri + React/Vue (SECONDARY)

**Why**: Lightweight, web-first, CSS pixel art mature

**Components**:
- **Backend**: Tauri (Rust)
- **Frontend**: React/Vue + TypeScript
- **Styling**: Pixelact UI or snes.css
- **Terminal**: tauri-plugin-pty + xterm.js
- **Distribution**: Platform-specific installers

**Development time**: 2-3 weeks to MVP

**Strengths**:
- Web dev familiar to many
- Lightweight distribution (<1MB)
- CSS pixel art is mature
- tauri-plugin-pty proven

**Weaknesses**:
- Platform variability (WebGL support)
- More terminal setup (xterm.js vs. built-in)

---

### Stack 3: raylib + Go (PERFORMANCE-FOCUSED)

**Why**: Proven by MasterPlan, minimal dependencies

**Components**:
- **Engine**: raylib (via raylib-go)
- **Language**: Go
- **UI**: raygui or custom immediate-mode
- **Terminal**: External (tmux/zellij via IPC)
- **Distribution**: Single binary, no deps

**Development time**: 4-6 weeks to MVP

**Strengths**:
- Proven project (MasterPlan)
- Excellent sprite/tilemap support
- Minimal dependencies
- Simple deployment

**Weaknesses**:
- Terminal integration requires IPC work
- Less integrated than Godot

---

### Stack 4: Bevy (HIGH-PERFORMANCE)

**Why**: Best rendering performance, Rust ecosystem

**Components**:
- **Engine**: Bevy 0.16+
- **UI**: CSS Flexbox layout
- **Language**: Rust (steep curve)
- **Terminal**: Custom subprocess + IPC
- **Distribution**: Platform-specific binaries

**Development time**: 6-8 weeks to MVP

**Strengths**:
- Best 2D rendering performance
- Strong Rust ecosystem integration
- Excellent sprite batching

**Weaknesses**:
- Steep learning curve
- Terminal integration requires custom work
- Longer development time

---

## 9. Decision Matrix

| Framework | Terminal | Pixel Art | Speed | Learning | Distribution |
|-----------|----------|-----------|-------|----------|--------------|
| **Godot** | ✅✅ godot-xterm | ✅✅ Built-in | ✅ 3-4 wks | ✅ GDScript | ✅ Single exe |
| **Tauri** | ✅ xterm.js | ✅ CSS | ✅ 2-3 wks | ✅ Web dev | ⚠️ Platform vars |
| **raylib** | ⚠️ IPC | ✅✅ Native | ✅ 4-6 wks | ✅ Simple | ✅ Single exe |
| **Bevy** | ⚠️ Custom | ✅ Shaders | ✅✅ Fast | ❌ Rust/ECS | ✅ Single exe |

---

## 10. Implementation Roadmap (Godot Recommended)

### Phase 1: Prototype (Week 1-2)
1. Create Godot 4.2+ project
2. Implement basic tilemap world (256x256 room or similar)
3. Add godot-xterm panel as proof-of-concept
4. Test terminal interaction (type, run shell commands)

### Phase 2: Agent Panels (Week 2-3)
1. Design panel UI (multiple terminal windows, agent status)
2. Implement panel docking/resizing system
3. Connect agent output to terminal emulators
4. Add basic agent orchestration (spawn, kill, status)

### Phase 3: Polish (Week 3-4)
1. Pixel-art aesthetics (sprite sheets, palette)
2. Keyboard navigation and shortcuts
3. Save/load panel layouts
4. Cross-platform testing and distribution

---

## Conclusion

**For a "pixelated office" native app with embedded agent terminal panels:**

1. **Godot 4.x** is the strongest recommendation
   - Mature terminal integration (godot-xterm)
   - Proven in productivity apps (Pixelorama)
   - Fast iteration with visual editor
   - Pixel-perfect rendering built-in

2. **Tauri + CSS** is viable if web-first approach preferred
   - Lightweight and modern
   - Mature pixel-art CSS frameworks
   - Terminal via xterm.js + tauri-plugin-pty
   - Platform variability is main concern

3. **raylib + Go** works well if pure performance/minimal dependencies desired
   - MasterPlan proves panel-based UI feasibility
   - Excellent sprite/tilemap support
   - Terminal integration via IPC required

4. **Bevy** best for extreme performance at cost of development time
   - Best 2D rendering speed
   - Steeper learning curve
   - Terminal integration requires custom work

---

## References

### Tauri
- Tauri Architecture & IPC: https://v2.tauri.app/concept/architecture/
- Pixelact UI: https://github.com/pixelact-ui/pixelact-ui
- snes.css: https://github.com/devMiguelCarrero/snes.css
- tauri-plugin-pty: https://github.com/Tnze/tauri-plugin-pty

### egui
- egui Documentation: https://docs.rs/egui/latest/egui/
- eframe Framework: https://docs.rs/eframe/latest/eframe/
- GitHub: https://github.com/emilk/egui

### raylib
- Official Homepage: https://www.raylib.com
- Examples: https://www.raylib.com/examples.html
- MasterPlan: https://github.com/SolarLune/masterplan
- raylib-tmx: https://github.com/RobLoach/raylib-tmx

### Godot
- Godot Engine: https://godotengine.org
- godot-xterm: https://github.com/lihop/godot-xterm
- Pixelorama: https://github.com/Orama-Interactive/Pixelorama

### Bevy
- Bevy Engine: https://bevy.org
- Documentation: https://docs.rs/bevy/latest/bevy/

### Pixel Rendering
- MDN: Crisp Pixel Art Look: https://developer.mozilla.org/en-US/docs/Games/Techniques/Crisp_pixel_art_look
- CRT Effect Analysis: https://datagubbe.se/crt/

---

*Report compiled from 5 parallel research agents. Authored by: Research Team, Orchestration-Research Squad*
