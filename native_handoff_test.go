package main

import (
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
