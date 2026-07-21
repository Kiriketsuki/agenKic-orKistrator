shader_type canvas_item;
// T16 (#125) — palette-swap + power shader. Drives every visual variant of
// an agent character from two inputs: `power_level` (0..1, continuous) and
// `provider_lut` (a per-provider color ramp texture). One shader, applied to
// both today's placeholder ColorRect body and T22's future AnimatedSprite2D
// frames — see agent_character.gd for the composition contract.
//
// COMPOSITION CONTRACT: this shader produces the *base* styled color from
// (class_color, provider_lut, power_level). The existing STATE_TINTS /
// working-pulse modulate flow is UNCHANGED — it is applied by Godot AFTER
// this shader runs, by multiplying the node's `modulate` into every pixel's
// COLOR via the engine's builtin behavior (COLOR.rgb here is multiplied by
// the incoming COLOR builtin, which already carries `modulate`). So:
//
//   final_pixel = shader(class_color, provider_lut, power_level) * modulate
//
// TEXTURE semantics: for a ColorRect body (today), TEXTURE is the engine's
// default opaque white 1x1 — texel.rgb == vec3(1,1,1), texel.a == 1 — so the
// styled base color comes entirely from `class_color`. For a T22
// grayscale AnimatedSprite2D frame, `class_color` is set to WHITE (see
// agent_character.gd doc-comment) and the sprite's own luminance drives the
// ramp instead. Same shader, no branch, by construction.
//
// POWER-LEVEL BANDS (documented sample points on a continuous curve — see
// palette_math.gd for the exact smooth functions; the shader never steps):
//   1 Novice       (~0.1)  — muted saturation, no glow/metallic/threads.
//   2 Adept        (~0.35) — full saturation, faint 1px-equivalent glow.
//   3 Master       (~0.6)  — rich saturation + metallic sheen + pulsing glow.
//   4 Grandmaster  (~0.8)  — gold thread overlay + brighter animated glow.
//   5 Legendary    (~1.0)  — iridescent hue-cycle + full emissive halo.
// All five effect amounts (saturation/metallic/glow/thread/iridescence) are
// precomputed OFF-shader by PaletteMath from the power_level float and
// pushed as plain uniforms — this shader only does per-pixel/TIME work.

// -- Provider LUT ------------------------------------------------------------
// One 256x1 gradient per provider (procedurally built by provider_palette.gd
// from tower.json color stops). Sampled by luminance: x = luminance(base).
uniform sampler2D provider_lut : source_color, filter_linear;
// 0 = provider unknown/not-yet-wired -> preserves today's vivid class colors
//     unmodified (non-regression contract, see provider_palette.gd).
// >0 = blend toward the provider hue-remap (config `lut_strength`, ~0.85).
uniform float lut_mix : hint_range(0.0, 1.0) = 0.0;

// -- Class base color ---------------------------------------------------------
// The agent's class color, moved OUT of the node's own `color`/frame data and
// into this uniform so the same shader path works for a flat ColorRect
// (texel==white) and a future grayscale sprite (texel==white required here).
uniform vec4 class_color : source_color = vec4(1.0, 1.0, 1.0, 1.0);

// -- Power-driven effect amounts (precomputed by PaletteMath, smooth 0..1) --
uniform float power_level : hint_range(0.0, 1.0) = 0.0;
uniform float saturation : hint_range(0.0, 2.0) = 1.0;
uniform float metallic : hint_range(0.0, 1.0) = 0.0;
uniform float glow_amount : hint_range(0.0, 1.0) = 0.0;
uniform float thread_amount : hint_range(0.0, 1.0) = 0.0;
uniform float iridescence : hint_range(0.0, 1.0) = 0.0;
uniform float pulse_speed : hint_range(0.0, 8.0) = 2.0;


// Standard perceptual luminance (Rec. 601-ish weights), used both to
// desaturate/saturate around and to key the provider LUT lookup.
float luminance(vec3 c) {
	return dot(c, vec3(0.299, 0.587, 0.114));
}


// Rotate a color's hue by `amount` turns (0..1) using a lightweight RGB
// rotation matrix approximation — cheap enough for a per-pixel canvas_item
// shader and sufficient for the "iridescent shimmer" read at Legendary.
vec3 hue_rotate(vec3 c, float amount) {
	float angle = amount * TAU;
	float s = sin(angle);
	float co = cos(angle);
	mat3 rot = mat3(
		vec3(0.299 + 0.701 * co + 0.168 * s, 0.587 - 0.587 * co + 0.330 * s, 0.114 - 0.114 * co - 0.497 * s),
		vec3(0.299 - 0.299 * co - 0.328 * s, 0.587 + 0.413 * co + 0.035 * s, 0.114 - 0.114 * co + 0.292 * s),
		vec3(0.299 - 0.300 * co + 1.250 * s, 0.587 - 0.588 * co - 1.050 * s, 0.114 + 0.886 * co - 0.203 * s)
	);
	return clamp(rot * c, 0.0, 1.0);
}


void fragment() {
	vec4 texel = texture(TEXTURE, UV);
	vec3 base = texel.rgb * class_color.rgb;

	// Saturation: mix between the luminance-gray version and the full-color
	// base. saturation < 1.0 mutes (Novice), > 1.0 oversaturates (Master+).
	float base_luma = luminance(base);
	vec3 styled = mix(vec3(base_luma), base, saturation);

	// Metallic: a moving specular-ish highlight band that sweeps vertically
	// over time, gated by `metallic` so it only reads at Master+.
	float sweep = 0.5 + 0.5 * sin(UV.y * 18.0 - TIME * pulse_speed * 0.6);
	styled += vec3(pow(sweep, 6.0)) * metallic * 0.5;

	// Gold threads: thin animated horizontal lines overlaid at Grandmaster+.
	float thread_lines = smoothstep(0.94, 1.0, sin(UV.y * 40.0 + TIME * pulse_speed * 0.3) * 0.5 + 0.5);
	vec3 gold = vec3(1.0, 0.84, 0.35);
	styled = mix(styled, gold, thread_lines * thread_amount);

	// Iridescence: full hue-cycle shimmer at Legendary only.
	float shimmer = fract(UV.y * 2.0 + TIME * pulse_speed * 0.08);
	styled = mix(styled, hue_rotate(styled, shimmer), iridescence);

	// Provider hue-remap: look the styled color's luminance up in the
	// provider's LUT, then blend in by lut_mix (0 for unknown provider —
	// preserves the vivid class colors shipped today).
	float key = clamp(luminance(styled), 0.0, 1.0);
	vec3 swapped = texture(provider_lut, vec2(key, 0.5)).rgb;
	vec3 final_color = mix(styled, swapped, lut_mix);

	// Smooth emissive glow — driven continuously by power_level (never
	// stepped, acceptance #6), with a TIME-based pulse so it reads as alive.
	// This is an in-silhouette rim/bloom approximation, not a true outer
	// halo (canvas_item fragment shaders cannot draw outside the node's own
	// quad) — a scaled-back glow node is deferred to T22 with real sprites.
	float pulse = 0.6 + 0.4 * sin(TIME * pulse_speed);
	vec3 emissive = vec3(1.0) * glow_amount * pulse * 0.35;

	COLOR.rgb = (final_color + emissive) * COLOR.rgb;
	COLOR.a = texel.a * COLOR.a;
}
