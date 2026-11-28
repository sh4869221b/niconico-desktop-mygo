package main

import (
	"errors"
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestNativeHandoffCapturesPositionAndPause(t *testing.T) {
	s, err := ui.NewVideoPlayer(danmakuTestVideo{image.NewRGBA(image.Rect(0, 0, 40, 30))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Seek(123450 * time.Millisecond)
	p := &playerWindow{target: home + "watch/sm9?from=10", native: &nativePlayer{stream: s, videoID: "sm9"}}
	for _, paused := range []bool{false, true} {
		if paused {
			s.Pause()
		} else {
			s.Play()
		}
		r := p.nativeWebRestore(routeFor(p.target), time.Now())
		if r == nil || r.video != "sm9" || r.position.seconds != 123.45 || r.position.paused != paused || routeFor(r.target).startTime() != 123*time.Second {
			t.Fatalf("lost native state: %+v", r)
		}
	}
	if p.nativeWebRestore(routeFor(home+"watch/sm12"), time.Now()) != nil {
		t.Fatal("restored a different video")
	}
	if p.nativeWebRestore(routeFor(home+"watch/sm9?from=20"), time.Now()) != nil {
		t.Fatal("a new explicit start was overridden by the handoff")
	}
	p.native.videoID = "sm12"
	if p.nativeWebRestore(routeFor(p.target), time.Now()) != nil {
		t.Fatal("accepted mismatching stream identity")
	}
}

func TestNativeHandoffAfterDecoderFailureKeepsLastKnownPosition(t *testing.T) {
	s, err := ui.NewVideoPlayer(danmakuTestVideo{image.NewRGBA(image.Rect(0, 0, 40, 30))})
	if err != nil {
		t.Fatal(err)
	}
	s.Seek(91500 * time.Millisecond)
	s.Pause()
	p := &playerWindow{target: home + "watch/sm9?from=10", native: &nativePlayer{stream: s, videoID: "sm9", cancel: func() {}}}
	p.native.failStream("fixture decoder failure")
	r := p.nativeWebRestore(routeFor(p.target), time.Now())
	if r == nil || r.position.seconds != 91.5 || !r.position.paused {
		t.Fatalf("decoder failure discarded last known position: %+v", r)
	}
}

func TestNativeHandoffBeforeStartupKeepsExplicitStart(t *testing.T) {
	p := &playerWindow{target: home + "watch/sm9?from=42", native: &nativePlayer{pauseRequested: true}}
	r := p.nativeWebRestore(routeFor(p.target), time.Now())
	if r == nil || r.position.seconds != 42 || !r.position.paused {
		t.Fatalf("lost startup intent: %+v", r)
	}
}

func TestNativeHandoffWaitsForVerifiedMediaIDForNumericAlias(t *testing.T) {
	const alias = home + "watch/123456?from=42"
	p := &playerWindow{target: alias, native: &nativePlayer{pauseRequested: true}}
	state := p.nativeWebRestore(routeFor(alias), time.Now())
	if state == nil || state.video != "" || state.routeID != "123456" || !state.position.paused {
		t.Fatalf("unverified alias was treated as a confirmed media ID: %+v", state)
	}
	p.restoring = state
	different := routeFor(home + "watch/sm8")
	p.recordWebSnapshot([]any{different.target, different.key, 8.0, false}, different)
	if p.restoring != state || state.video != "" {
		t.Fatal("a different watch route resolved the alias")
	}
	p.recordWebSnapshot([]any{alias, "sm9", 10.0, false}, routeFor(alias))
	if p.restoring != state || state.video != "sm9" || !state.position.paused || state.position.seconds != 42 {
		t.Fatalf("could not restore the pending intent after verifying the alias: %+v", p.restoring)
	}
}

type handoffFailingDecoder struct{ sentFrame bool }

func (d *handoffFailingDecoder) Rewind() error {
	d.sentFrame = false
	return nil
}

func (d *handoffFailingDecoder) Next() (ui.VideoFrame, error) {
	if d.sentFrame {
		return ui.VideoFrame{}, errors.New("fixture decoder failure")
	}
	d.sentFrame = true
	return ui.VideoFrame{Image: image.NewRGBA(image.Rect(0, 0, 40, 30)), Duration: time.Second}, nil
}

func TestNativeHandoffAfterAnEngineStopsDoesNotInventAPause(t *testing.T) {
	s, err := ui.NewVideoPlayer(new(handoffFailingDecoder))
	if err != nil {
		t.Fatal(err)
	}
	n := &nativePlayer{stream: s, videoID: "sm9", handoffKnown: true, retryAt: 25 * time.Second, cancel: func() {}}
	v := ui.NewTester(func(c *ui.Context) { ui.Video(c, s).Fill() }, 640, 480)
	toggleNativePlayback(s)
	toggleNativePlayback(s) // Resume input before the decoder fails.
	v.Advance(time.Second)
	if s.Err() == nil || s.Playing() {
		t.Fatal("fixture did not fail during playback")
	}
	n.failStream("fixture")
	p := &playerWindow{target: home + "watch/sm9", native: n}
	if r := p.nativeWebRestore(routeFor(p.target), time.Now()); r == nil || r.position.seconds != 25 || r.position.paused {
		t.Fatalf("engine failure overwrote the pre-failure intent: %+v", r)
	}
}
