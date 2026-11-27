// Diagnostic variant of the class pixel shader. Preserve the original quad and
// interpolation; texels with no glyph coverage cannot change the destination.
ClassPS visibleGlyphPS(ClassOut i) {
	float mode = floor(i.params.x / 8);
	float kind = i.params.x - 8 * mode;
#if GLYPH
	if (kind > 1.5 && kind < 2.5) {
		float coverage = maskTex.SampleLevel(samp, i.pt.zw, 0).r;
		if (coverage == 0) discard;
		float4 c = premul(i.color);
		if (i.params.z > 0.5) c = paint(i.pt.xy, i.params.z, float4(0,0,0,0), i.color, i.color2, i.grad);
		ClassPS o;
		float4 res = c * textCoverage(coverage, unpremul(c), i.inner.x, i.inner.y, i.radii);
		o.color = res * (classClip(i, mode) * i.params.w);
#if SUBPIXEL
		o.alpha = o.color.aaaa;
#endif
		return o;
	}
#endif
#if SUBPIXEL
	if (kind > 4.5) {
		float3 coverage = colorTex.SampleLevel(samp, i.pt.zw, 0).rgb;
		if (!any(coverage)) discard;
		float4 c = premul(i.color);
		if (i.params.z > 0.5) c = paint(i.pt.xy, i.params.z, float4(0,0,0,0), i.color, i.color2, i.grad);
		float3 straight = unpremul(c);
		float3 a = subpixelCoverage(coverage, straight, i.inner.x, i.inner.y, i.radii);
		float3 w = a * c.a * classClip(i, mode) * i.params.w;
		float wa = (w.r + w.g + w.b) / 3;
		ClassPS o;
		o.color = float4(straight * w, wa);
		o.alpha = float4(w, wa);
		return o;
	}
#endif
	return classps(i);
}
