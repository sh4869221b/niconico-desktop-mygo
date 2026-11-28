package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPlaybackSettingsExplainsModesBeforeChanging(t *testing.T) {
	for _, mode := range []playbackMode{webPlayback, nativePlayback} {
		b := &browser{playMode: mode}
		view := ui.NewTester(b.drawPlaybackSettings, 640, 350)
		for _, text := range []string{
			"公式：公式の弾幕・コメント投稿・対応配信を利用できます。",
			"独自：日本語弾幕と端末内のローカルNGを利用できます。コメント投稿や一部の配信には対応していません。",
			"どちらも別ウィンドウで開きます。ここで選ぶ既定は次に開く動画から適用します。再生中の「公式 WebView で開く」はその動画だけの切替で、既定は変わりません。",
		} {
			if !view.HasText(text) {
				t.Errorf("mode %s: missing explanation %q", mode, text)
			}
		}
		if b.playMode != mode {
			t.Fatal("reading settings changed the default")
		}
	}
}
