package main

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestPlayerHistoryAcrossSameDocumentNavigation(t *testing.T) {
	a := home + "watch/sm9"
	b := home + "watch/sm12"
	c := home + "watch/sm13"
	p := &playerWindow{}
	browser := &browser{win: new(mygo.Window), player: p, libraryPath: filepath.Join(t.TempDir(), "library.json")}
	observe := func(raw, title string) {
		if p.document.observe(raw, title) && p.document.video != "" {
			browser.recordVideo(p, p.document.video, p.document.title)
		}
	}
	requireTitle := func(raw, want string) {
		t.Helper()
		for _, item := range browser.data.History {
			if item.URL == raw && item.Title == want {
				return
			}
		}
		t.Fatalf("missing %q with title %q in %+v", raw, want, browser.data.History)
	}

	observe(a, "動画A")
	observe(a, "動画B") // the title changes first
	observe(b+"?from=60", "動画B")
	requireTitle(a, "動画A")
	requireTitle(b, "動画B")

	observe(c, "動画B") // the URL changes first
	requireTitle(c, c)
	observe(c, "動画C")
	observe(c+"#comments", "動画C")
	requireTitle(c, "動画C")
	observe(login, "ログイン")
	if len(browser.data.History) != 3 || browser.data.History[0].URL != c {
		t.Fatalf("unexpected history: %+v", browser.data.History)
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

func TestPlayerShowsCommentLanguageWarningAlongsideStatus(t *testing.T) {
	p := &playerWindow{status: "プレーヤーの状態", commentLanguageWarning: true}
	view := ui.NewTester(p.draw, 640, 420)
	if !view.HasText(p.status) || !view.HasText(commentLanguageWarning) {
		t.Fatalf("player text: %v", view.Texts())
	}
}
