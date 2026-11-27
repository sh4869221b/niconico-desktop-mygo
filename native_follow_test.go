package main

import (
	"fmt"
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func testNativePlaybackStream() (*ui.VideoPlayer, error) {
	return ui.NewVideoPlayer(danmakuTestVideo{image.NewRGBA(image.Rect(0, 0, 40, 30))})
}

func TestCommentListFollowsSeekAndSuspendsForReading(t *testing.T) {
	stream, err := testNativePlaybackStream()
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	stream.Pause()
	n := &nativePlayer{stream: stream, recorded: true}
	for i := range 55 {
		n.allComments = append(n.allComments, timedComment{Body: fmt.Sprintf("コメント %d", i), VposMs: int64(i) * 1000})
	}
	n.applyCommentNG(nil)
	p, b := &playerWindow{native: n}, &browser{}
	view := ui.NewTester(func(c *ui.Context) { b.drawPlayer(p, c) }, 960, 560)
	stream.Seek(30 * time.Second)
	view.Frame()
	if first, _ := n.commentList.Visible(); first != 30 || stream.Playing() {
		t.Fatalf("paused forward seek led to row %d", first)
	}
	view.Scroll(820, 260, 0, -120)
	if !n.follow.manual {
		t.Fatal("manual scrolling kept auto-follow enabled")
	}
	stream.Seek(10 * time.Second)
	view.Frame()
	if first, _ := n.commentList.Visible(); first == 10 {
		t.Fatal("a seek interrupted manual scrolling")
	}
	if err := view.Click("再生位置に追従"); err != nil {
		t.Fatal(err)
	}
	if first, _ := n.commentList.Visible(); first != 10 {
		t.Fatalf("resuming follow led to row %d", first)
	}
}

func TestDenseStartCommentsAreStagedUntilVideoCanDisplay(t *testing.T) {
	n := &nativePlayer{recorded: true, status: "読み込み中", loadingStream: true}
	p, b := &playerWindow{native: n}, &browser{}
	rows := make([]timedComment, 15000)
	for i := range rows {
		rows[i] = timedComment{Body: "冒頭", VposMs: 0}
	}
	n.stagedComments = &nativeCommentData{rows: rows, cues: makeDanmakuCues(rows)}
	view := ui.NewTester(func(c *ui.Context) { b.drawPlayer(p, c) }, 960, 560)
	if err := view.Click("一時停止"); err != nil {
		t.Fatal(err)
	}
	if !n.pauseRequested || len(n.commentRows) != 0 || n.stagedComments == nil {
		t.Fatal("network startup displayed comments or ignored the pause")
	}
	var err error
	n.stream, err = testNativePlaybackStream()
	if err != nil {
		t.Fatal(err)
	}
	defer n.stream.Close()
	n.loadingStream = false
	n.stream.Pause()
	view.Frame()
	if len(n.commentRows) != len(rows) || n.stagedComments != nil || n.follow.lastRow != 0 {
		t.Fatal("dense, same-time comments did not appear at the initial position")
	}
	if _, scroll := n.follow.row(5*time.Millisecond, n.allComments, n.commentRows); scroll {
		t.Fatal("the same timestamp group scheduled another scroll")
	}
}
