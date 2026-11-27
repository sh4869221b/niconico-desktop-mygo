// Test overlay for comparing two GPU implementations on one recorded frame.
package ui

import (
	reference "github.com/egoist/mygo/internal/danmakuref/d3d11"
	"github.com/egoist/mygo/internal/gpu/d3d11"
	"github.com/egoist/mygo/internal/scene"
)

func (r *Recorder) DanmakuTestBGRA(baseline bool) ([]byte, [6]int, error) {
	return danmakuRead(r.h.last, baseline)
}

func DanmakuOverlapReader() func(bool) ([]byte, [6]int, error) {
	s := d3d11.DanmakuOverlapScene()
	return func(b bool) ([]byte, [6]int, error) { return danmakuRead(s, b) }
}

func danmakuRead(s *scene.Scene, baseline bool) ([]byte, [6]int, error) {
	if baseline {
		return reference.DanmakuReadback(s)
	}
	return d3d11.DanmakuReadback(s)
}
