package main

import (
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Shapes with DirectWrite and reads the actual premultiplied D3D11 target.
// The native player gets an in-memory video. No browser or stream is opened.
func TestCompareD3D11Danmaku(t *testing.T) {
	for _, density := range danmakuDensities {
		n, draw := densityPlayer(t, density.interval)
		n.stream.Pause()
		for _, size := range []struct {
			w, h  int
			scale float32
		}{{980, 680, 1}, {740, 524, 1.25}} {
			rec := ui.NewRecorder(draw, ui.RecorderOptions{Width: size.w, Height: size.h, Scale: size.scale})
			for i, ms := range []int64{5000, 5017, 5033, 5033, 13999, 2701, 2720, 26000} {
				n.stream.Seek(time.Duration(ms) * time.Millisecond)
				frame := rec.Frame(time.Duration(i) * time.Second / 60)
				pixels, work, err := rec.DanmakuTestBGRA()
				if err != nil {
					t.Fatal(err)
				}
				if i == 1 && size.scale == 1 {
					t.Logf("WORK %s active=%d instances=%d batches=%d mask/ClearType-pixels=%d", density.name, len(n.danmaku.active), work[0], work[1], work[2])
				}
				t.Logf("FRAME %s %d %d %g %d %d %d %x", density.name, size.w, size.h, size.scale, ms, len(n.danmaku.active), len(frame.Glyphs), sha256.Sum256(pixels))
			}
			rec.Close()
		}
	}

	// Fit long unique text across the viewport and exercise a saturated lane.
	cues := benchmarkCues(true, true)
	var d danmakuOverlay
	var ms int64
	box := ui.Rect{X: 11.25, Y: 7.5, W: 430.5, H: 254}
	rec := ui.NewRecorder(func(c *ui.Context) { d.draw(c, box, cues, ms) }, ui.RecorderOptions{Width: 548, Height: 325, Scale: 1.25})
	defer rec.Close()
	for i, at := range []int64{5000, 5017, 10000, 701, 701, 718} {
		ms = at
		if i == 4 {
			box.W, box.H = 370, 221
		}
		frame := rec.Frame(time.Duration(i) * time.Second / 60)
		pixels, _, err := rec.DanmakuTestBGRA()
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("FRAME Unique %d %d %d %s %x", ms, len(d.active), len(frame.Glyphs), fmt.Sprint(box), sha256.Sum256(pixels))
	}
}
