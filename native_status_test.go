package main

import (
	"errors"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestResultStatusAnnouncesEachTransitionOnce(t *testing.T) {
	b := &browser{navigation: []screenRoute{routeFor(home + "search/test")}, closed: true}
	v := ui.NewTester(b.draw, 960, 560)
	v.UseScreenReader()
	check := func(want string) {
		t.Helper()
		v.Frame()
		if got := v.Announcements(); !slices.Equal(got, []string{want}) {
			t.Fatalf("announcements %v, want %q", got, want)
		}
		v.Frame()
		v.Frame()
		if got := v.Announcements(); len(got) != 0 {
			t.Fatalf("repeated every frame: %v", got)
		}
	}
	b.busy = true
	b.epoch++
	check("読み込み中…")
	if a, ok := v.Access("結果領域"); !ok || !a.Busy {
		t.Fatal("result region is not busy")
	}
	b.busy = false
	b.resultErr = errors.New("通信失敗")
	check("通信失敗")
	b.busy = true
	b.epoch++
	check("読み込み中…")
	b.busy = false
	b.resultErr = nil
	b.result.items = []card{{title: "first"}, {title: "second"}}
	check("このページに2件の結果を読み込みました。")
	if a, ok := v.Access("結果領域"); !ok || a.Busy {
		t.Fatal("result region remained busy")
	}
	b.status = "保存しました。"
	check("保存しました。")
}
