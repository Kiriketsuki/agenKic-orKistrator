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
