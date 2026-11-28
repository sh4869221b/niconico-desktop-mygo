package main

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Device scale transitions exercise app rendering/hit testing in DIP space.
// They do not emulate WM_DPICHANGED or Media Foundation's native surface.
func TestPlayerHitTargetsAcrossDeviceScaleTransitions(t *testing.T) {
	green := color.RGBA{34, 215, 91, 255}
	frame := image.NewRGBA(image.Rect(0, 0, 40, 30))
	draw.Draw(frame, frame.Bounds(), image.NewUniform(green), image.Point{}, draw.Src)
	stream, err := ui.NewVideoPlayer(danmakuTestVideo{frame})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	n := &nativePlayer{stream: stream, recorded: true, allComments: []timedComment{{Body: "DPI comment row", VposMs: 0}}}
	n.applyCommentNG(nil)
	p := &playerWindow{native: n}
	b := &browser{}
	view := ui.NewTester(func(c *ui.Context) { b.drawPlayer(p, c) }, 1400, 650)
	for _, size := range [][2]int{{1400, 650}, {1500, 700}} {
		stream.Pause()
		view.SetSize(size[0], size[1])
		view.SetScale(1)
		baseline, ok := view.Access("再生位置")
		if !ok {
			t.Fatal("seek slider missing")
		}
		for _, scale := range []float32{1, 1.25, 1.5, 2, 1.5, 1.25, 1} {
			view.SetScale(scale)
			slider, ok := view.Access("再生位置")
			if !ok || slider.Bounds != baseline.Bounds {
				t.Fatalf("scale %.1f changed seek DIP bounds: %+v vs %+v", scale, slider.Bounds, baseline.Bounds)
			}
			stream.Pause()
			view.Frame()
			img := view.Image()
			if img.Bounds().Dx() != int(float32(size[0])*scale) {
				t.Fatal("snapshot did not use device pixels")
			}
			picture := image.Rectangle{Min: image.Pt(img.Bounds().Dx(), img.Bounds().Dy())}
			for y := 0; y < img.Bounds().Dy(); y++ {
				for x := 0; x < img.Bounds().Dx(); x++ {
					if img.RGBAAt(x, y) == green {
						picture.Min.X = min(picture.Min.X, x)
						picture.Min.Y = min(picture.Min.Y, y)
						picture.Max.X = max(picture.Max.X, x+1)
						picture.Max.Y = max(picture.Max.Y, y+1)
					}
				}
			}
			if picture.Empty() {
				t.Fatalf("video vanished after scale %.1f", scale)
			}
			if math.Abs(float64(picture.Dx())/float64(picture.Dy())-4.0/3) > 0.02 {
				t.Fatal("video aspect ratio changed")
			}
			cx, cy := float32(picture.Min.X+picture.Max.X)/2/scale, float32(picture.Min.Y+picture.Max.Y)/2/scale
			view.ClickAt(cx, cy)
			if !stream.Playing() {
				t.Fatalf("picture center did not resume at scale %.1f", scale)
			}
			view.ClickAt(cx, cy)
			if stream.Playing() {
				t.Fatalf("picture center did not pause at scale %.1f", scale)
			}
			// Verify that this probe is inside the adjacent black letterbox.
			bx, by := picture.Min.X-3, int(cy*scale)
			if bx < 0 || img.RGBAAt(bx, by) != (color.RGBA{0, 0, 0, 255}) {
				t.Fatalf("fixture has no black side bar at scale %.1f", scale)
			}
			view.ClickAt(float32(bx)/scale, float32(by)/scale)
			if stream.Playing() {
				t.Fatalf("letterbox toggled playback at scale %.1f", scale)
			}
			clickSeek := func(fraction float32) time.Duration {
				r := slider.Bounds
				view.ClickAt(r.X+r.W*fraction, r.Y+r.H/2)
				if stream.Playing() {
					t.Fatal("paused seek resumed playback")
				}
				return stream.Position()
			}
			left, right := clickSeek(0.25), clickSeek(0.75)
			if left <= 0 || right <= left || right >= time.Minute {
				t.Fatalf("seek targets lost order after scale %.1f: %v, %v", scale, left, right)
			}
			row, ok := view.Find("DPI comment row")
			if !ok || row.W <= 0 || row.H <= 0 {
				t.Fatalf("comment row disappeared at scale %.1f", scale)
			}
			jump, ok := view.Find(formatPlaybackTime(0) + "へ移動")
			if !ok {
				t.Fatalf("comment timestamp is missing at scale %.2f", scale)
			}
			view.ClickAt(jump.X+jump.W/2, jump.Y+jump.H/2)
			if stream.Position() != 0 || stream.Playing() {
				t.Fatalf("comment timestamp did not seek to the start and stay paused at scale %.2f", scale)
			}
		}
	}
}
