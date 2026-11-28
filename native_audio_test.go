package main

import (
	"image"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestNativeAudioPersistsIndependentVolumeAndMute(t *testing.T) {
	b := &browser{playbackPath: filepath.Join(t.TempDir(), "playback.json"), nativeVolume: 37, nativeMuted: true}
	b.saveNativeAudio()
	saved, err := loadNativeAudio(b.nativeAudioPath())
	if err != nil || saved.Volume != 37 || !saved.Muted {
		t.Fatalf("lost audio preferences: %+v %v", saved, err)
	}
	s, err := ui.NewVideoPlayer(danmakuTestVideo{image.NewRGBA(image.Rect(0, 0, 40, 30))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	restarted := &browser{nativeVolume: saved.Volume, nativeMuted: saved.Muted}
	restarted.applyNativeAudio(s)
	if s.Volume() != 0.37 || !s.Muted() {
		t.Fatal("next stream lost persisted audio state")
	}
	n := &nativePlayer{stream: s, recorded: true}
	v := ui.NewTester(func(c *ui.Context) { b.drawPlayer(&playerWindow{native: n}, c) }, 960, 560)
	if err := v.Click("ミュート解除"); err != nil {
		t.Fatal(err)
	}
	saved, err = loadNativeAudio(b.nativeAudioPath())
	if err != nil || saved.Muted || saved.Volume != 37 {
		t.Fatalf("unmute overwrote volume: %+v %v", saved, err)
	}
}

func TestNativeAudioDefaults(t *testing.T) {
	p, err := loadNativeAudio(filepath.Join(t.TempDir(), "missing"))
	if err != nil || p.Volume != 80 || p.Muted {
		t.Fatalf("bad defaults: %+v %v", p, err)
	}
}
