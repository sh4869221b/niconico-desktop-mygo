// This test overlay adds readback to an in-memory Recorder for both revisions.
package ui

import "github.com/egoist/mygo/internal/gpu/d3d11"

func (r *Recorder) DanmakuTestBGRA() ([]byte, [3]int, error) {
	return d3d11.DanmakuReadback(r.h.last)
}
