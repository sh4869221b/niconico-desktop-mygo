package main

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

type longSeekVideo struct{ danmakuTestVideo }

func (longSeekVideo) Duration() time.Duration                     { return 2 * time.Hour }
func (longSeekVideo) Seek(d time.Duration) (time.Duration, error) { return d, nil }

func TestLongVideoSeekKeys(t *testing.T) {
	s, err := ui.NewVideoPlayer(longSeekVideo{danmakuTestVideo{image.NewRGBA(image.Rect(0, 0, 40, 30))}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Pause()
	n := &nativePlayer{stream: s, recorded: true}
	p := &playerWindow{native: n}
	b := &browser{}
	v := ui.NewTester(func(c *ui.Context) { b.drawPlayer(p, c) }, 960, 560)
	if err := v.Click("再生位置"); err != nil {
		t.Fatal(err)
	}
	v.Key(0, ui.KeyHome)
	for _, step := range []struct {
		key  ui.Key
		want time.Duration
	}{
		{ui.KeyRight, 5 * time.Second}, {ui.KeyRight, 10 * time.Second}, {ui.KeyLeft, 5 * time.Second},
		{ui.KeyPageUp, 35 * time.Second}, {ui.KeyPageDown, 5 * time.Second}, {ui.KeyHome, 0},
		{ui.KeyLeft, 0}, {ui.KeyEnd, 2 * time.Hour}, {ui.KeyRight, 2 * time.Hour},
	} {
		v.Key(0, step.key)
		if s.Position() != step.want {
			t.Fatalf("key %v: got %s, want %s", step.key, s.Position(), step.want)
		}
		if s.Playing() {
			t.Fatal("seek resumed paused video")
		}
	}
}

func TestSeekKeysKeepFractionalPointerOffset(t *testing.T) {
	seconds := 123.45
	v := ui.NewTester(func(c *ui.Context) { ui.Slider(c, &seconds, 0, 7200).KeyboardStep(5, 30).Label("seek") }, 500, 80)
	v.Key(0, ui.KeyTab)
	v.Key(0, ui.KeyRight)
	if math.Abs(seconds-128.45) > 1e-9 {
		t.Fatalf("relative seek snapped fractional time: %f", seconds)
	}
	v.Key(0, ui.KeyPageUp)
	if math.Abs(seconds-158.45) > 1e-9 {
		t.Fatalf("large seek snapped fractional time: %f", seconds)
	}
}

func TestSeekPointerRemainsContinuous(t *testing.T) {
	seconds := 0.0
	v := ui.NewTester(func(c *ui.Context) {
		ui.Slider(c, &seconds, 0, 7200).KeyboardStep(5, 30).Label("seek").Width(500)
	}, 500, 80)
	r, ok := v.Find("seek")
	if !ok {
		t.Fatal("missing slider")
	}
	v.ClickAt(r.X+r.W*0.4137, r.Y+r.H/2)
	if seconds <= 0 || seconds >= 7200 || seconds == float64(int(seconds/5))*5 {
		t.Fatalf("pointer position was snapped: %f", seconds)
	}
}
