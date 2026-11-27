package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// An in-memory 4:3 video drives the same view as the native player window.
type danmakuTestVideo struct{ frame *image.RGBA }

func (v danmakuTestVideo) Next() (ui.VideoFrame, error) {
	return ui.VideoFrame{Image: v.frame, Duration: time.Minute}, nil
}

func TestNativePlayerStartupTimeout(t *testing.T) {
	type playback struct{ current, failed *nativePlayer }
	newView := func() (*ui.Tester, *playback) {
		state := &playback{}
		view := ui.NewTester(func(c *ui.Context) {
			if n := state.current; n != nil && n.startExpired(c) {
				state.failed, state.current = n, nil
			}
		}, 320, 240)
		return view, state
	}
	t.Run("initially minimized and repeatedly restored", func(t *testing.T) {
		view, state := newView()
		view.SetWindowHidden(true)
		player := &nativePlayer{}
		state.current = player
		view.Frame()
		view.Advance(time.Minute)
		view.SetWindowHidden(false)
		view.Advance(6 * time.Second)
		view.SetWindowHidden(true)
		view.Advance(time.Minute)
		view.SetWindowHidden(false)
		view.Advance(4 * time.Second)
		view.SetWindowHidden(true)
		view.Advance(time.Minute)
		view.SetWindowHidden(false)
		view.Advance(10*time.Second - time.Millisecond)
		if state.failed != nil {
			t.Fatal("startup expired before 20 visible seconds")
		}
		view.Advance(time.Millisecond)
		if state.failed != player {
			t.Fatal("a visible stalled startup did not expire after using its remaining allowance")
		}
	})
	t.Run("hidden notification at deadline", func(t *testing.T) {
		view, state := newView()
		player := &nativePlayer{startDeadline: time.Microsecond}
		state.current = player
		view.Frame()
		// Frame timers dispatch at least one millisecond after arming. Make
		// the hide event reach the view at the deadline while the wake is pending.
		view.Advance(time.Microsecond)
		view.SetWindowHidden(true)
		view.Advance(time.Minute)
		if state.failed != nil {
			t.Fatal("a due timeout closed the minimized player")
		}
		view.SetWindowHidden(false)
		if state.failed != player {
			t.Fatal("restoring a stalled player with an exhausted allowance did not report the timeout")
		}
	})
	t.Run("video replaced or closed before deadline", func(t *testing.T) {
		view, state := newView()
		state.current = &nativePlayer{}
		view.Frame()
		view.Advance(nativeStartTimeout - time.Second)
		replacement := &nativePlayer{}
		state.current = replacement
		// Let the old scheduled wake reach the new player before a fresh build.
		view.Advance(time.Second)
		if state.failed != nil || state.current != replacement || replacement.startDeadline <= nativeStartTimeout {
			t.Fatal("the previous video's deadline affected its replacement")
		}
		state.current = nil
		view.Advance(time.Minute)
		if state.failed != nil {
			t.Fatal("a queued wake affected a closed player")
		}
	})
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

func TestPlayerOverviewAndPublisherLeaveVideoPlaying(t *testing.T) {
	frame := image.NewRGBA(image.Rect(0, 0, 40, 30))
	green := color.RGBA{34, 215, 91, 255}
	draw.Draw(frame, frame.Bounds(), image.NewUniform(green), image.Point{}, draw.Src)
	stream, err := ui.NewVideoPlayer(danmakuTestVideo{frame})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	_, _, description := plainDescription(`<a href="https://www.nicovideo.jp/mylist/456">概要のマイリスト</a><br><a href="https://www.nicovideo.jp/watch/sm8">概要の動画</a><br>` + strings.Repeat("長い概要です。", 400))
	tags := make([]string, 24)
	for i := range tags {
		tags[i] = fmt.Sprintf("%d %s", i, strings.Repeat("長いタグ", 15))
	}
	n := &nativePlayer{stream: stream, recorded: true, description: description,
		tags: tags, tagNotice: "タグはありません。", commentNotice: "コメント欄の内容",
		publisher: publisherFor("user", "123", "投稿者名")}
	p := &playerWindow{native: n}
	b := &browser{player: p, index: -1, closed: true}
	view := ui.NewTester(func(c *ui.Context) { b.drawPlayer(p, c) }, 960, 560)
	click := func(name string) {
		t.Helper()
		if err := view.Click(name); err != nil {
			t.Fatal(err)
		}
	}
	click("概要欄を表示")
	if !n.showOverview || !view.HasText("コメント欄の内容") || !view.HasText("概要のマイリスト") || !view.HasText("概要の動画") {
		t.Fatal("overview links and comments should be visible together")
	}
	click("mylist/456")
	if r := b.navigation[b.index]; r.kind != screenList || r.key != "456" || b.player != p || !stream.Playing() {
		t.Fatalf("overview link did not open in the main window: %+v", r)
	}
	click("sm8")
	if b.playbackRequest != 1 {
		t.Fatal("overview video link did not use the player entry point")
	}
	click("投稿者名の投稿動画")
	if r := b.navigation[b.index]; r.kind != screenUser || r.key != "123" || b.player != p || !stream.Playing() {
		t.Fatalf("publisher link disturbed playback or used the wrong route: %+v", r)
	}
	click("シリーズ")
	if r := b.navigation[b.index]; r.kind != screenUserSeries || r.key != "123" || b.player != p || !stream.Playing() {
		t.Fatalf("publisher series link disturbed playback or used the wrong route: %+v", r)
	}
	click("コメント一覧")
	if !n.showOverview || view.HasText("コメント欄の内容") {
		t.Fatal("closing comments affected the overview")
	}
	click("コメント一覧")
	click("概要欄を非表示")
	if n.showOverview || !view.HasText("コメント欄の内容") {
		t.Fatal("closing the overview affected the comments")
	}
	click("概要欄を表示")
	view.SetSize(560, 420)
	checkPicture := func(scale string) {
		t.Helper()
		img := view.Image()
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				if img.RGBAAt(x, y) == green {
					return
				}
			}
		}
		t.Fatalf("video disappeared with expanded overview, tags and comments at %s", scale)
	}
	focus := func(label string) {
		t.Helper()
		for range 90 {
			if view.Focused(label) {
				return
			}
			view.Key(0, ui.KeyTab)
		}
		t.Fatalf("could not focus %s", label)
	}
	scales := []float32{1, 1.5, 2}
	for _, scale := range scales {
		view.SetPreferences(ui.Preferences{TextScale: scale})
		checkPicture(fmt.Sprintf("%.1f text scale", scale))
	}
	view.SetPreferences(ui.Preferences{})
	view.SetZoom(2)
	checkPicture("2× UI zoom")
	view.SetZoom(1)
	for _, scale := range scales {
		view.SetPreferences(ui.Preferences{TextScale: scale})
		focus("一時停止")
		view.Key(0, ui.KeyEnter)
		if stream.Playing() {
			t.Fatalf("pause control unavailable at text scale %.1f", scale)
		}
		focus("再生位置")
		position := stream.Position()
		view.Key(0, ui.KeyRight)
		if stream.Position() <= position {
			t.Fatalf("seek control unavailable at text scale %.1f", scale)
		}
		focus("音量")
		volume := stream.Volume()
		view.Key(0, ui.KeyLeft)
		if stream.Volume() >= volume {
			t.Fatalf("volume control unavailable at text scale %.1f", scale)
		}
		focus("再生")
		view.Key(0, ui.KeyEnter)
		if !stream.Playing() {
			t.Fatalf("resume control unavailable at text scale %.1f", scale)
		}
	}
}

