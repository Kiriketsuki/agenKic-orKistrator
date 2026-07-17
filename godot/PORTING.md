# Porting the HTML tower demos into Godot

Everything under `godot/` drops into the repo at `godot/` (project root `res://`). All art is authored at 1× pixel scale — render at integer zoom (4×–6×) with nearest filtering.

## Project settings

- `rendering/textures/canvas_textures/default_texture_filter = Nearest`
- `display/window/stretch/mode = canvas_items`, `stretch/scale_mode = integer`
- Snap 2D transforms/vertices to pixel: `rendering/2d/snap/snap_2d_transforms_to_pixel = true`

## Character sprite sheets — `assets/sprites/`

7 sheets, one per class. Each is **64×100 px = 4 columns × 5 rows of 16×20 cells**. Character is centred in the cell, feet on row 19 (baseline = cell bottom). Row order is fixed:

| Row | State | Frames | FPS | HTML timing |
|:--|:--|:--|:--|:--|
| 0 | idle | 4 | 4 | 1s loop |
| 1 | walking | 4 | 8 | 0.5s loop |
| 2 | working | 4 | 8 | 0.5s loop |
| 3 | reporting | 4 | 6 | 0.66s loop |
| 4 | stunned | 4 | 6 | 0.66s loop |

Ready-made `SpriteFrames` resources sit beside each sheet (`<class>_frames.tres`) with all 5 looping animations pre-cut at those FPS. Use with `AnimatedSprite2D`, or attach `scripts/agents/agent_sprite.gd` and set `character_class`.

Class accents (nameplates, selection glow, minimap dots):

| Class | Tier | Accent |
|:--|:--|:--|
| archmage | S | `#bf33e6` |
| enchanter | A | `#1ad9cc` |
| wardkeeper | A | `#40bf4d` |
| alchemist | B | `#d98c00` |
| librarian | B | `#a66626` |
| scribe | C | `#5999f2` |
| apprentice | D | `#a6a6bf` |

Provider badge glyphs (draw as text next to nameplate): claude `✦ #c88a4a` · gemini `♦ #7a9dc4` · openai `◆ #9a9a9a` · ollama `▲ #6db36d` · deepseek `◈ #a877c9`.

## Tileset — `assets/tiles/`

`tower_tileset.png` is 128×16: eight 16×16 tiles in one row, left to right:
`stone_floor, stone_cracked, stone_wall, wall_moss, moss_overlay, rune_floor, stairs, wood_floor`.

`tower_tileset.tres` is a Godot 4 `TileSet` with two atlas sources:
- **source 0** — the tiles above, atlas coords `(0..7, 0)`
- **source 1** — `assets/props/workstations.png`, same layout:
  `cauldron, lectern, anvil, desk, bookshelf, candle, crystal_orb, brazier`

`moss_overlay` has alpha — paint it on a TileMapLayer above the floor layer. Props can also be placed as `Sprite2D` + `AtlasTexture` when they need y-sort or animation later.

## UI nine-patches — `assets/ui/`

| Texture | Size | Margins (L,T,R,B) | Use |
|:--|:--|:--|:--|
| `parchment_panel.png` | 48×48 | 6,6,6,6 | panels, quest board |
| `nameplate_frame.png` | 48×16 | 6,4,6,4 | agent nameplates |
| `scroll_background.png` | 64×48 | 8,8,8,8 | scroll popups (alpha edges) |

Each has a matching `*_stylebox.tres` (`StyleBoxTexture`) for Theme/Panel use; for raw nodes use `NinePatchRect` with the same margins.

## HTML demo → Godot mapping

| Demo screen (Tower Demos.dc.html) | Godot target |
|:--|:--|
| Full tower (interactive) | `main.tscn` — tower exterior + floor tabs |
| Main interface | `tower_manager.gd` + `floor_scene.gd` (hex floor per `config/tower.json`: 6 sides) |
| Spawn an agent | spawn flow: fade-in on stairs tile → `walking` to workstation → `working` |
| Flooring & edges | `edge_layout.gd` — floor rotation steps 60° per click (hex) |
| Atmosphere | candle/brazier flicker: modulate pulse ~0.66s; rune_floor glow loop |

Animation state machine matches `AgentSprite.AnimState`: idle ↔ walking → working → reporting; stunned on error, return to idle. `linger_duration_sec: 30` (tower.json) before a finished agent despawns.

## Checklist

1. Copy `godot/assets/` and `godot/scripts/agents/agent_sprite.gd` into the repo.
2. Open in Godot 4.3+ — `.import` files generate automatically (defaults are fine once the project filter is Nearest).
3. Verify a sheet: instance `AnimatedSprite2D`, set frames to `alchemist_frames.tres`, play `walking`.
4. Build floors from `tower_tileset.tres` (source 0 floors/walls, source 1 props).
5. `asset_manifest.json` is the machine-readable spec if you generate loaders.
