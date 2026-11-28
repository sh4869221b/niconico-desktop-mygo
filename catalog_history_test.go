package main

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCatalogRefreshRetainsAnAnchorByVideoID(t *testing.T) {
	r, c := browseFixture(screenList, 100)
	p, err := c.load(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	b := &browser{navigation: []screenRoute{r}, closed: true}
	b.presentNativeResult(p, nil)
	view := ui.NewTester(b.draw, 1000, 600)
	for range 11 {
		view.Scroll(700, 500, 0, 480)
	}
	first, last := b.nativeVisit().list.Visible()
	if first < 2 || last+1 >= len(p.items) {
		t.Fatalf("test did not reach a row inside the list: %d–%d", first, last)
	}
	title := p.items[(first+last)/2-1].title // intro is the first row
	before, ok := view.Access(title)
	if !ok || before.Offscreen {
		t.Fatalf("anchor row %q was unavailable before refresh", title)
	}
	if err := view.Click("更新"); err != nil {
		t.Fatal(err)
	}
	b.result, b.busy = catalogPage{}, true
	view.Frame()
	p.items = slices.Insert(slices.Clone(p.items), 0, card{link: home + "watch/sm101", title: "差し込まれた動画"})
	b.presentNativeResult(p, nil)
	view.Frame()
	after, ok := view.Access(title)
	if !ok || after.Offscreen || after.Bounds.Y != before.Bounds.Y {
		t.Fatalf("refresh moved the visible video from %v to %v", before.Bounds, after.Bounds)
	}
}
