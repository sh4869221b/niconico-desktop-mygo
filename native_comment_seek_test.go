package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestCommentProgressGroupAndSeek(t *testing.T) {
	s, err := testNativePlaybackStream()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Pause()
	n := &nativePlayer{stream: s, allComments: []timedComment{{Body: "one", VposMs: 1000}, {Body: "same", VposMs: 1000}, {Body: "next", VposMs: 2000}}, follow: commentFollow{manual: true}}
	n.applyCommentNG(nil)
	for _, tc := range []struct {
		at   time.Duration
		want int64
	}{{0, -1}, {time.Second, 1000}, {1500 * time.Millisecond, 1000}, {2 * time.Second, 2000}, {500 * time.Millisecond, -1}} {
		if got := reachedCommentGroup(tc.at, n.allComments, n.commentRows); got != tc.want {
			t.Fatalf("group at %s = %d, want %d", tc.at, got, tc.want)
		}
	}
	s.Seek(time.Second)
	b := &browser{}
	v := ui.NewTester(func(c *ui.Context) { ui.Column(c).Fill().Children(func() { b.drawNativeComments(c, n, s.Position()) }) }, 400, 350)
	markers := 0
	for _, text := range v.Texts() {
		if text == "現在のコメント" {
			markers++
		}
	}
	if markers != 2 {
		t.Fatalf("same-time group has %d markers: %v position %s", markers, v.Texts(), s.Position())
	}
	if err := v.Click("0:02へ移動"); err != nil {
		t.Fatal(err)
	}
	if s.Position() != 2*time.Second || s.Playing() || !n.follow.manual {
		t.Fatal("comment seek lost pause or manual browsing")
	}
}
