shader_type canvas_item;
// Phase 6 section 4. Fades the bottom of the tower out, so the shaft and the
// plinth dissolve into the treeline instead of ending on a hard horizontal cut.
// The fade runs on local Y in art pixels, not on UV. The stair shaft draws a
// region taller than its texture, so its UV.y runs past 1.0, and the plinth
// polygons carry no UV at all. Local Y works for both.

// Where the fade starts, in node-local pixels. Alpha stays untouched above it.
uniform float fade_start_y = 0.0;
// Where the fade ends. Alpha reaches zero at this line and below it.
uniform float fade_end_y = 1.0;

varying float v_local_y;

void vertex() {
	v_local_y = VERTEX.y;
}

void fragment() {
	float span = max(fade_end_y - fade_start_y, 0.001);
	float t = clamp((v_local_y - fade_start_y) / span, 0.0, 1.0);
	COLOR.a *= 1.0 - smoothstep(0.0, 1.0, t);
}
