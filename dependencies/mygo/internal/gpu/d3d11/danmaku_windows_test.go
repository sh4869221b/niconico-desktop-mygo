package d3d11

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/egoist/mygo/internal/scene"
)

// Compare the old and experimental batch policy on the same D3D device.
// The runs exceed the normal small-batch area and overlap in draw order.
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
			s.Glyphs = append(s.Glyphs, scene.Glyph{X: x, Y: y, W: side, H: side, U: 2, V: 2, UW: side, VH: side, Color: color, Subpixel: layer%2 != 0})
		}
		s.Ops = append(s.Ops, scene.Op{Kind: scene.OpGlyphs, Start: int32(first), End: int32(len(s.Glyphs))})
	}
	hwnd, err := newHiddenWindow(s.Width, s.Height)
	if err != nil {
		t.Fatal(err)
	}
	defer destroyWindow(hwnd)
	r, err := New(hwnd)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Release()
	draw := func(join bool) ([]byte, int) {
		t.Helper()
		r.b.JoinGlyphs = join
		if err := r.draw(s); err != nil {
			t.Fatal(err)
		}
		pix, _, err := r.read()
		if err != nil {
			t.Fatal(err)
		}
		return pix, len(r.b.Batches)
	}
	base, oldBatches := draw(false)
	joined, newBatches := draw(true)
	if newBatches >= oldBatches {
		t.Fatalf("same-device probe did not join runs: %d to %d", oldBatches, newBatches)
	}
	if !bytes.Equal(base, joined) {
		for i, v := range base {
			if v != joined[i] {
				t.Fatalf("BGRA byte %d changed from %d to %d", i, v, joined[i])
			}
		}
	}
}
