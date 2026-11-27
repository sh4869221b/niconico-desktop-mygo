// Test overlay for comparing two GPU implementations on one recorded frame.
package ui

import (
	reference "github.com/egoist/mygo/internal/danmakuref/d3d11"
	"github.com/egoist/mygo/internal/gpu/d3d11"
)

func (r *Recorder) DanmakuTestBGRA(baseline bool) ([]byte, [6]int, error) {
	if baseline {
		return reference.DanmakuReadback(r.h.last)
	}
	return d3d11.DanmakuReadback(r.h.last)
}
