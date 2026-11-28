package main

import (
	"context"
	"errors"
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

type failingNativeStageDecoder struct {
	frames int
}

func (d *failingNativeStageDecoder) Rewind() error {
	d.frames = 0
	return nil
}

func (d *failingNativeStageDecoder) Next() (ui.VideoFrame, error) {
	if d.frames != 0 {
		return ui.VideoFrame{}, errors.New("fixture decoder stopped")
	}
	d.frames++
	return ui.VideoFrame{Image: image.NewRGBA(image.Rect(0, 0, 40, 30)), Duration: time.Second}, nil
}

func TestRetryKeepsThePlayingIntentAndStartingPointAfterFailures(t *testing.T) {
	stream, err := ui.NewVideoPlayer(new(failingNativeStageDecoder))
	if err != nil {
		t.Fatal(err)
	}
	v := ui.NewTester(func(c *ui.Context) { ui.Video(c, stream).Fill() }, 640, 480)
	v.Advance(time.Second)
	if stream.Err() == nil || stream.Playing() || stream.Paused() {
		t.Fatalf("fixture did not fail during playback: %v", stream.Err())
	}
	route := routeFor(home + "watch/sm9?from=2")
	previous := &nativePlayer{stream: stream, retryAt: 25 * time.Second, cancel: func() {}}
	previous.failStream("mid-stream failure")
	if previous.retryPaused || previous.retryAt != 25*time.Second {
		t.Fatalf("lost playing intent or position: paused=%v time=%v", previous.retryPaused, previous.retryAt)
	}
	for range 2 {
		next := newNativeAttempt(route, previous.retryAt, previous.retryPaused, func() {})
		next.startAt = 0 // Media Foundation accepted the start but never displayed a picture.
		next.failStream("startup timeout")
		if next.retryAt != 25*time.Second || next.retryPaused {
			t.Fatalf("consecutive failed retry discarded state: paused=%v time=%v", next.retryPaused, next.retryAt)
		}
		previous = next
	}
}
