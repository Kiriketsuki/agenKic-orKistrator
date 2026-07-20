shader_type canvas_item;
render_mode blend_add;
// T15 (#124) — additive edge glow for tower floors. Intensity tracks a
// floor's composite_load (0..1, pushed from FloorScene.set_composite_load
// via set_shader_parameter). Applied to Line2D outlines (EdgeGlow /
// ActiveEdgeGlow) so it composites as pure additive light with no
// occlusion of the floor geometry beneath it.

uniform float glow : hint_range(0.0, 1.0) = 0.0;
uniform vec4 glow_color : source_color = vec4(0.55, 0.85, 1.0, 1.0);
uniform float pulse_speed : hint_range(0.0, 8.0) = 2.0;

void fragment() {
	float pulse = 0.6 + 0.4 * sin(TIME * pulse_speed);
	vec3 lit = glow_color.rgb * glow * pulse;
	COLOR = vec4(lit, COLOR.a * glow_color.a);
}
