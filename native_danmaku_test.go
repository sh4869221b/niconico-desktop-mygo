package main

import (
	"github.com/egoist/mygo/ui"
	"strings"
	"testing"
)

func TestDanmakuCommands(t *testing.T) {
	for _, tc := range []struct {
		command string
		want    commentStyle
	}{
		{"", commentStyle{scrollComment, 26, 0xffffff}},
		{"UE big red", commentStyle{topComment, 38, 0xff0000}},
		{"shita small marineblue", commentStyle{bottomComment, 15, 0x3399ff}},
		{"naka medium #31A2DE", commentStyle{scrollComment, 26, 0x31a2de}},
		{"ue shita small big pink orange", commentStyle{topComment, 15, 0xff8080}},
		{"184 #ab invalid", commentStyle{scrollComment, 26, 0xffffff}},
	} {
		if got := parseCommentStyle(strings.Fields(tc.command)); got != tc.want {
			t.Errorf("%q: got %+v, want %+v", tc.command, got, tc.want)
		}
	}
}

func TestDanmakuPreservesCommentArt(t *testing.T) {
	if got := cleanDanmakuText("　歌詞  A\r\n  B\tC\x00"); got != "　歌詞  A\n  B　C" {
		t.Fatalf("%q", got)
	}
	if got := cleanDanmakuText("\x00　"); got != "" {
		t.Fatalf("control-only: %q", got)
	}
	if got := cleanDanmakuText(strings.Repeat("歌", 500)); len([]rune(got)) != 240 {
		t.Fatalf("unbounded body: %d", len([]rune(got)))
	}
	if got := cleanDanmakuText(strings.Repeat("a\n", 30)); strings.Count(got, "\n") != 11 {
		t.Fatalf("unbounded lines: %d", strings.Count(got, "\n"))
	}
}

func TestDanmakuScrollAvoidsOvertake(t *testing.T) {
	var l commentLanes
	l.reset(360)
	a := commentBox{start: 0, end: scrollLifetime, width: 64, height: 26}
	b := commentBox{start: 800, end: 800 + scrollLifetime, width: 350, height: 26}
	y := l.place(a, scrollComment, 640, 360)
	if b.x(b.start, 640) <= a.x(b.start, 640)+a.width {
		t.Fatal("test setup: no initial gap")
	}
	if b.x(a.end, 640) >= 4 {
		t.Fatal("test setup: does not overtake")
	}
	if got := l.place(b, scrollComment, 640, 360); got < y+a.height {
		t.Fatalf("catch-up in same lane: y=%v", got)
	}
	c := commentBox{start: 1800, end: 1800 + scrollLifetime, width: 50, height: 26}
	if got := l.place(c, scrollComment, 640, 360); got != y {
		t.Fatalf("available first lane not reused: y=%v", got)
	}
	if a.x(a.start, 640) != 640 || a.x(a.end, 640) != -a.width {
		t.Fatal("wrong timeline endpoints")
	}
}

func TestDanmakuFixedStackAndSaturation(t *testing.T) {
	var l commentLanes
	l.reset(360)
	a := commentBox{start: 0, end: fixedLifetime, width: 85, height: 30}
	top := l.place(a, topComment, 640, 360)
	bottom := l.place(a, bottomComment, 640, 360)
	if top != 0 || bottom != 330 {
		t.Fatalf("fixed anchors top=%v bottom=%v", top, bottom)
	}
	if got := l.place(a, topComment, 640, 360); got < a.height {
		t.Fatalf("fixed overlap y=%v", got)
	}
	if got := l.place(a, bottomComment, 640, 360); got+a.height > bottom {
		t.Fatalf("fixed overlap y=%v", got)
	}
	last := a
	last.start, last.end = a.end, a.end+fixedLifetime
	if got := l.place(last, bottomComment, 640, 360); got != bottom {
		t.Fatalf("expired fixed lane not reused: y=%v", got)
	}
	for range 100 {
		if got := l.place(last, bottomComment, 640, 360); got < 0 || got+last.height > 360 {
			t.Fatalf("overflow clipped y=%v", got)
		}
	}
}

func TestDanmakuCrossesPillarbox(t *testing.T) {
	view := ui.Rect{X: 10, Y: 15, W: 700, H: 360}
	picture := videoBounds(view, 320, 240)
	got := danmakuBounds(view, picture)
	if got.X != 40 || got.Y != 15 || got.W != 640 || got.H != 360 {
		t.Fatalf("4:3 canvas: %+v", got)
	}
	hd := videoBounds(view, 1920, 1080)
	if got := danmakuBounds(view, hd); got != hd {
		t.Fatalf("16:9 canvas: %+v, want %+v", got, hd)
	}
	narrow := ui.Rect{X: 0, Y: 0, W: 420, H: 360}
	if got := danmakuBounds(narrow, videoBounds(narrow, 320, 240)); got.X != 0 || got.W != 420 {
		t.Fatalf("overflowed viewport: %+v", got)
	}
}

func TestDanmakuEntranceAndOrder(t *testing.T) {
	cues := makeDanmakuCues([]timedComment{
		{Body: "top", VposMs: 1500, Commands: []string{"ue"}},
		{Body: "scroll", VposMs: 2100},
		{Body: "bottom", VposMs: 2400, Commands: []string{"shita"}},
	})
	if len(cues) != 3 || cues[0].body != "scroll" || cues[0].start != 1100 || cues[0].end != 5100 || cues[1].start != 1500 || cues[2].end != 5400 {
		t.Fatalf("unexpected cue schedule: %+v", cues)
	}
}

func TestDanmakuTextKeepsOutlineAndClip(t *testing.T) {
	var overlay danmakuOverlay
	r := ui.Rect{X: 20, Y: 10, W: 140, H: 105}
	cues := makeDanmakuCues([]timedComment{{Body: "TEST", VposMs: 0, Commands: []string{"ue", "red", "big"}}})
	img := ui.Render(func(ctx *ui.Context) {
		ui.Box(ctx).Fill().Background(ui.RGB(255, 255, 255)).Children(func() {
			overlay.draw(ctx, r, cues, 500)
		})
	}, 180, 125, 1)
	red, dark := 0, 0
	for y := 0; y < 125; y++ {
		for x := 0; x < 180; x++ {
			c := img.RGBAAt(x, y)
			isRed := c.R > 180 && c.G < 90 && c.B < 90
			isDark := c.R < 90 && c.G < 90 && c.B < 90
			if !isRed && !isDark {
				continue
			}
			if x < 20 || x >= 160 || y < 10 || y >= 115 {
				t.Fatalf("comment pixel escaped its clip at %d,%d", x, y)
			}
			if isRed {
				red++
			}
			if isDark {
				dark++
			}
		}
	}
	if red == 0 || dark == 0 {
		t.Fatalf("expected red fill and black outline, got %d and %d pixels", red, dark)
	}
}
