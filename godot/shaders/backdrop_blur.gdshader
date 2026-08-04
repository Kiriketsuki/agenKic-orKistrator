shader_type canvas_item;
// Two-pass box blur, baked once against the title-screen tower backdrop
// (T-item-1). Sampling TEXTURE at fixed offsets works on gl_compatibility,
// unlike BackBufferCopy/SCREEN_TEXTURE, which the compatibility renderer
// does not support for this use case.

uniform float blur_radius_px : hint_range(0.0, 32.0) = 10.0;
uniform int pass_count : hint_range(1, 3) = 2;

void fragment() {
	vec2 texel = TEXTURE_PIXEL_SIZE * blur_radius_px;
	vec4 total = vec4(0.0);
	float weight_sum = 0.0;
	for (int pass = 0; pass < pass_count; pass++) {
		float shift = float(pass) + 1.0;
		vec2 horizontal_offset = vec2(texel.x * shift, 0.0);
		vec2 vertical_offset = vec2(0.0, texel.y * shift);
		float weight = 1.0 / shift;
		total += texture(TEXTURE, UV) * weight;
		total += texture(TEXTURE, UV + horizontal_offset) * weight;
		total += texture(TEXTURE, UV - horizontal_offset) * weight;
		total += texture(TEXTURE, UV + vertical_offset) * weight;
		total += texture(TEXTURE, UV - vertical_offset) * weight;
		weight_sum += weight * 5.0;
	}
	COLOR = total / weight_sum;
}
