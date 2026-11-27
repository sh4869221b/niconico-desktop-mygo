package d3d11

import (
	"bytes"
	"runtime"
	"slices"
	"testing"

	"github.com/egoist/mygo/internal/scene"
)

// Exercise ordered, overlapping mask, subpixel and color glyphs with both
// the existing all-class shader and the two mixed-text class shaders.
func TestOutlinedGlyphBatches(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	const side = 32
	s := &scene.Scene{Width: 194, Height: 118, Scale: 1,
		Clear:     scene.Color{R: 31, G: 87, B: 139, A: 255},
		MaskAtlas: scene.NewAtlas(1, 64, 64), ColorAtlas: scene.NewAtlas(4, 64, 64),
		Text: scene.TextParams{GammaRatios: scene.GammaRatios(1.8), Contrast: 0.5, SubpixelContrast: 1.2},
	}
	mask := make([]byte, side*side)
	rgb := make([]byte, 4*len(mask))
	for i := range mask {
		a := byte(31*i + 17*(i/side))
		mask[i] = a
		rgb[4*i], rgb[4*i+1], rgb[4*i+2], rgb[4*i+3] = a, 255-a, byte(7*i), 255
	}
	s.MaskAtlas.Put(2, 2, side, side, mask, side)
	s.ColorAtlas.Put(2, 2, side, side, rgb, 4*side)
	for layer := range 10 {
		first := len(s.Glyphs)
		for i := range 25 {
			x, y := float32((i*23+layer*3)%166-8), float32((i*17+layer)%101-5)
			color := scene.Color{R: byte(31 * layer), G: byte(37 * i), B: byte(29*layer + 3*i), A: byte(131 + (5*i+layer)%125)}
			s.Glyphs = append(s.Glyphs, scene.Glyph{X: x, Y: y, W: side, H: side, U: 2, V: 2, UW: side, VH: side, Color: color, Subpixel: i%2 != 0, Colored: layer%2 != 0 && i%8 == 2})
		}
		s.Ops = append(s.Ops, scene.Op{Kind: scene.OpGlyphs, Start: int32(first), End: int32(len(s.Glyphs))})
	}
	hwnd, err := newHiddenWindow(s.Width, s.Height)
	if err != nil {
		t.Fatal(err)
	}
	defer destroyWindow(hwnd)
	old := classShaders
	defer func() { classShaders = old }()
	draw := func(sets []classShader) []byte {
		t.Helper()
		classShaders = sets
		r, err := New(hwnd)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Release()
		if err := r.draw(s); err != nil {
			t.Fatal(err)
		}
		mixed := false
		for _, b := range r.b.Batches {
			if b.Classes() == 48 || b.Classes() == 112 {
				mixed = true
			}
		}
		if !mixed {
			t.Fatal("fixture did not exercise mixed glyphs")
		}
		pix, _, err := r.read()
		if err != nil {
			t.Fatal(err)
		}
		return pix
	}
	baseline := slices.DeleteFunc(slices.Clone(old), func(s classShader) bool { return s.classes == 48 || s.classes == 112 })
	base, specialized := draw(baseline), draw(old)
	if !bytes.Equal(base, specialized) {
		for i, v := range base {
			if v != specialized[i] {
				t.Fatalf("BGRA byte %d changed from %d to %d", i, v, specialized[i])
			}
		}
	}
}
