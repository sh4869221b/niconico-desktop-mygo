package main

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

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

func TestPlayerResumesObservingAfterPrecommitFailure(t *testing.T) {
	a, next := home+"watch/sm9", home+"watch/sm12"
	d := playerDocument{loaded: true}
	d.observe(a, routeFor(a))
	d.begin()
	d.stopLoad() // the correlated failure arrived without a new document commit
	if d.loading || !d.loaded || d.source != a {
		t.Fatal("the previous document did not remain observable after a failed load")
	}
	d.observe(next, routeFor(next))
	if d.video != next {
		t.Fatal("lost same-document navigation after the failed load")
	}
	d.begin()
	d.commit(next)
	d.stopLoad()
	if d.loading || d.loaded {
		t.Fatal("a committed error page was treated as playable")
	}
}

func TestFilteredSearchInPlayerUsesTheMainWindow(t *testing.T) {
	for _, target := range []string{home + "tag/test?l_range=1", home + "search/test?page=3&l_range=1"} {
		dest := routeFor(target)
		// Standard links, popup requests and observed page changes use this route
		// to dispatch browse destinations out of the playback window.
		if !dest.browsesInMain() || !dest.isSearchPage() || dest.kind != screenOfficial || dest.target != target {
			t.Fatalf("player did not route the complete search URL to the official main view: %+v", dest)
		}
	}
	for _, target := range []string{home + "watch/sm9?from=60", login} {
		if routeFor(target).browsesInMain() {
			t.Errorf("sent playback or sign-in to the main window: %q", target)
		}
	}
}

func TestPlayerTitleLookupRetriesAfter503(t *testing.T) {
	route := routeFor(home + "watch/sm9")
	d := playerDocument{loaded: true}
	d.observe(route.target, route)
	now := time.Unix(100, 0)
	c := newCatalog(nil)
	requests := 0
	c.http.Transport = requestFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.Path != "/api/getthumbinfo/sm9" || r.Header.Get("Cookie") != "" {
			t.Fatalf("unexpected lookup: %s", r.URL)
		}
		if requests == 1 {
			return testResponse(http.StatusServiceUnavailable, ""), nil
		}
		return testResponse(http.StatusOK, `<nicovideo_thumb_response status="ok"><thumb><video_id>sm9</video_id><title>回復したタイトル</title></thumb></nicovideo_thumb_response>`), nil
	})
	for attempt := range 2 {
		ctx, ok := d.startTitleLookup(now)
		if !ok {
			t.Fatalf("lookup %d did not start", attempt+1)
		}
		if _, allowed := d.startTitleLookup(now); allowed {
			t.Fatal("started a concurrent lookup")
		}
		result, err := c.video(ctx, route)
		if !d.finishTitleLookup(d.revision, route.link, now) || (err != nil) != (attempt == 0) {
			t.Fatalf("unexpected lookup result %v", err)
		}
		if err == nil && !d.acceptTitle(d.revision, route.link, result.title) {
			t.Fatal("did not accept the recovered title")
		}
		if _, allowed := d.startTitleLookup(now.Add(500 * time.Millisecond)); allowed {
			t.Fatal("started a lookup within the retry delay")
		}
		now = now.Add(time.Second)
	}
	if d.title != "回復したタイトル" || requests != 2 {
		t.Fatalf("title %q, requests %d", d.title, requests)
	}
}

func TestPlayerTitleLookupLimitAndStaleCompletion(t *testing.T) {
	route := routeFor(home + "watch/sm9")
	d := playerDocument{loaded: true}
	d.observe(route.target, route)
	now := time.Unix(100, 0)
	for attempt := range maxTitleLookups {
		if _, ok := d.startTitleLookup(now); !ok || !d.finishTitleLookup(d.revision, route.link, now) {
			t.Fatalf("lookup %d failed to settle", attempt+1)
		}
		now = now.Add(time.Duration(attempt+1) * time.Second)
	}
	if _, ok := d.startTitleLookup(now); ok {
		t.Fatal("exceeded the lookup cap for this video visit")
	}
	d.begin()
	d.finish(route.target)
	oldRevision := d.revision
	oldCtx, ok := d.startTitleLookup(now)
	if !ok {
		t.Fatal("a new navigation did not reset the lookup budget")
	}
	d.begin() // reload the same video while the old request is still in flight
	d.commit(route.target)
	d.finish(route.target)
	if oldCtx.Err() == nil {
		t.Fatal("old request was not canceled")
	}
	if _, ok := d.startTitleLookup(now); !ok {
		t.Fatal("new request did not start")
	}
	if d.finishTitleLookup(oldRevision, route.link, now) || d.acceptTitle(oldRevision, route.link, "古いタイトル") || d.cancelTitleLookup == nil {
		t.Fatal("an old result interfered with lookup for the new visit")
	}
	d.stopTitleLookup()
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
	view := ui.NewTester(func(c *ui.Context) { p.draw(c, nil) }, 640, 420)
	if !view.HasText(p.status) || !view.HasText(commentLanguageWarning) {
		t.Fatalf("player text: %v", view.Texts())
	}
}
