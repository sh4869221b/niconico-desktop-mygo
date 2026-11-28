package main

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestMetadataRetryControlsPreservePlaybackAndBrowsing(t *testing.T) {
	s, err := testNativePlaybackStream()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Pause()
	s.Seek(12 * time.Second)
	n := &nativePlayer{stream: s, recorded: true, tagFailed: true, commentsFailed: true, tagNotice: "タグを取得できませんでした。", commentNotice: "コメントを取得できませんでした。", follow: commentFollow{manual: true}}
	p := &playerWindow{native: n}
	b := &browser{player: p}
	v := ui.NewTester(func(c *ui.Context) { b.drawPlayer(p, c) }, 960, 560)
	for _, label := range []string{"タグを再取得", "コメントを再取得"} {
		if _, ok := v.Find(label); !ok {
			t.Fatalf("missing %s", label)
		}
	}
	n.stagedComments = &nativeCommentData{rows: []timedComment{{Body: "retried"}}}
	v.Frame()
	if s.Position() != 12*time.Second || s.Playing() || !n.follow.manual || len(n.commentRows) != 1 {
		t.Fatal("installing retry results disturbed playback/manual browsing")
	}
}

func TestMetadataRejectsStalePlayerVideoAndGeneration(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	r := routeFor(home + "watch/sm9")
	n := &nativePlayer{metadataCtx: ctx, metadataRoute: r, tagGeneration: 2, commentGeneration: 3}
	p := &playerWindow{native: n, target: r.target}
	b := &browser{player: p}
	if !b.acceptsNativeMetadata(p, n, true, 2) || !b.acceptsNativeMetadata(p, n, false, 3) {
		t.Fatal("rejected current request")
	}
	if b.acceptsNativeMetadata(p, n, true, 1) || b.acceptsNativeMetadata(p, n, false, 2) {
		t.Fatal("accepted old generation")
	}
	p.target = home + "watch/sm12"
	if b.acceptsNativeMetadata(p, n, true, 2) {
		t.Fatal("accepted different video")
	}
	p.target = r.target
	p.native = &nativePlayer{}
	if b.acceptsNativeMetadata(p, n, true, 2) {
		t.Fatal("accepted replaced player")
	}
	p.native = n
	cancel()
	if b.acceptsNativeMetadata(p, n, true, 2) {
		t.Fatal("accepted cancelled request")
	}
}
