package main

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestNativeDisplayPreferencesAndOSMotion(t *testing.T) {
	b := &browser{playbackPath: filepath.Join(t.TempDir(), "playback.json")}
	for _, reduce := range []bool{false, true} {
		n := &nativePlayer{}
		b.applyNativeDisplay(n, reduce)
		if n.showComments == reduce || n.hideCommentPanel || n.showOverview {
			t.Fatal("incorrect OS-following defaults")
		}
	}
	s, err := testNativePlaybackStream()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	n := &nativePlayer{stream: s, recorded: true, displayPending: true}
	v := ui.NewTester(func(c *ui.Context) { b.drawPlayer(&playerWindow{native: n}, c) }, 960, 560)
	if a, ok := v.Access("弾幕を表示"); !ok || !a.Checked {
		t.Fatal("danmaku toggle lacks on state")
	}
	for _, label := range []string{"弾幕を表示", "コメント一覧", "概要欄を表示"} {
		if err := v.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	prefs, err := loadNativeDisplay(b.nativeDisplayPath())
	if err != nil {
		t.Fatal(err)
	}
	b.nativeDisplay = prefs
	next := &nativePlayer{}
	b.applyNativeDisplay(next, false)
	if next.showComments || !next.hideCommentPanel || !next.showOverview {
		t.Fatalf("independent preferences lost: %+v", prefs)
	}
	b.nativeDisplay.Danmaku = "show"
	b.applyNativeDisplay(next, true)
	if !next.showComments {
		t.Fatal("explicit display cannot override OS preference")
	}
}
