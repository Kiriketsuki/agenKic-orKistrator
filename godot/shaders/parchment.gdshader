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

void fragment() {
	vec2 uv = UV;

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
