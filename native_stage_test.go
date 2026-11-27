package main

import (
	"context"
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestNativeWaitingStages(t *testing.T) {
	seen := map[string]bool{}
	for _, state := range [][3]bool{{false, false, false}, {true, true, false}, {true, false, true}} {
		label := nativeWaitStatus(state[0], state[1], state[2])
		if label == "" || seen[label] {
			t.Fatal("waiting stages are indistinguishable")
		}
		seen[label] = true
	}
	if nativeWaitStatus(true, false, false) != "" {
		t.Fatal("playing video says waiting")
	}
	if formatPlaybackDuration(0) != "—" || formatPlaybackDuration(time.Minute) != "1:00" {
		t.Fatal("unknown duration displayed as zero")
	}
}

func TestNativeFailureOffersRetryAndRetainsState(t *testing.T) {
	s, err := ui.NewVideoPlayer(danmakuTestVideo{image.NewRGBA(image.Rect(0, 0, 40, 30))})
	if err != nil {
		t.Fatal(err)
	}
	s.Seek(25 * time.Second)
	s.Pause()
	n := &nativePlayer{stream: s, cancel: func() {}}
	n.failStream("fixture failure")
	if n.retryAt != 25*time.Second || !n.retryPaused {
		t.Fatal("failure discarded retry position/pause")
	}
	p := &playerWindow{native: n}
	b := &browser{}
	v := ui.NewTester(func(c *ui.Context) { b.drawPlayer(p, c) }, 960, 560)
	if _, ok := v.Find("独自プレーヤーで再試行"); !ok {
		t.Fatal("no same-mode retry")
	}
	n.loadingStream = true
	n.cancel = context.CancelFunc(func() {})
	v.Frame()
	if _, ok := v.Find("独自プレーヤーで再試行"); ok {
		t.Fatal("retry offered while already loading")
	}
}
