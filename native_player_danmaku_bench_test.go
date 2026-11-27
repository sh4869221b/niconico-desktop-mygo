package main

import (
	"crypto/sha256"
	"fmt"
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func densityCues(interval int) []danmakuCue {
	var comments []timedComment
	for i := 0; i*interval < 20000; i++ {
		body := fmt.Sprintf("弾幕 %02d テスト！", i%47)
		var commands []string
		switch i % 17 {
		case 0:
			body += "\n　歌詞 ABC"
			commands = []string{"ue", "big", "red"}
		case 1:
			commands = []string{"shita", "black"}
		case 2:
			commands = []string{"small", "marineblue"}
		case 3:
			body = strings.Repeat("長いコメント abc ", 12)
		case 4:
			body = "مرحبا 🧑🏽‍🚀 か\u3099"
		}
		comments = append(comments, timedComment{Body: body, VposMs: int64(interval * i), Commands: commands})
	}
	return makeDanmakuCues(comments)
}

var danmakuDensities = []struct {
	name     string
	interval int
}{
	{"Sparse", 1000}, {"Normal", 100}, {"Dense", 8},
}

func densityPlayer(t testing.TB, interval int) (*nativePlayer, func(*ui.Context)) {
	t.Helper()
	stream, err := ui.NewVideoPlayer(danmakuTestVideo{image.NewRGBA(image.Rect(0, 0, 40, 30))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stream.Close)
	n := &nativePlayer{stream: stream, recorded: true, showComments: true, comments: densityCues(interval)}
	p := &playerWindow{native: n}
	br := &browser{}
	return n, func(c *ui.Context) { br.drawPlayer(p, c) }
}

// Render the player window with the same decoder, media time and frame sizes
// at three arrival rates. Recorder builds the CPU scene without a native GPU.
func BenchmarkPlayerDanmakuDensity(b *testing.B) {
	for _, tc := range danmakuDensities {
		b.Run(tc.name, func(b *testing.B) {
			n, draw := densityPlayer(b, tc.interval)
			rec := ui.NewRecorder(draw, ui.RecorderOptions{Width: 980, Height: 680})
			defer rec.Close()
			n.stream.Seek(5 * time.Second)
			rec.Frame(0)
			samples := make([]time.Duration, b.N)
			total := 0
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				at := time.Duration(i) * time.Second / 60
				if i > 0 && i%384 == 0 {
					b.StopTimer()
					n.stream.Seek(5 * time.Second)
					rec.Frame(at)
					b.StartTimer()
				}
				start := time.Now()
				rec.Frame(at)
				samples[i] = time.Since(start)
				total += len(n.danmaku.active)
			}
			b.StopTimer()
			slices.Sort(samples)
			b.ReportMetric(float64(samples[b.N-1-(b.N/20)].Nanoseconds())/1e6, "ms/p95")
			b.ReportMetric(float64(total)/float64(b.N), "active/frame")
		})
	}
}

// On the same host and fonts, compare the logged RGBA digests across revisions.
// Check the actual window's ordered comment labels against the cue lifetimes and
// the unclipped part of the viewport with the deterministic software renderer.
func TestPlayerDanmakuDensityFrames(t *testing.T) {
	for _, tc := range danmakuDensities {
		t.Run(tc.name, func(t *testing.T) {
			n, draw := densityPlayer(t, tc.interval)
			view := ui.NewTester(draw, 560, 360)
			n.stream.Pause()
			bodies := make(map[string]bool)
			for _, cue := range n.comments {
				bodies[cue.body] = true
			}
			check := func(ms int64) [sha256.Size]byte {
				t.Helper()
				n.stream.Seek(time.Duration(ms) * time.Millisecond)
				view.Frame()
				var active int
				for _, cue := range n.comments {
					if cue.start <= ms && ms < cue.end {
						active++
					}
				}
				d := &n.danmaku
				if len(d.active) != active {
					t.Fatalf("%dms: %d active, want %d", ms, len(d.active), active)
				}
				var visible, labels []string
				for _, item := range d.active {
					if item.style.position == scrollComment {
						x := item.x(ms, d.width)
						if x >= d.width || x+item.width <= 0 {
							continue
						}
					}
					visible = append(visible, item.body)
				}
				for _, label := range view.Texts() {
					if bodies[label] {
						labels = append(labels, label)
					}
				}
				if !slices.Equal(labels, visible) {
					t.Fatalf("%dms: got %d drawn labels, want %d in cue order", ms, len(labels), len(visible))
				}
				sum := sha256.Sum256(view.Image().Pix)
				t.Logf("%dms active=%d drawn=%d image=%x", ms, active, len(labels), sum)
				return sum
			}
			for frame := range 24 {
				check(5000 + int64(frame)*1000/60)
			}
			check(14000)
			sum := check(2733)
			if check(2733) != sum {
				t.Fatal("paused frame changed")
			}
			view.SetSize(468, 386)
			check(2733)
			view.SetScale(1.25)
			check(2750)
			check(25000)
		})
	}
}
