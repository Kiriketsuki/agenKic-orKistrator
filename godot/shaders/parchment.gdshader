shader_type canvas_item;
// Procedural aged-parchment fill: fibre mottling from a runtime noise texture,
// a warm sepia gradient, an edge vignette, and top/bottom cylindrical curl
// shading to fake the rolled ends of the scroll. No texture assets required —
// `fibre_noise` is assigned a NoiseTexture2D generated in code at runtime.

uniform sampler2D fibre_noise : filter_linear_mipmap, repeat_enable;
uniform vec4 sepia_base : source_color = vec4(0.82, 0.71, 0.53, 1.0);
uniform vec4 sepia_edge : source_color = vec4(0.55, 0.42, 0.26, 1.0);
uniform vec4 fibre_tint : source_color = vec4(0.42, 0.32, 0.18, 1.0);
uniform float fibre_strength : hint_range(0.0, 1.0) = 0.18;
uniform float vignette_strength : hint_range(0.0, 1.0) = 0.35;
uniform float curl_height : hint_range(0.0, 0.3) = 0.08;

// --- T18 (#129) parchment edge flutter ---
// Written in fragment(), not vertex(): a ColorRect is one 4-vertex quad in
// Godot 4.2's canvas renderer, so a vertex() displacement cannot taper away
// from the edges or vary along X — it can only shear the whole quad, which
// would drag the parchment body (and visually, the reading surface) with it.
// Warping UV inside the edge bands gives a true edge-only flutter, and the
// scroll's text lives on a sibling RichTextLabel node that this shader never
// touches, so body text can never swim. Amplitude is supplied in UV units by
// SpellScrollView via PanelFloatMath.flutter_uv_amplitude(px, height_px).
uniform bool flutter_enabled = false;
uniform float flutter_amplitude : hint_range(0.0, 0.2) = 0.0;
uniform float flutter_speed : hint_range(0.0, 20.0) = 0.0;
uniform float flutter_band : hint_range(0.01, 0.5) = 0.14;
uniform float flutter_wavelength : hint_range(1.0, 24.0) = 9.0;

void fragment() {
	vec2 uv = UV;

	if (flutter_enabled) {
		// Edge weight: 1.0 exactly at the top/bottom border, falling to 0.0
		// at `flutter_band` inward, squared so the taper is gentle and the
		// mid-body weight is exactly zero (no residual motion under text).
		float top_w = 1.0 - clamp(uv.y / flutter_band, 0.0, 1.0);
		float bottom_w = 1.0 - clamp((1.0 - uv.y) / flutter_band, 0.0, 1.0);
		float edge = max(top_w, bottom_w);
		edge = edge * edge;
		float wave = sin(uv.x * flutter_wavelength + TIME * flutter_speed);
		uv.y += wave * flutter_amplitude * edge;
	}

	float fibre = texture(fibre_noise, uv * 3.0).r;
	vec3 base_color = mix(sepia_edge.rgb, sepia_base.rgb, uv.y * 0.3 + 0.7);
	base_color = mix(base_color, fibre_tint.rgb, fibre * fibre_strength);

	// Edge vignette — darkens toward the border of the scroll body.
	vec2 centered = uv - vec2(0.5);
	float dist = length(centered * vec2(1.0, 1.1));
	float vignette = smoothstep(0.35, 0.72, dist);
	base_color = mix(base_color, sepia_edge.rgb, vignette * vignette_strength);

	// Top/bottom cylindrical curl — shadow band right at the rolled edge,
	// with a thin highlight just inside it to sell the roll.
	float top_shadow = smoothstep(0.0, curl_height, uv.y);
	float bottom_shadow = smoothstep(1.0, 1.0 - curl_height, uv.y);
	base_color *= mix(0.55, 1.0, top_shadow);
	base_color *= mix(0.55, 1.0, bottom_shadow);

	float top_highlight = 1.0 - smoothstep(0.0, curl_height * 0.5, uv.y);
	float bottom_highlight = 1.0 - smoothstep(1.0, 1.0 - curl_height * 0.5, uv.y);
	base_color += vec3(0.10) * top_highlight;
	base_color += vec3(0.10) * bottom_highlight;

	COLOR = vec4(base_color, sepia_base.a);
}
