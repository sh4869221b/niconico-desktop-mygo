package main

import (
	"path/filepath"
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

func TestPlayerToggleShortcutsIgnoreRepeat(t *testing.T) {
	s, err := testNativePlaybackStream()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := &playerWindow{native: &nativePlayer{stream: s, recorded: true}}
	b := &browser{nativeVolume: 80, playbackPath: filepath.Join(t.TempDir(), "playback.json")}
	v := ui.NewTester(func(c *ui.Context) { b.drawPlayer(p, c) }, 960, 560)
	for _, control := range []struct {
		key    ui.Key
		active func() bool
	}{
		{ui.KeySpace, func() bool { return !s.Playing() }},
		{ui.KeyM, s.Muted},
	} {
		v.KeyDown(control.key)
		if !control.active() {
			t.Fatalf("key %v did not toggle", control.key)
		}
		v.KeyRepeat(0, control.key)
		if !control.active() {
			t.Fatalf("repeat of key %v toggled again", control.key)
		}
		if control.key == ui.KeyM {
			prefs, err := loadNativeAudio(b.nativeAudioPath())
			if err != nil || !prefs.Muted || prefs.Volume != 80 {
				t.Fatalf("keyboard mute preferences: %+v (%v)", prefs, err)
			}
		}
		v.KeyUp(control.key)
		v.Key(0, control.key)
		if control.active() {
			t.Fatalf("second press of key %v did not toggle", control.key)
		}
	}
	if err := v.Click("一時停止"); err != nil {
		t.Fatal(err)
	}
	v.KeyDown(ui.KeySpace)
	if !s.Playing() {
		t.Fatal("Space did not activate the focused play button")
	}
	v.KeyRepeat(0, ui.KeySpace)
	if !s.Playing() {
		t.Fatal("repeat of Space toggled the focused play button again")
	}
	v.KeyUp(ui.KeySpace)
}
