package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPlayerKeysAndDiscoverableControls(t *testing.T) {
	s, err := testNativePlaybackStream()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	n := &nativePlayer{stream: s, recorded: true}
	p := &playerWindow{native: n}
	b := &browser{}
	v := ui.NewTester(func(c *ui.Context) { b.drawPlayer(p, c) }, 960, 560)
	v.Key(0, ui.KeySpace)
	if s.Playing() {
		t.Fatal("Space did not pause")
	}
	v.Key(0, ui.KeyM)
	if !s.Muted() {
		t.Fatal("M did not mute")
	}
	if err := v.Click("操作キー"); err != nil {
		t.Fatal(err)
	}
	if !v.HasText("F11") || !v.HasText("Esc") {
		t.Fatal("key list unavailable")
	}
	if _, ok := v.Find("全画面"); !ok {
		t.Fatal("fullscreen control missing")
	}
	if err := v.Click("再生"); err != nil {
		t.Fatal(err)
	}
	v.Key(0, ui.KeySpace)
	if s.Playing() {
		t.Fatal("focused pause button and global Space ran twice")
	}
}

func TestCommentListKeysKeepPriority(t *testing.T) {
	s, err := testNativePlaybackStream()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	n := &nativePlayer{stream: s, recorded: true, allComments: []timedComment{{Body: "comment"}}}
	n.applyCommentNG(nil)
	p := &playerWindow{native: n}
	b := &browser{}
	focused := false
	v := ui.NewTester(func(c *ui.Context) { b.drawPlayer(p, c); focused = n.commentList.Focused(c) }, 960, 560)
	for i := 0; i < 40 && !focused; i++ {
		v.Key(0, ui.KeyTab)
	}
	if !focused {
		t.Fatal("list not reachable")
	}
	v.Key(0, ui.KeySpace)
	if !s.Playing() {
		t.Fatal("global Space stole comment list input")
	}
}
