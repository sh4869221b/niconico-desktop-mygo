package main

import (
	"testing"

	"github.com/egoist/mygo"
)

func TestSameVideoFocusPolicy(t *testing.T) {
	watch := home + "watch/sm9"
	for _, mode := range []playbackMode{nativePlayback, webPlayback} {
		p := &playerWindow{mode: mode, target: watch + "?from=20", native: &nativePlayer{loadingStream: true}, view: new(mygo.WebView), document: playerDocument{loaded: true, source: watch + "?from=20"}}
		for _, tc := range []struct {
			raw  string
			want bool
		}{
			{watch, true}, {"https://nico.ms/sm9", true}, {watch + "#comments", true},
			{watch + "?from=0", false}, {watch + "?from=20", false}, {home + "watch/sm12", false},
		} {
			if got := p.canFocusPlayback(routeFor(tc.raw), mode); got != tc.want {
				t.Errorf("%s %s: %t", mode, tc.raw, got)
			}
		}
		if p.canFocusPlayback(routeFor(watch), playbackMode("other")) {
			t.Fatal("ignored mode change")
		}
	}
	p := &playerWindow{mode: nativePlayback, target: watch, native: &nativePlayer{status: "failed"}}
	if p.canFocusPlayback(routeFor(watch), nativePlayback) {
		t.Fatal("failed player cannot retry")
	}
	p = &playerWindow{mode: webPlayback, target: watch, view: new(mygo.WebView), document: playerDocument{loaded: true, source: home + "search/test"}}
	if p.canFocusPlayback(routeFor(watch), webPlayback) {
		t.Fatal("mistook another live page for the video")
	}
}