func TestNativePlayerKeepsPlayingWithDenseOrFailedComments(t *testing.T) {
	const commentCount = 15000
	for _, test := range []struct {
		name, notice string
		count        int
	}{
		{name: "dense", count: commentCount},
		{name: "failed", notice: "コメントを取得できませんでした。"},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := make([]timedComment, test.count)
			for i := range rows {
				rows[i] = timedComment{Body: fmt.Sprintf("コメント %d", i), VposMs: int64(i / 100)}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			stream, err := ui.NewVideoPlayer(danmakuTestVideo{image.NewRGBA(image.Rect(0, 0, 40, 30))})
			if err != nil {
				t.Fatal(err)
			}
			defer stream.Close()
			n := &nativePlayer{cancel: cancel, stream: stream, recorded: true, showComments: true,
				stagedComments: &nativeCommentData{rows: rows, cues: makeDanmakuCues(rows), notice: test.notice}}
			p := &playerWindow{native: n}
			b := &browser{}
			view := ui.NewTester(func(c *ui.Context) { b.drawPlayer(p, c) }, 960, 560)
			view.Advance(nativeStartTimeout + time.Second)
			if n.stream != stream || !stream.Playing() || ctx.Err() != nil || n.status != "" {
				t.Fatalf("comments stopped playback: %q", n.status)
			}
			if len(n.commentRows) != test.count || n.stagedComments != nil || n.commentNotice != test.notice {
				t.Fatalf("comments did not reach the view: %d rows, %q", len(n.commentRows), n.commentNotice)
			}
			if test.notice != "" && !slices.Contains(view.Texts(), test.notice) {
				t.Fatal("the comment error is missing from the comment panel")
			}
		})
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
	click("弾幕を表示")
	if n.showComments || hasRed() {
		t.Fatal("hidden overlay still painted")
	}
	click("弾幕を表示")
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
