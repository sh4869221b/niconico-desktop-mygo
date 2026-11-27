package main

import (
	"image"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// An in-memory 4:3 video drives the same view as the native player window.
type danmakuTestVideo struct{ frame *image.RGBA }

func (v danmakuTestVideo) Next() (ui.VideoFrame, error) {
	return ui.VideoFrame{Image: v.frame, Duration: time.Minute}, nil
}

func TestNativePlayerPlaybackAndVolumeControls(t *testing.T) {
	stream, err := ui.NewVideoPlayer(danmakuTestVideo{image.NewRGBA(image.Rect(0, 0, 40, 30))})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	stream.SetVolume(defaultNativeVolume / 100.0)
	player := &playerWindow{native: &nativePlayer{stream: stream, recorded: true}}
	b := &browser{nativeVolume: defaultNativeVolume}
	view := ui.NewTester(func(c *ui.Context) { b.drawPlayer(player, c) }, 960, 560)
	view.ClickAt(300, 220)
	if stream.Playing() {
		t.Fatal("clicking the picture did not pause")
	}
	view.RightClickAt(300, 220)
	if stream.Playing() {
		t.Fatal("right-clicking the picture changed playback")
	}
	view.ClickAt(16, 220)
	if stream.Playing() {
		t.Fatal("clicking the letterbox changed playback")
	}
	if err := view.Click("再生位置"); err != nil {
		t.Fatal(err)
	}
	if stream.Playing() {
		t.Fatal("seeking changed play state")
	}
	for i := 0; i < 20 && !view.Focused("音量"); i++ {
		view.Key(0, ui.KeyTab)
	}
	if !view.Focused("音量") {
		t.Fatal("volume slider is not keyboard focusable")
	}
	view.Key(0, ui.KeyLeft)
	if stream.Volume() != 0.79 || b.nativeVolume != 79 || stream.Playing() {
		t.Fatalf("volume control changed volume to %.2f and play state to %t", stream.Volume(), stream.Playing())
	}
	view.ClickAt(300, 220)
	if !stream.Playing() {
		t.Fatal("clicking the picture did not resume")
	}
	view.SetSize(560, 420)
	if err := view.Click("一時停止"); err != nil {
		t.Fatal(err)
	}
	if stream.Playing() {
		t.Fatal("playback controls did not work at minimum size")
	}
}

func TestNativePlayerCommentMenuRemovesActiveAuthorAndRestores(t *testing.T) {
	stream, err := ui.NewVideoPlayer(danmakuTestVideo{image.NewRGBA(image.Rect(0, 0, 40, 30))})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	author := commentAuthor{ID: "123"}
	n := &nativePlayer{stream: stream, showComments: true, recorded: true, startAt: time.Second}
	n.allComments = []timedComment{
		{Body: "NG対象", VposMs: 1000, Commands: []string{"ue", "red"}, author: author},
		{Body: "同じ投稿者", VposMs: 1000, Commands: []string{"shita", "red"}, author: author},
		{Body: "同じ投稿者", VposMs: 1000, Commands: []string{"ue", "green"}, author: commentAuthor{ID: "456"}},
		{Body: "IDなし", VposMs: 1000, Commands: []string{"ue"}},
	}
	n.allCues = makeDanmakuCues(n.allComments)
	n.applyCommentNG(nil)
	b := &browser{commentNG: commentNG{path: filepath.Join(t.TempDir(), "comment-ng.json")}}
	player := &playerWindow{native: n}
	view := ui.NewTester(func(c *ui.Context) { b.drawPlayer(player, c) }, 960, 560)
	if err := view.Click("一時停止"); err != nil {
		t.Fatal(err)
	}
	if len(n.danmaku.active) != 4 {
		t.Fatalf("active comments before NG: %d", len(n.danmaku.active))
	}
	if err := view.RightClick("IDなし"); err != nil {
		t.Fatal(err)
	}
	if err := view.ChooseMenuItem("このユーザーを NG"); err == nil {
		t.Fatal("was able to block a comment without an author")
	}
	view.CloseMenu()
	if err := view.RightClick("NG対象"); err != nil {
		t.Fatal(err)
	}
	if err := view.ChooseMenuItem("このユーザーを NG"); err != nil {
		t.Fatal(err)
	}
	view.Frame()
	if !slices.Equal(n.commentRows, []int{2, 3}) || len(n.danmaku.active) != 2 ||
		slices.ContainsFunc(n.danmaku.active, func(c drawnComment) bool { return c.style.rgb == 0xff0000 }) {
		t.Fatalf("NG did not update list and active overlay: %v / %v", n.commentRows, n.danmaku.active)
	}
	if slices.Contains(view.Texts(), "NG対象") {
		t.Fatal("blocked row is still displayed")
	}
	if err := view.Click("NG管理"); err != nil {
		t.Fatal(err)
	}
	if err := view.Click("解除 ユーザーID 123"); err != nil {
		t.Fatal(err)
	}
	view.Frame()
	if len(n.commentRows) != 4 || len(n.danmaku.active) != 4 || b.commentNG.blocked[author] {
		t.Fatal("NG removal did not restore the list and currently active comments")
	}
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
		stream: stream, recorded: true, showComments: true, startAt: routeFor(home + "watch/sm9?from=1").startTime(),
		comments: makeDanmakuCues([]timedComment{
			{Body: "TEST", VposMs: 1000, Commands: []string{"ue", "red", "big"}},
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
	if n.startAt != 0 || stream.Position() < time.Second {
		t.Fatal("native player did not apply the requested start time")
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
