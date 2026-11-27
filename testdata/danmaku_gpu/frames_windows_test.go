package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Shapes with DirectWrite and reads the actual premultiplied D3D11 target.
// The native player gets an in-memory video. No browser or stream is opened.
func compareDanmakuFrame(t *testing.T, read func(bool) ([]byte, [6]int, error), name string, measure bool) []byte {
	t.Helper()
	base, oldWork, err := read(true)
	if err != nil {
		t.Fatal(err)
	}
	got, work, err := read(false)
	if err != nil {
		t.Fatal(err)
	}
	if oldWork[0] != work[0] {
		t.Errorf("%s: instance count %d, reference %d", name, work[0], oldWork[0])
	}
	if measure {
		if os.Getenv("DANMAKU_CHECK_DENSE_ONLY") == "1" {
			switch strings.Fields(name)[0] {
			case "Sparse", "Normal":
				if oldWork[1] != work[1] {
					t.Errorf("%s: sparse/normal batching changed %d to %d", name, oldWork[1], work[1])
				}
			case "Dense":
				if work[1] >= oldWork[1] {
					t.Errorf("%s: dense batching did not reduce submissions %d to %d", name, oldWork[1], work[1])
				}
			}
		}
		for _, pair := range []struct {
			label string
			w     [6]int
		}{{"reference", oldWork}, {"candidate", work}} {
			v := pair.w
			t.Logf("WORK %s %s instances=%d batches=%d mask/ClearType-pixels=%d mixed-text-batches=%d mixed-color-batches=%d mixed-glyph-pixels=%d", pair.label, name, v[0], v[1], v[2], v[3], v[4], v[5])
		}
	}
	if !bytes.Equal(base, got) {
		if len(base) != len(got) {
			t.Fatalf("%s: reference size %d, candidate %d", name, len(base), len(got))
		}
		again, _, err := read(true)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(base, again) {
			t.Fatalf("%s: reference renderer is not reproducible on the very same scene", name)
		}
		diff, maxDelta := 0, 0
		for i, v := range base {
			if v != got[i] {
				diff++
				delta := int(v) - int(got[i])
				maxDelta = max(maxDelta, delta, -delta)
			}
		}
		t.Errorf("%s: %d/%d BGRA channels differ, max delta %d", name, diff, len(base), maxDelta)
	}
	return got
}

func TestCompareD3D11Danmaku(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	testDanmakuDamage(t)
	compareDanmakuFrame(t, ui.DanmakuOverlapReader(), "Overlapping translucent glyphs", true)
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
				name := fmt.Sprintf("%s %dx%d@%g %dms active=%d", density.name, size.w, size.h, size.scale, ms, len(n.danmaku.active))
				pixels := compareDanmakuFrame(t, rec.DanmakuTestBGRA, name, i == 1 && size.scale == 1)
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
		pixels := compareDanmakuFrame(t, rec.DanmakuTestBGRA, fmt.Sprintf("Unique %d", at), false)
		t.Logf("FRAME Unique %d %d %d %s %x", ms, len(d.active), len(frame.Glyphs), fmt.Sprint(box), sha256.Sum256(pixels))
	}
}

// Cross sparse/dense transitions, pause, backward seeks and a resize with
// both renderers retaining prior pixels. Also compare against a fresh target.
func testDanmakuDamage(t *testing.T) {
	n, draw := densityPlayer(t, 8)
	n.stream.Pause()
	var history ui.DanmakuHistory
	defer history.Close()
	index := 0
	for _, window := range []struct {
		w, h  int
		scale float32
		times []int64
	}{
		{980, 680, 1, []int64{0, 1500, 2950, 3200, 3750, 5000, 5000, 5017, 701}},
		{740, 524, 1.25, []int64{701, 3900, 5100, 5100, 26000}},
	} {
		rec := ui.NewRecorder(draw, ui.RecorderOptions{Width: window.w, Height: window.h, Scale: window.scale})
		for _, ms := range window.times {
			n.stream.Seek(time.Duration(ms) * time.Millisecond)
			rec.Frame(time.Duration(index) * time.Second / 60)
			index++
			want, refMode, err := history.Draw(rec, true)
			if err != nil {
				t.Fatal(err)
			}
			got, candMode, err := history.Draw(rec, false)
			if err != nil {
				t.Fatal(err)
			}
			full, _, err := rec.DanmakuTestBGRA(true)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(want, got) {
				t.Errorf("retained frame %d at %dms differs (redraw mode %d/%d)", index, ms, refMode, candMode)
			}
			if !bytes.Equal(want, full) {
				t.Errorf("reference retained frame %d at %dms differs from full redraw", index, ms)
			}
			t.Logf("DAMAGE %dms %d active=%d reference=%d candidate=%d", ms, window.w, len(n.danmaku.active), refMode, candMode)
		}
		rec.Close()
	}
}
