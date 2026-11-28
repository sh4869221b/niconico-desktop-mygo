package main

import "testing"

type reopenPage struct {
	url     string
	loading bool
}

func (p reopenPage) URL() string     { return p.url }
func (p reopenPage) IsLoading() bool { return p.loading }

func TestSameVideoFocusPolicy(t *testing.T) {
	watch := home + "watch/sm9"
	for _, mode := range []playbackMode{nativePlayback, webPlayback} {
		p := &playerWindow{mode: mode, target: watch + "?from=20", native: &nativePlayer{loadingStream: true}, document: playerDocument{loaded: true}}
		page := reopenPage{url: watch + "?from=20"}
		for _, tc := range []struct {
			raw  string
			want bool
		}{
			{watch, true}, {"https://nico.ms/sm9", true}, {watch + "#comments", true},
			{watch + "?from=0", false}, {watch + "?from=20", false}, {home + "watch/sm12", false},
		} {
			if got := p.canFocusPlayback(routeFor(tc.raw), mode, page); got != tc.want {
				t.Errorf("%s %s: %t", mode, tc.raw, got)
			}
		}
		if p.canFocusPlayback(routeFor(watch), playbackMode("other"), page) {
			t.Fatal("ignored mode change")
		}
	}
	p := &playerWindow{mode: nativePlayback, target: watch, native: &nativePlayer{status: "failed"}}
	if p.canFocusPlayback(routeFor(watch), nativePlayback, nil) {
		t.Fatal("failed player cannot retry")
	}
}

func TestReopenUsesLiveWatchPageBeforePolling(t *testing.T) {
	const a, b = home + "watch/sm9", home + "watch/sm12"
	p := &playerWindow{mode: webPlayback, target: a, document: playerDocument{loaded: true, source: a}}
	live := reopenPage{url: b}
	if p.canFocusPlayback(routeFor(a), webPlayback, live) {
		t.Fatal("discarded request for the old video after a same-document navigation")
	}
	if !p.canFocusPlayback(routeFor(b), webPlayback, live) {
		t.Fatal("reloaded the current video while waiting for the observer")
	}
	p.document.finish(b)
	if !p.canFocusPlayback(routeFor(b), webPlayback, live) {
		t.Fatal("reloaded a completed page before the first observation")
	}
	live.loading = true
	if p.canFocusPlayback(routeFor(b), webPlayback, live) {
		t.Fatal("skipped navigation while the live page was loading")
	}
}

func TestReopenRetriesFailedWebViewLoadWithRetainedDocument(t *testing.T) {
	watch := routeFor(home + "watch/sm9")
	p := &playerWindow{mode: webPlayback, target: watch.target, document: playerDocument{loaded: true}}
	p.document.begin()
	p.document.stopLoad()
	p.failedWebTarget = watch.target
	if !p.document.loaded {
		t.Fatal("fixture did not retain the previously loaded document")
	}
	if p.canFocusPlayback(watch, webPlayback, reopenPage{url: watch.target}) {
		t.Fatal("focus-only path hid a failed navigation")
	}
}
