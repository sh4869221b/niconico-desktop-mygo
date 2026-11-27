package main

import (
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// An in-memory 4:3 video drives the same view as the native player window.
type danmakuTestVideo struct{ frame *image.RGBA }

func (v danmakuTestVideo) Next() (ui.VideoFrame, error) {
	return ui.VideoFrame{Image: v.frame, Duration: time.Minute}, nil
}
func (danmakuTestVideo) Rewind() error           { return nil }
func (danmakuTestVideo) Duration() time.Duration { return time.Minute }

func TestPlayerDanmakuControls(t *testing.T) {
	stream, err := ui.NewVideoPlayer(danmakuTestVideo{image.NewRGBA(image.Rect(0, 0, 40, 30))})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	n := &nativePlayer{
		stream: stream, recorded: true, showComments: true,
		comments: makeDanmakuCues([]timedComment{
			{Body: "TEST", Commands: []string{"ue", "red", "big"}},
		}),
	}
	p := &playerWindow{native: n}
	b := &browser{}
	view := ui.NewTester(func(c *ui.Context) { b.drawPlayer(p, c) }, 760, 420)
	click := func(label string) {
		t.Helper()
		if err := view.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	hasRed := func() bool {
		img := view.Image()
		for y := 0; y < img.Rect.Dy(); y++ {
			for x := 0; x < img.Rect.Dx(); x++ {
				p := img.RGBAAt(x, y)
				if p.R > 180 && p.G < 80 && p.B < 80 {
					return true
				}
			}
		}
		return false
	}
	click("一時停止")
	if stream.Playing() || !hasRed() {
		t.Fatal("expected the paused video with a red overlay")
	}
	click("コメント表示切替")
	if n.showComments || hasRed() {
		t.Fatal("hidden overlay still painted")
	}
	click("コメント表示切替")
	if !n.showComments || !hasRed() {
		t.Fatal("overlay did not return")
	}
	view.SetSize(580, 460)
	view.Frame()
	r := n.commentViewport
	if want := danmakuBounds(r, videoBounds(r, 40, 30)); n.danmaku.width != want.W || n.danmaku.height != want.H || !hasRed() {
		t.Fatalf("resized overlay %.1fx%.1f, want %+v", n.danmaku.width, n.danmaku.height, want)
	}
	stream.Seek(6 * time.Second)
	view.Frame()
	if hasRed() {
		t.Fatal("expired comment remained after seek")
	}
	stream.Seek(time.Second)
	view.Frame()
	if !hasRed() || stream.Playing() {
		t.Fatal("backward seek did not restore the comment while paused")
	}
	click("再生")
	if !stream.Playing() {
		t.Fatal("did not resume")
	}
}
