class_name TowerConfig
## Configuration for the tower layout, loaded from tower.json.

var polygon_sides: int = 6
var linger_duration_sec: float = 30.0
var permanent_floors: Array[Dictionary] = []

# --- T15 (#124) polygon-morphing tunables ---
## Active-agent count that saturates active_agents_norm to 1.0.
var load_capacity: int = 6
## Rolling window (seconds) over which recent task completions are counted
## for the honest throughput proxy (see FloorMorph doc-comment).
var throughput_window_sec: float = 60.0
## Completions/window that saturates task_throughput_norm to 1.0.
var throughput_cap: int = 6
## Side-count clamp — the bucket table itself only ever produces 6..12, this
## is a config-level sanity clamp around it.
var min_sides: int = 6
var max_sides: int = 12
## Circumradius multiplier range floors "breathe" across as load rises.
var breathe_min_scale: float = 1.0
var breathe_max_scale: float = 1.25
## Deadband (in load units) around a bucket edge before the side count is
## allowed to flip, to prevent oscillation while load hovers on a boundary.
var bucket_hysteresis: float = 0.03
## Interval (seconds) between TowerManager's periodic composite_load sweeps.
## composite_load is otherwise only recomputed on agent-event callbacks
## (register/state-change/deregister), so a floor's completion ring — and
## therefore its polygon — never decays once activity stops; this periodic
## sweep is what lets a loaded floor shrink back down on its own.
var load_recompute_interval_sec: float = 5.0

# --- T16 (#125) palette-swap shader tunables ---
## HONEST-MINIMAL placeholder power level applied to every agent — no real
## per-agent tier signal reaches the client yet (see palette_math.gd).
var default_power_level: float = 0.4
## OPTIONAL demo scaffolding: character_class -> power_level override, used
## only to visually spread the 5 named bands across classes on a live tower
## until a real server-side tier field exists. Empty by default.
var class_power_levels: Dictionary = {}
## Blend strength toward a known provider's LUT hue-remap (0..1). Unknown/
## empty provider always uses lut_mix=0 regardless of this value — see
## provider_palette.gd.
var lut_strength: float = 0.85
## provider name -> {"stops": [hex, hex, hex], "particle_style": "..."} used
## to build each provider's GradientTexture1D LUT (see provider_palette.gd)
## and (T17/#127) its particle accent color + shape style. Seeded from
## floating_rune.gd's PROVIDER_COLORS as a shared reference.
var providers: Dictionary = {}

# --- T17 (#127) tier particle effects tunables ---
## Global per-agent particle budget cap (acceptance #5) — threaded
## once-per-floor via TowerManager._create_floor ->
## FloorScene.configure_particle_budget(), NOT per-agent (see
## particle_math.gd / agent_character.gd doc-comments).
var max_particles_per_agent: int = 24

# --- T18 (#129) panel floaty animation tunables ---
## Master switch for ALL FOUR panel effects (bob, parchment flutter, drag
## trail, border shimmer). This project has no settings screen — this knob is
## the "effects toggleable in settings" acceptance criterion, following the
## T15/T16/T17 config-knob precedent. Threaded once via
## PanelManager._wire_panel() -> PanelBase.configure_panel_effects().
var panel_effects_enabled: bool = true
## Idle ambient float amplitude in pixels (snapped to whole pixels by
## PanelFloatMath.bob_offset_snapped — see its doc-comment).
var panel_bob_amplitude_px: float = 2.0
## Idle float period in seconds; also the base for the parchment flutter
## speed (PanelFloatMath.flutter_speed_rad applies FLUTTER_SPEED_RATIO).
var panel_bob_period_sec: float = 4.0
## Parchment edge-flutter amplitude in pixels, converted to shader UV units
## against the parchment's pixel height at apply time.
var panel_flutter_amplitude_px: float = 1.5
## Budget cap for the drag particle trail, clamped the same way
## max_particles_per_agent is. 0 disables the trail outright.
var max_drag_trail_particles: int = 12


static func from_file(path: String) -> TowerConfig:
	var config := TowerConfig.new()
	var file := FileAccess.open(path, FileAccess.READ)
	if file == null:
		push_warning("TowerConfig: could not open %s, using defaults" % path)
		return config
	var parsed: Variant = JSON.parse_string(file.get_as_text())
	if not parsed is Dictionary:
		push_warning("TowerConfig: invalid JSON in %s, using defaults" % path)
		return config
	var d: Dictionary = parsed as Dictionary
	config.polygon_sides = d.get("polygon_sides", 6)
	config.linger_duration_sec = d.get("linger_duration_sec", 30.0)
	config.load_capacity = d.get("load_capacity", 6)
	config.throughput_window_sec = d.get("throughput_window_sec", 60.0)
	config.throughput_cap = d.get("throughput_cap", 6)
	config.min_sides = d.get("min_sides", 6)
	config.max_sides = d.get("max_sides", 12)
	config.breathe_min_scale = d.get("breathe_min_scale", 1.0)
	config.breathe_max_scale = d.get("breathe_max_scale", 1.25)
	config.bucket_hysteresis = d.get("bucket_hysteresis", 0.03)
	config.load_recompute_interval_sec = d.get("load_recompute_interval_sec", 5.0)
	config.default_power_level = d.get("default_power_level", 0.4)
	config.lut_strength = d.get("lut_strength", 0.85)
	config.max_particles_per_agent = d.get("max_particles_per_agent", 24)
	config.panel_effects_enabled = d.get("panel_effects_enabled", true)
	config.panel_bob_amplitude_px = d.get("panel_bob_amplitude_px", 2.0)
	config.panel_bob_period_sec = d.get("panel_bob_period_sec", 4.0)
	config.panel_flutter_amplitude_px = d.get("panel_flutter_amplitude_px", 1.5)
	config.max_drag_trail_particles = d.get("max_drag_trail_particles", 12)
	var floors_raw: Variant = d.get("permanent_floors", [])
	if floors_raw is Array:
		for item: Variant in (floors_raw as Array):
			if item is Dictionary:
				config.permanent_floors.append(item as Dictionary)
	var class_power_raw: Variant = d.get("class_power_levels", {})
	if class_power_raw is Dictionary:
		config.class_power_levels = class_power_raw as Dictionary
	var providers_raw: Variant = d.get("providers", {})
	if providers_raw is Dictionary:
		config.providers = providers_raw as Dictionary
	return config
