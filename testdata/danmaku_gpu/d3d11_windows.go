// Test overlay compiled against both revisions of the actual D3D11 builder.
package d3d11

import (
	"fmt"
	"math"
	"runtime"

	"github.com/egoist/mygo/internal/scene"
)

// DanmakuReadback returns BGRA, built instances, built batches and the sum
// of mask/ClearType quad areas inside the batch scissors. A fresh target
// draws the whole frame, independently of the window's presentation policy.
func DanmakuReadback(s *scene.Scene) ([]byte, [6]int, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var work [6]int
	hwnd, err := newHiddenWindow(s.Width, s.Height)
	if err != nil {
		return nil, work, err
	}
	defer destroyWindow(hwnd)
	r, err := New(hwnd)
	if err != nil {
		return nil, work, err
	}
	defer r.Release()
	if r.compileClasses {
		return nil, work, fmt.Errorf("expected ahead-of-time class shaders")
	}
	if err := r.draw(s); err != nil {
		return nil, work, err
	}
	work[0], work[1] = len(r.b.Instances), len(r.b.Batches)
	for _, batch := range r.b.Batches {
		switch batch.Classes() {
		case 48:
			work[3]++
		case 112:
			work[4]++
		}
		for _, in := range r.b.Instances[batch.Start : batch.Start+batch.Count] {
			if in.Params[0] != 2 && in.Params[0] != 5 {
				continue
			}
			box := in.Rect
			x, y := int(math.Round(float64(box[0]))), int(math.Round(float64(box[1])))
			w := max(0, min(x+int(box[2]), int(batch.Scissor.Right))-max(x, int(batch.Scissor.Left)))
			h := max(0, min(y+int(box[3]), int(batch.Scissor.Bottom))-max(y, int(batch.Scissor.Top)))
			work[2] += w * h
			if classes := batch.Classes(); classes == 48 || classes == 112 {
				work[5] += w * h
			}
		}
	}
	pixels, _, err := r.read()
	return pixels, work, err
}

// Overlapping glyphs with fully empty, partially covered and per-channel
// ClearType texels over a translucent background.
func DanmakuOverlapScene() *scene.Scene {
	const side = 32
	s := &scene.Scene{Width: 194, Height: 118, Scale: 1,
		Clear:     scene.Color{R: 31, G: 87, B: 139, A: 139},
		MaskAtlas: scene.NewAtlas(1, 64, 64), ColorAtlas: scene.NewAtlas(4, 64, 64),
		Text: scene.TextParams{GammaRatios: scene.GammaRatios(1.8), Contrast: 0.5, SubpixelContrast: 1.2},
	}
	mask := make([]byte, side*side)
	rgb := make([]byte, 4*len(mask))
	for i := range mask {
		a := byte(31*i + 17*(i/side))
		if i%side < 10 {
			a = 0
		}
		mask[i] = a
		rgb[4*i], rgb[4*i+1], rgb[4*i+2], rgb[4*i+3] = a, 255-a, byte(7*i), 255
		if i%side < 6 {
			rgb[4*i+1], rgb[4*i+2] = 0, 0
		}
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

	return s
}
