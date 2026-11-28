package main

import (
	"errors"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestOfficialPlayerCreationFailureCanRetryInItsWindow(t *testing.T) {
	const target = home + "watch/sm9?from=60"
	window := new(mygo.Window)
	p := &playerWindow{win: window, mode: webPlayback, target: target}
	b := &browser{player: p, playMode: webPlayback}
	attempts := 0
	failCreate := func(options mygo.WebViewOptions) (*mygo.WebView, error) {
		attempts++
		if !options.Page.Untrusted || options.Page.PreloadScript != webPlayerPreload {
			t.Fatal("a new attempt changed the official player's settings")
		}
		return nil, errors.New("WebView2 temporarily unavailable")
	}
	b.attachPlayerUsing(p, failCreate)
	if p.failedWebTarget != target || p.attachingWeb || p.view != nil {
		t.Fatal("first failure did not retain a retryable video")
	}
	var requested playbackMode
	view := ui.NewTester(func(c *ui.Context) {
		p.draw(c, func(mode playbackMode) {
			requested = mode
			if mode == webPlayback {
				b.attachPlayerUsing(p, failCreate)
			}
		})
	}, 560, 420)
	view.Key(0, ui.KeyTab)
	if !view.Focused("公式プレーヤーを再試行") {
		t.Fatal("the first recovery action could not be reached by the keyboard")
	}
	view.Key(0, ui.KeyEnter)
	if attempts != 2 || requested != webPlayback || b.player != p || p.win != window || p.failedWebTarget != target {
		t.Fatalf("retry replaced the window or did not reattach: %d attempts, %q", attempts, p.failedWebTarget)
	}
	if err := view.Click("この動画を独自プレーヤーで開く"); err != nil {
		t.Fatal(err)
	}
	if requested != nativePlayback || b.playMode != webPlayback {
		t.Fatal("the fallback did not remain a per-video action")
	}
}

func TestOfficialPlayerRecoveryKeepsFailedWatchInsteadOfNonVideoURL(t *testing.T) {
	const first = home + "watch/sm9"
	const failed = home + "watch/sm12?from=10"
	p := &playerWindow{win: new(mygo.Window), target: first}
	p.failWebPlayer(failed, "動画ページを開けませんでした。", false)
	if p.failedWebTarget != failed || p.replaceFailedWebView {
		t.Fatal("load recovery did not retain the exact video URL")
	}
	p.failWebPlayer(login, "公式プレーヤーの表示が終了しました。", true)
	if p.failedWebTarget != first || !p.replaceFailedWebView {
		t.Fatal("renderer recovery used an unrelated page URL")
	}
	old := p.document.revision
	p.discardWebView()
	if p.replaceFailedWebView || p.document.revision <= old {
		t.Fatal("destroying the old view did not invalidate its callbacks")
	}
}
