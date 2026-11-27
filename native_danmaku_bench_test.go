package main

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Keep the fixture and driver usable on the parity revision for A/B runs.
func benchmarkCues(long, unique bool) []danmakuCue {
	comments := make([]timedComment, 720)
	for i := range comments {
		body := fmt.Sprintf("弾幕 %d　テスト！", i%47)
		if long && i%3 == 0 {
			body = strings.Repeat("長いコメント ABC ", 19)
		}
		var commands []string
		switch i % 13 {
		case 0:
			body += "\n   歌詞"
			commands = []string{"ue", "big", "red"}
		case 1:
			commands = []string{"shita", "black"}
		case 2:
			commands = []string{"small", "marineblue"}
		}
		if unique {
			body = fmt.Sprintf("%s #%d", body, i)
		}
		comments[i] = timedComment{Body: body, VposMs: int64(i * 25), Commands: commands}
	}
	return makeDanmakuCues(comments)
}

func BenchmarkDanmakuFrame(b *testing.B) {
	for _, name := range []string{"Moving", "Unique", "Long", "Paused", "Seek", "ColdSeek", "Gap"} {
		b.Run(name, func(b *testing.B) {
			cues := benchmarkCues(name == "Long", name == "Unique" || name == "ColdSeek")
			var d danmakuOverlay
			ms := int64(5000)
			if name == "Gap" {
				ms = 30000
			}
			r := ui.Rect{X: 10, Y: 20, W: 960, H: 540}
			rec := ui.NewRecorder(func(c *ui.Context) {
				d.draw(c, r, cues, ms)
			}, ui.RecorderOptions{Width: 980, Height: 580})
			defer rec.Close()
			rec.Frame(0)
			// Reset the media clock each block. The backwards seek primes the
			// same lanes each time and is outside the timed moving workload.
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if i%512 == 0 && i > 0 && (name == "Moving" || name == "Long" || name == "Unique") {
					b.StopTimer()
					ms = 5000
					rec.Frame(time.Duration(i) * time.Second / 60)
					b.StartTimer()
				}
				switch name {
				case "Paused", "Gap":
				case "ColdSeek":
					d = danmakuOverlay{}
					ms = 5000 + int64(i%2)*7000
				case "Seek":
					ms = 5000 + int64(i%2)*7000
				default:
					ms = 5000 + int64(i%512)*1000/60
				}
				rec.Frame(time.Duration(i) * time.Second / 60)
			}
		})
	}
}

// Log portable comparison points: compare the hashes on the same machine,
// fonts and toolkit between revisions. Different hosts can use different fonts.
func TestDanmakuFrameSequence(t *testing.T) {
	for _, scale := range []float32{1, 1.5} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			checkDanmakuSequence(t, scale)
		})
	}
}

func checkDanmakuSequence(t *testing.T, scale float32) {
	cues := benchmarkCues(true, false)
	var d danmakuOverlay
	r := ui.Rect{X: 13, Y: 7, W: 320, H: 180}
	var ms int64
	view := ui.NewTester(func(c *ui.Context) { d.draw(c, r, cues, ms) }, 420, 250)
	view.SetScale(scale)
	type frame struct {
		ms   int64
		w, h float32
	}
	var previous frame
	var lastHash [sha256.Size]byte
	for _, f := range []frame{
		{0, 320, 180}, {501, 320, 180}, {5100, 320, 180}, {5100, 320, 180},
		{5117, 320, 180}, {12555, 320, 180}, {1277, 320, 180},
		{1277, 394, 230}, {1355, 394, 230}, {28000, 394, 230},
	} {
		ms, r.W, r.H = f.ms, f.w, f.h
		view.Frame()
		want := 0
		for _, c := range cues {
			if c.start <= ms && ms < c.end {
				want++
			}
		}
		if len(d.active) != want {
			t.Fatalf("at %d ms: active=%d want %d", ms, len(d.active), want)
		}
		hash := sha256.Sum256(view.Image().Pix)
		if f == previous && hash != lastHash {
			t.Fatalf("pixels changed while paused at %d ms", ms)
		}
		previous, lastHash = f, hash
		t.Logf("scale=%v %dms %.0fx%.0f active=%d image=%x", scale, ms, r.W, r.H, want, hash)
	}
}

// A burst of simultaneous comments exercises both first-fit and overflow.
func BenchmarkDanmakuLanes(b *testing.B) {
	var lanes commentLanes
	lanes.reset(540)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := int64(i/16) * 10
		pos := commentPosition(i % 3)
		lanes.place(commentBox{start: start, end: start + 3000, width: float32(30 + i%999), height: float32(17 + i%44)}, pos, 960, 540)
	}
}
