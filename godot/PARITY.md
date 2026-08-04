# HTML → Godot visual parity guide

Why the Godot build on `feature/163-feature-phase-3-magic-polish-t15-t22` looks nothing like `Tower Demos.dc.html`, and exactly what to change. Grounded in the branch's actual `tower_manager.gd`, `floor_scene.gd`, `tower_exterior.gd`, `main.tscn`, `project.godot`.

The good news: the assets ARE wired in on this branch (wall/floor tiles, torch/window deco, workstation props, AgentCharacter, nameplate nine-patch). The gap is not missing art — it is **one root cause plus five side effects**.

---

## Root cause — the pixel-scale regime was abandoned

The HTML demo is a **320×180 pixel world rendered at an integer zoom** (4× → 1280×720). Every sprite, tile, and slab lives in art pixels; the *whole world* is magnified uniformly.

The 163 branch instead renders a **1920×1080 vector canvas** (`viewport_width=1920`, `stretch/mode="canvas_items"`, `aspect="expand"`) and pushes 1×-authored art into it with per-node fudge factors:

| Element | Branch scale | Result on screen |
|:--|:--|:--|
| Floor slab | 460×73 px (viewport-proportional, `_recalculate_layout_metrics`) | 24% of screen width — demo slab is **87%** |
| Agent sprites | 1× (16×20 px) | microscopic |
| Props | ad-hoc `scale = 2.5` | 40 px, mismatched |
| Torch / window | ad-hoc `scale = 2.0` | 32 px, mismatched |
| Wall tile UV | 1× (16 px repeat) | dollhouse brick on a big slab |
| Sky | `sky.png` stretched fullscreen | 1px stars (your screenshot) |

Three different pixel densities on one screen is why it reads as "so different" — nothing shares a grid, and the tower is a miniature floating in 1080p space.

### The fix: one world, one zoom

Keep the 1080p project (the panel/UI layer needs it). Make the **tower world live in art pixels** and let the camera provide the magnification:

1. **Camera zoom = 6** (integer): `320 × 6 = 1920`, `180 × 6 = 1080` — the visible world becomes exactly the demo's 320×180 canvas. Compute at runtime, keep it integer:
   ```gdscript
   var z: float = floorf(get_viewport_rect().size.y / 180.0)  # 6 at 1080p
   _camera.zoom = Vector2(z, z)
   ```
2. **Delete viewport-proportional sizing.** `_recalculate_layout_metrics()` currently derives `_floor_width/_floor_height/_floor_spacing/_tower_radius` from viewport size. With the camera owning magnification, these are constants in art px:
   ```gdscript
   _floor_width = 280.0    # 87% of the 320px world, same as demo
   _floor_height = 40.0
   _floor_spacing = 56.0   # demo floor tops 96/320/560 at 4× → 56 at 1×
   _tower_radius = 40.0
   ```
   Keep `_update_tower_frame()`'s centering, but position in world coords (0,0), not screen-center pixels.
