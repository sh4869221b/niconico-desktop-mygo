package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestJumpToResultsSkipsSavedRows(t *testing.T) {
	for _, count := range []int{0, 30, 500} {
		r, catalog := browseFixture(screenUser, 100)
		p, err := catalog.load(t.Context(), r)
		if err != nil {
			t.Fatal(err)
		}
		b := &browser{navigation: []screenRoute{r}, result: p, closed: true, showLibrary: true}
		for i := range count {
			e, _ := makeEntry(fmt.Sprintf("%swatch/sm%d", home, i+1), fmt.Sprintf("登録%d", i))
			b.data.Pins = append(b.data.Pins, e)
		}
		v := ui.NewTester(b.draw, 1000, 700)
		if err := v.Click("結果へ移動"); err != nil {
			t.Fatal(err)
		}
		if !v.Focused(p.title) {
			t.Fatalf("could not skip %d saved rows", count)
		}
	}
}

func TestF6CyclesRegionsAndSkipsHiddenLibrary(t *testing.T) {
	b := &browser{navigation: []screenRoute{routeFor(home)}, closed: true, showLibrary: true}
	v := ui.NewTester(b.draw, 1000, 700)
	v.Key(ui.Ctrl, ui.KeyL)
	v.Key(0, ui.KeyF6)
	if !v.Focused("登録・履歴領域") {
		t.Fatal("F6 did not reach library")
	}
	v.Key(0, ui.KeyF6)
	if !v.Focused("ニコニコ動画") {
		t.Fatal("F6 did not reach content")
	}
	v.Key(ui.Shift, ui.KeyF6)
	if !v.Focused("登録・履歴領域") {
		t.Fatal("reverse F6 failed")
	}
	b.showLibrary = false
	v.Frame()
	v.Key(ui.Ctrl, ui.KeyL)
	v.Key(0, ui.KeyF6)
	if !v.Focused("ニコニコ動画") {
		t.Fatal("hidden library trapped focus")
	}
	v.Key(0, ui.KeyF6)
	if !v.Focused("キーワード、タグ、動画IDまたはURL") {
		t.Fatal("cycle did not return to search")
	}
}
