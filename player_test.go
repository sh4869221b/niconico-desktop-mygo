package main

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestPlayerHistoryAcrossSameDocumentNavigation(t *testing.T) {
	a := home + "watch/sm9"
	second := home + "watch/sm12"
	p := &playerWindow{document: playerDocument{loaded: true}}
	b := &browser{win: new(mygo.Window), player: p, libraryPath: filepath.Join(t.TempDir(), "library.json")}
	d := &p.document
	observe := func(raw string, response []any) {
		route := routeFor(raw)
		if d.observe(raw, route) && d.video != "" {
			p.visitWebVideo(d.video)
			b.recordVideo(p, d.video, d.title)
		}
		if d.acceptTitle(d.revision, route.link, watchResponseTitle(response, raw, route.key)) {
			b.recordVideo(p, d.video, d.title)
		}
	}
	requireTitle := func(raw, want string) {
		t.Helper()
		for _, item := range b.data.History {
			if item.URL == raw && item.Title == want {
				return
			}
		}
		t.Fatalf("missing %q with title %q in %+v", raw, want, b.data.History)
	}

	observe(a, []any{a, "sm9", "同じタイトル"})
	firstRevision := d.revision
	observe(a, []any{a, "sm12", "別動画の情報が先に更新"})
	observe(second+"?from=60", []any{second + "?from=60", "sm9", "同じタイトル"})
	requireTitle(a, "同じタイトル")
	requireTitle(second, second)
	if d.acceptTitle(firstRevision, a, "遅れて届いたAの応答") {
		t.Fatal("accepted metadata from the previous video")
	}
	observe(second+"?from=60", []any{second + "?from=60", "sm12", "同じタイトル"})
	observe(second+"#comments", []any{second + "#comments", "sm12", "同じタイトル"})
	requireTitle(second, "同じタイトル")
	observe(login, nil)
	if len(b.data.History) != 2 || b.data.History[0].URL != second {
		t.Fatalf("unexpected history: %+v", b.data.History)
	}
	observe(a, nil)
	if d.acceptTitle(firstRevision, a, "初回のナビゲーションから遅れて届いた応答") {
		t.Fatal("a previous visit supplied the title even though the video ID matched")
	}
}

func TestPlayerLoadFailureBelongsToPendingNavigation(t *testing.T) {
	a, b := home+"watch/sm9", home+"watch/sm12"
	var d playerDocument
	d.begin(a)
	d.begin(b)
	if d.finish(a) || d.failed(a, a) {
		t.Fatal("a previous request was accepted before the new document committed")
	}
	d.commit(b)
	if d.failed(a, b) || !d.finish(b) {
		t.Fatal("the current document did not finish cleanly")
	}
	if d.failed(a, b) || d.failed(b, b) || !d.loaded {
		t.Fatal("a delayed failure invalidated the completed load")
	}
	d.observe(a, routeFor(a)) // same-document movement remains trackable
	if d.video != a {
		t.Fatal("same-document navigation was lost")
	}
	d.begin(b)
	d.commit(b)
	if !d.failed(b, b) || d.loaded {
		t.Fatal("failed to detect a genuine error on a pending load")
	}
	if !d.finish(a) {
		t.Fatal("did not accept a finish after pushState changed the committed document's URL")
	}
}

func TestWebFallbackAppliesToOneVideo(t *testing.T) {
	a := routeFor(home + "watch/sm9?from=60")
	b := routeFor(home + "watch/sm12?from=120")
	p := &playerWindow{mode: webPlayback, webVideo: a.link, document: playerDocument{loaded: true}}
	p.document.observe(a.target, a)
	if p.needsNativePlayer(a, nativePlayback) || p.needsNativePlayer(routeFor(login), nativePlayback) {
		t.Fatal("the fallback or sign-in did not stay in WebView")
	}
	if !p.needsNativePlayer(b, nativePlayback) || p.needsNativePlayer(b, webPlayback) {
		t.Fatal("the next video did not follow the regular playback preference")
	}
	// The native handoff uses the new route's playback URL without inheriting A's options.
	if b.target != home+"watch/sm12?from=120" {
		t.Fatal(b.target)
	}
}

func TestHistoryDeletionAndExplicitReplay(t *testing.T) {
	target := home + "watch/sm9"
	p := &playerWindow{}
	b := &browser{win: new(mygo.Window), player: p, history: true, index: 0,
		navigation: []screenRoute{routeFor(home + "search/test")}, libraryPath: filepath.Join(t.TempDir(), "library.json")}
	b.recordVideo(p, target, "動画")
	b.remove(b.data.History[0])
	b.loading() // unrelated navigation in the main window
	b.recordVideo(p, target+"#comments", "タイトル更新")
	if len(b.data.History) != 0 {
		t.Fatalf("a player update restored deleted history: %+v", b.data.History)
	}

	r := routeFor(target + "?from=60")
	p.resumeHistory(r.link)
	b.recordVideo(p, r.target, "動画")
	b.recordVideo(p, r.target, "")
	if len(b.data.History) != 1 || b.data.History[0].URL != target || b.data.History[0].Title != "動画" {
		t.Fatalf("history after replay: %+v", b.data.History)
	}

	b.player = &playerWindow{}
	b.recordVideo(p, r.target, "閉じたプレーヤーの更新")
	if b.data.History[0].Title != "動画" {
		t.Fatalf("a replaced player updated history: %+v", b.data.History)
	}
}

func TestDeletedHistoryReturnsOnWebBackToAnotherVideo(t *testing.T) {
	a, next := home+"watch/sm9", home+"watch/sm12"
	p := &playerWindow{webVideo: a, document: playerDocument{loaded: true}}
	b := &browser{win: new(mygo.Window), player: p, history: true, libraryPath: filepath.Join(t.TempDir(), "library.json")}
	b.recordVideo(p, a, "動画A")
	b.remove(b.data.History[0])
	p.visitWebVideo(a) // a delayed notification or a fragment change
	b.recordVideo(p, a+"#comments", "動画A")
	if len(b.data.History) != 0 {
		t.Fatal("same-video update restored history")
	}
	p.visitWebVideo(next)
	b.recordVideo(p, next, "動画B")
	p.visitWebVideo(a) // back from B starts a new visit to A
	b.recordVideo(p, a, "動画A")
	if len(b.data.History) != 2 || b.data.History[0].URL != a {
		t.Fatalf("missing history after returning from another video: %+v", b.data.History)
	}
}

func TestPlayerShowsCommentLanguageWarningAlongsideStatus(t *testing.T) {
	p := &playerWindow{status: "プレーヤーの状態", commentLanguageWarning: true}
	view := ui.NewTester(p.draw, 640, 420)
	if !view.HasText(p.status) || !view.HasText(commentLanguageWarning) {
		t.Fatalf("player text: %v", view.Texts())
	}
}