3. **Remove every ad-hoc sprite scale.** Props `2.5→1.0`, torch/window `2.0→1.0`, nameplate `Vector2(100,30)→art-px or move to UI layer (see §5)`. In a uniform world, `scale != 1.0` on art is always a bug.
4. **Re-proportion the 3/4-view plane** to the art grid: `PLANE_DEPTH 26→16` (one tile row, matching the demo's 64px@4× floor band), `PLANE_FLARE 18→8`.
5. **Restrict ctrl-zoom to integers.** `ZOOM_MIN 0.5 / STEP 0.1` produces fractional zooms that shimmer the pixel grid. Step between `{4, 5, 6, 8}` instead.

Everything below assumes this regime. **Demo px ÷ 4 = world px** for any value you lift from `Tower Demos.dc.html`.

---

## The five side effects to clean up

### 1. Background: one stretched sky.png instead of the parallax stack

`main.tscn` has a single fullscreen `TextureRect` (`stretch_mode=6`) — that's the flat purple void with 1px stars. Replace with the shipped backdrop stack (`PORTING.md` table), inside the world (so it zooms with the camera), not a CanvasLayer:

```
ParallaxBackground
├─ Sky     motion_scale 0.0   backdrop_sky.png     320×180 fixed
├─ Clouds  motion_scale 0.1   backdrop_clouds.png  mirroring.x=320, autoscroll ~2 px/s
├─ Peaks   motion_scale 0.25  backdrop_peaks.png   mirroring.x=320, bottom-anchored
└─ Canopy  motion_scale 0.5   backdrop_canopy.png  mirroring.x=320, bottom of frame
```

At zoom 6 the sky's stars become 6px blocks — the demo look.

### 2. No stair shaft — floors read as floating plates

The demo tiles `stair_shaft.png` vertically on the tower axis behind the slabs, bridging every inter-floor gap. Add one `Sprite2D` (region_enabled, vertical region tiling, ~36–48 world px wide) or `TextureRect` (`stretch_mode=TILE`) spanning bottom floor → top floor, z-index below `FloorsContainer`. Resize in `_layout_floors()`.

### 3. Tower exterior is still flat placeholder polygons

`tower_exterior.gd` is unchanged from T4: procedural `Polygon2D`s in flat `Color(0.25,0.30,0.22)` / `(0.15,0.18,0.15)` — the detached triangle in your screenshot. Replace with the demo's cap: a 60×14 pentagon (`50% 0, 100% 55%, 84% 100%, 16% 100%, 0 55%`) in `#3f4a3a` seated directly on the top slab, and a base plinth under the bottom slab. Better: paint `roof_cap.png` / `base_plinth.png` so they carry ink outlines like the rest of the art — no flat vector fills next to pixel art.

### 4. Motion doesn't match the demo (and violates the no-bounce rule)

Demo: everything refocuses over **0.55 s `cubic-bezier(0.16,1,0.3,1)`** — position, scale, alpha together. Branch: fisheye tweens 0.2 s SINE, camera scrolls 0.3 s **TRANS_BACK** (spring overshoot — explicitly banned in the design system: no bounces, no overshoot).

```gdscript
const REFOCUS_DUR := 0.55
# camera (in _do_scroll_tween) and fisheye (in _apply_fisheye_layout):
.set_trans(Tween.TRANS_EXPO).set_ease(Tween.EASE_OUT)  # ≈ the demo bezier
```

Keep `_elastic_overscroll` if you like the affordance, but QUAD/OUT both ways — no BACK. The tween-kill hygiene on this branch is already good; don't lose it.

### 5. Text rendered in world space breaks at integer zoom

`_name_label` (plus its NinePatchRect plate, 100×30) lives in world px — at zoom 6 the default vector font renders 6× and the plate covers a third of the slab. Two clean options:

- **Preferred:** move floor labels + agent nameplates to `UILayer` (already exists, renders at window res). Fira Code, demo colors — focused `#c8a84e`, dim `#8a9a7a`, uppercase, letter-spaced. Project anchors each frame or on refocus: `floor_node.get_global_transform_with_canvas().origin`.
- In-world only with a 5–7 px bitmap pixel font, never a DynamicFont.

The nameplate/parchment nine-patches then also render at UI resolution (`NinePatchRect`, margins per `PORTING.md`) — crisp, like the demo's CSS `border-image`.

---

## What to keep — 163's polish survives the regime change

- **T15 morph** (load-driven n-gon, breathe, edge glow): fully compatible. `_effective_width` just becomes art-px (280 base). Wall-tile UVs (`uv = scaled`) already repeat at 16 px — correct once the slab is 280 wide.
- **T17 particles**: budgets unchanged; emitters emit in world px (dial extents ÷ ~1.6 if they were tuned against 460-wide floors).
- Torch flicker `_process` pulse — matches the demo's atmosphere loop; keep.
- Scroll queue / overscroll / jump_to_floor logic — untouched.

## Verification checklist

1. 1080p window: one art pixel = 6×6 screen px everywhere — tile, agent, prop, star. No element at a different density.
2. Focused slab spans ~87% of screen width, vertically centred; adjacent floors at 0.4 scale sit 56 world px away with no voids.
3. Shaft visibly connects every floor gap; roof cap sits ON the top slab.
4. Clouds drift; peaks/canopy parallax on refocus.
5. Refocus eases 0.55 s, no spring overshoot anywhere.
6. Labels crisp at window res; nameplate nine-patch not stretched blurry.
7. Ctrl-zoom snaps between integer zooms only.

## Hard No's

- No per-node scale factors on art sprites (2.0 / 2.5 fudges) — the camera is the only magnifier.
- No viewport-proportional world geometry — art px are absolute.
- No flat-color vector polygons beside pixel art (exterior, cornice, plinth: texture them or paint sprites).
- No `TRANS_BACK` / spring overshoot — the system is precise.
- No vector-font text in world space.

---

# Phase 4 — floor anatomy & rotation parity

Follow-up to `PARITY.md`. Grounded in the post-b472593 build (screenshot 2026-08-04): scale regime, backdrops, shaft, and roof are correct. Two design gaps remain — the focused floor reads as a rampart, and edge rotation reads as a teleport. This file is the implementation spec for both.

All values in **world/art px** (camera zoom 6 owns magnification). Demo px ÷ 4 = world px.

---

## 1. Floor anatomy — from rampart to room

### Current (wrong)

The T15 n-gon (`_background`, 280×40) is textured edge-to-edge with `stone_wall`; windows/torches are embedded in the brick; agents/props render below the slab, visually lost. 100% wall, 0% room.

### Target — three depth bands (demo anatomy)

```
        ┌ cornice (light stone strip, 2 px)
 band A │ BACK WALL — one row of stone_wall/wall_moss, 16 px
        │   windows + torches live ONLY here
        ├───────────────────────────────────
 band B │ FLOOR PLANE — stone_floor/wood_floor in the
        │   3/4 trapezoid, ~20 px deep, flare 8 px
        │   props sit at the back of it, against the wall line
        │   agents stand ON it, feet at plane mid-depth,
        │   heads overlapping ABOVE the wall line
        ├───────────────────────────────────
 band C │ FRONT LIP — plinth shadow line, 3 px, #0f1117-ish
```

Ratio flips from 40:0 (all wall) to **16 wall : 24 floor**. The agents become the largest, brightest element on the slab — that's the demo's read.

### Implementation notes (`floor_scene.gd`)

- Keep the n-gon as the **silhouette/clip only**. Texture its top 16 px band with the wall tile (either a second Polygon2D clipped to the band, or UV-map the wall texture so rows below 16 px sample the floor tile — the two-polygon approach is simpler and cheaper than a shader).
- `_rebuild_dressing()`: windows and torches reposition onto the wall band (`y ≈ -_floor_height/2 + 8`). Delete the current mid-slab placement.
- The existing 3/4 plane is right — set `PLANE_DEPTH = 20`, `PLANE_FLARE = 8`, and give it the floor tile per floor type (stone for Main Hall, wood for orkistrator — pull from a per-floor `floor_tile` field, default `stone_floor`).
- Agents: `feet_y = -_floor_height/2 + 16 + PLANE_DEPTH * 0.55` — heads clear the cornice by ~6 px. Props: back edge of the plane, `y = -_floor_height/2 + 16 + 2`.
- **Per-floor wash** (demo's soft-light gradient): one `ColorRect` over bands A+B, `CanvasItemMaterial BLEND_MODE_MUL`, alpha ≈ 0.15. Suggested tints: Main Hall `#3a4a3a`, Archive `#2e3448`, orkistrator `#4a3a2e`. This is what gives floors identity beyond brick gray.
- Label/nameplate stays on the UILayer (Phase 3 rule) — never inside band A.

### Acceptance

- On the focused floor, an idle agent is fully visible, feet grounded on floor tiles, head above the wall line.
- No window or torch below the wall band.
- Two adjacent floors are distinguishable by wash tint at 0.4 fisheye scale.

---

## 2. Edge rotation — from teleport to turn

### Current (wrong)

`_rotate_focused_edge()` tweens the ENTIRE floor node ±max(region·0.18, 320) px — the whole building flies off-screen and snaps back in two 0.15 s halves. Reads as a teleport; with an empty target edge, nothing visibly changes at all.

### Target — the room turns; the building stays

Ship **A + B** together; C is optional taste. One continuous **0.55 s, TRANS_EXPO / EASE_OUT** motion (never two half-tweens, never TRANS_BACK).

**A. Interior carousel.** The slab, silhouette, and dressing stay fixed. Only the contents layer (`AgentSlots` + props) moves:

```gdscript
# direction: +1 rotate right
var edge_w: float = EdgeLayout.edge_width_for_polygon(polygon_sides, _effective_width)
var tw := create_tween().set_parallel(true)\
    .set_trans(Tween.TRANS_EXPO).set_ease(Tween.EASE_OUT)
tw.tween_property(_agent_slots_node, "position:x", -direction * edge_w, 0.275)
tw.tween_property(_agent_slots_node, "modulate:a", 0.0, 0.2)
tw.chain().tween_callback(func() -> void:
    set_active_edge(new_edge)                       # rebuild for the new edge
    _agent_slots_node.position.x = direction * edge_w
)
tw.chain().tween_property(_agent_slots_node, "position:x", 0.0, 0.275)
tw.parallel().tween_property(_agent_slots_node, "modulate:a", 1.0, 0.25)
```

Delete the floor-node `position:x` tween from `tower_manager.gd` entirely.

**B. Wall UV scroll.** Simultaneously tween the wall band's texture offset by one edge width in the same direction — brick streaming past sells "the cylinder is turning" even when both edges are empty:

```gdscript
# wall_poly.texture_offset is in texture px; 1 world px = 1 texture px here
tw.parallel().tween_property(wall_poly, "texture_offset:x",
    wall_poly.texture_offset.x + direction * edge_w, 0.55)
```

(If band A is UV-mapped instead of a separate polygon, tween the uv array's x via a `tween_method` — same visual.)

**C. Fake perspective (optional).** During the slide, `_agent_slots_node.scale.x` 1.0 → 0.9 → 1.0 (two chained 0.275 s halves). Scale only — no skew, no rotation, no bounce.

**Orientation widget.** Port the demo's hex diagram as a small UILayer control (bottom-left, ~72 px screen): the floor's current n-gon drawn as `Line2D`/`draw_polygon`, active edge highlighted (`#FBB13C` at glow, others `#363a4f`), rotating 360/sides° per step with the same 0.55 s EXPO ease. It doubles as the composite-load readout (side count is already dynamic from T15). Without it, rotation on an empty edge is invisible.

### Acceptance

- Rotating with agents on both edges: old crew slides out one edge-width while new crew slides in; slab never moves.
- Rotating between two empty edges: wall brick visibly scrolls + hex widget turns — the action is never silent.
- No motion anywhere uses TRANS_BACK or exceeds one continuous ease.

---

## 3. Small polish (same PR)

| Issue | Fix |
|:--|:--|
| Moon's screentone halo clips as a hard square | Fade the dither's alpha radially in the PNG (or drop the halo — the demo moon has none) |
| Gray base hexagon under Main Hall reads as a floating pedestal | Narrow to shaft width (~48 px), darken to `#1a1d26`, tuck 4 px under the bottom slab — it's a plinth, not a plaza |
| Archive floor's windows/torches at 0.4 scale become sub-pixel noise | Hide dressing below fisheye distance 1 (`set_show_interior` already gates interior — gate `_dressing` the same way at distance ≥ 2) |

## Order of work

1. Floor bands (§1) — biggest visual payoff, self-contained in `floor_scene.gd`.
2. Rotation A+B (§2) — depends on §1's wall/contents split.
3. Hex widget + polish (§2 widget, §3).

---

# Phase 5 — why it still reads off

Post-Phase-4 build review (screenshot 2026-08-04, latest push to `feature/163-…-t15-t22`). The band anatomy, carousel, UV scroll, and zoom regime are all in and correct. The remaining gap is four perceptual problems — each small in code, large on screen.

---

## 1. Everything is the same gray — the tower has no color identity

The demo's floors are instantly tellable apart: Main Hall = **mossy green wall** (`wall_moss`), Archive = cold stone, orkistrator = **wood**. The build uses `stone_wall` for every wall band (`WALL_TEXTURE` is a single const), and the identity wash is invisible: a 15% multiply toward the tint on already-dark tiles shifts pixels by ~2 RGB points.

**Fix:**
- Add a `wall` key to `FLOOR_DRESSING` and per-floor wall textures:
  `main → wall_moss`, `archive → stone_wall`, `orkistrator → stone_wall` + wood plane (already set).
- Retire the multiply-wash subtlety: either raise `WASH_ALPHA` to **0.35**, or simpler and closer to the demo's soft-light — drop the wash polygon and bake the tint into the band `color` modulates directly (`_wall_band.color = wash_tint.lerp(Color.WHITE, 0.55)`, plane slightly brighter). Direct modulate is one multiply anyway, minus a node.
- Floor label color: demo gold `#c8a84e` on focused, `#8a9a7a` dim — the current world-label is near-black and sub-legible. Bump the UILayer font size ~1.5×.

## 2. The floor plane reads as a second wall (the "carrier deck")

Band B is a face-on trapezoid of full-brightness 16px tiles, flaring *wider than the slab* at the front — it reads as an aircraft-carrier deck, not ground. Un-foreshortened tiles can never read as a horizontal plane; the demo doesn't try — its floor band is a straight rect that is **darker than the wall and short**.

**Fix (all three, cheap):**
- `PLANE_FLARE = 0` — the plane stays inside the slab silhouette. The flare is what breaks the outline.
- Invert the value relationship: plane `color = Color(0.78, 0.78, 0.74)` (darker than the wall's 0.82), wall stays the lit surface. Ground recedes; walls catch torchlight.
- Add a 1px darker seam line at the wall/plane junction (the demo's step shadow) — this single line does more for the "room corner" read than the whole trapezoid.
- Optional depth cue done right: a vertical gradient on the plane (top 100% → bottom 85% brightness) via a 4px `GradientTexture2D` overlay — foreshortening by value, not by shape.

## 3. Rotation is invisible because every edge looks identical

The carousel + UV scroll are implemented — but windows, torches, and the wall band are **static dressing, identical for all 6–12 edges**. Rotate between two empty edges and the frame after the turn is pixel-identical to the frame before: the motion plays, the result reads as "nothing happened." The demo never has this problem because each edge shows different contents.

**Fix — make edges distinct, and move the tells into the moving layer:**
- Reparent windows into the scrolling/carousel layer (or offset their x by `active_edge * edge_w` and include them in the UV-scroll motion). Torches can stay fixed (they're the door frame, not the room).
- Derive per-edge dressing deterministically: `hash(floor_name, edge)` picks window count (1–3), window x-jitter, and one ambient prop (bookshelf / candle / crystal_orb from the atlas) even when the edge has zero agents. Empty edge ≠ empty room.
- Ship the edge compass prominently while rotating: fade it in at 1.0 alpha during the turn, decay to 0.4 after — the widget exists (`edge_compass.gd`), give it the moment.

## 4. Chrome details that break the silhouette

| Read | Cause | Fix |
|:--|:--|:--|
| Floating gray monorail above each floor | Cornice is a *light* strip with 4px overhang (`±hw+4`, `-hh-3`) | Flush trim, ink-weight trick: 2px light line at slab top edge, 1px dark below; zero overhang |
| Dark hexagon pedestal floating under Main Hall | `tower_exterior.gd` base polygon still 40px-radius hex at y+30 | Narrow to shaft width (~24px half-width), rect not hex, tuck 4px under the bottom slab, color `#14161f` |
| Tower column reads as a monolith with drawers | Shaft drawn at full opacity in front of the sky, same value as slabs | Drop shaft modulate to ~0.8 alpha or darken 20% — it's *behind* the rooms; the demo shaft is clearly recessive |
| Rooms are dead when no agents are connected | Interior only populates from `_agent_slots` | The per-edge ambient prop from §3 fixes this for free |

---

## Priority

1. §1 color identity — one-day change, biggest "matches the demo" jump.
2. §3 per-edge dressing — makes rotation *mean* something (the standing complaint).
3. §2 plane fixes — kills the carrier deck.
4. §4 chrome — an afternoon of nudges.

## Acceptance

- Screenshot the three floors side by side: each is identifiable by wall color alone at 0.4 fisheye scale.
- Rotate on an empty floor: the post-turn frame differs visibly from the pre-turn frame (different windows/prop), and the compass telegraphs the step.
- The slab silhouette is a clean chamfered rectangle — nothing overhangs, nothing flares.
- With zero agents connected, the focused floor still looks inhabited (ambient prop + torch flicker).
