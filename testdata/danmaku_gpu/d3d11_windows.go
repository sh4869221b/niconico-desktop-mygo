// Test overlay compiled against both revisions of the actual D3D11 builder.
package d3d11

import (
	"math"
	"runtime"

	"github.com/egoist/mygo/internal/scene"
)

// DanmakuReadback returns BGRA, built instances, built batches and the sum
// of mask/ClearType quad areas inside the batch scissors. A fresh target
// draws the whole frame, independently of the window's presentation policy.
func DanmakuReadback(s *scene.Scene) ([]byte, [3]int, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var work [3]int
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
	if err := r.draw(s); err != nil {
		return nil, work, err
	}
	work[0], work[1] = len(r.b.Instances), len(r.b.Batches)
	for _, batch := range r.b.Batches {
		for _, in := range r.b.Instances[batch.Start : batch.Start+batch.Count] {
			if in.Params[0] != 2 && in.Params[0] != 5 {
				continue
			}
			box := in.Rect
			x, y := int(math.Round(float64(box[0]))), int(math.Round(float64(box[1])))
			w := max(0, min(x+int(box[2]), int(batch.Scissor.Right))-max(x, int(batch.Scissor.Left)))
			h := max(0, min(y+int(box[3]), int(batch.Scissor.Bottom))-max(y, int(batch.Scissor.Top)))
			work[2] += w * h
		}
	}
	pixels, _, err := r.read()
	return pixels, work, err
}
