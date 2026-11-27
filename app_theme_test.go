package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestAppColorsHaveReadableNormalHoverAndPressedText(t *testing.T) {
	for _, base := range []*ui.Theme{ui.LightTheme(), ui.DarkTheme()} {
		for _, accent := range []ui.Color{{}, ui.Hex("#ffff00"), ui.Hex("#0078d4"), ui.Hex("#ff0000"), ui.Hex("#000000"), ui.Hex("#ffffff")} {
			for _, high := range []bool{false, true} {
				theme := readableAppTheme(base.WithPreferences(ui.Preferences{Accent: accent, HighContrast: high}))
				for _, bg := range []ui.Color{theme.Accent, theme.AccentHover, theme.AccentPressed} {
					if got := colorContrast(theme.AccentText, bg); got < 4.5 {
						t.Fatalf("dark=%t accent=%v high=%t primary contrast %f", base.Dark, accent, high, got)
					}
				}
				for _, bg := range []ui.Color{theme.Background, theme.Surface, theme.SurfaceHover, theme.SurfacePressed} {
					if got := colorContrast(theme.TextMuted, bg); got < 4.5 {
						t.Fatalf("secondary contrast %f", got)
					}
				}
			}
		}
	}
}

func TestOriginalColorsReproduceReportedContrast(t *testing.T) {
	dark := ui.DarkTheme()
	if got := colorContrast(dark.AccentText, dark.Accent); got < 3.6 || got > 3.8 {
		t.Fatalf("unexpected baseline %f", got)
	}
	if got := colorContrast(dark.AccentText, dark.AccentHover); got < 2.5 || got > 2.6 {
		t.Fatalf("unexpected hover baseline %f", got)
	}
}
