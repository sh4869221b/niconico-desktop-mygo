package main

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// Apply after the system's accent and high-contrast preferences. One text
// color must work on all three faces of the same primary button.
func readableAppTheme(original *ui.Theme) *ui.Theme {
	t := *original
	black, white := ui.RGB(0, 0, 0), ui.RGB(255, 255, 255)
	faces := []ui.Color{t.Accent, t.AccentHover, t.AccentPressed}
	worst := func(fg ui.Color) float64 {
		ratio := 21.0
		for _, bg := range faces {
			ratio = min(ratio, colorContrast(fg, bg))
		}
		return ratio
	}
	fg := white
	if worst(black) > worst(white) {
		fg = black
	}
	t.AccentText = fg
	far := black
	if fg == black {
		far = white
	}
	fit := func(bg ui.Color) ui.Color {
		for step := 0; step <= 100; step++ {
			candidate := bg.Mix(far, float32(step)/100)
			if colorContrast(fg, candidate) >= 4.5 {
				return candidate
			}
		}
		return far
	}
	t.Accent, t.AccentHover, t.AccentPressed = fit(t.Accent), fit(t.AccentHover), fit(t.AccentPressed)
	for step := 0; step <= 100; step++ {
		candidate := original.TextMuted.Mix(t.Text, float32(step)/100)
		valid := true
		for _, bg := range []ui.Color{t.Background, t.Surface, t.SurfaceHover, t.SurfacePressed} {
			valid = valid && colorContrast(candidate, bg) >= 4.5
		}
		if valid {
			t.TextMuted = candidate
			break
		}
	}
	return &t
}

func colorContrast(a, b ui.Color) float64 {
	lum := func(c ui.Color) float64 {
		linear := func(v uint8) float64 {
			s := float64(v) / 255
			if s <= 0.04045 {
				return s / 12.92
			}
			return math.Pow((s+0.055)/1.055, 2.4)
		}
		return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
	}
	x, y := lum(a), lum(b)
	return (max(x, y) + 0.05) / (min(x, y) + 0.05)
}
