package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// Drive the real window content. Disable the asynchronous loader so this test
// owns each result and never needs a native event loop or a network connection.
func TestBrowseLongListNavigation(t *testing.T) {
	r, c := browseFixture(screenList, 100)
	p, err := c.load(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	app := browser{navigation: []screenRoute{r}, result: p, closed: true}
	view := ui.NewTester(func(c *ui.Context) {
		app.result = catalogPage{}
		if app.navigation[app.index] == r {
			app.result = p
		}
		app.draw(c)
	}, 1260, 850)
	click := func(label string) {
		t.Helper()
		if err := view.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	checkRoute := func(want string) {
		t.Helper()
		if got := app.navigation[app.index].target; got != want {
			t.Fatalf("route %q, want %q", got, want)
		}
	}
	// The description link scrolls with the collection.
	click(p.links[0].title)
	checkRoute(p.links[0].link)
	click("戻る")
	checkRoute(r.target)
	// Reach an item that was outside the initial viewport, after resizing.
	view.SetSize(1000, 600)
	for i := 0; i < 12; i++ {
		view.Scroll(700, 350, 0, 480)
	}
	click(p.items[99].title)
	checkRoute(p.items[99].link)
	click("戻る")
	checkRoute(r.target)
	click("進む")
	checkRoute(p.items[99].link)
	click("戻る")
	click("投稿者")
	checkRoute(p.items[0].owner)
	click("戻る")
	click("次へ")
	checkRoute(r.withPage(1).target)
}

func TestBrowseTabsAcrossVirtualRows(t *testing.T) {
	r, c := browseFixture(screenUser, 100)
	p, err := c.load(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	app := browser{navigation: []screenRoute{r}, result: p}
	view := ui.NewTester(app.draw, 1000, 600)
	next := 1
	for i := 0; i < 160 && next <= len(p.items); i++ {
		view.Key(0, ui.KeyTab)
		if view.Focused(p.items[next-1].title) {
			next++
		}
	}
	if next != len(p.items)+1 {
		t.Fatalf("Tab did not reach item %d", next)
	}
	view.Key(ui.Shift, ui.KeyTab)
	if !view.Focused(p.items[98].title) {
		t.Fatalf("Shift+Tab did not return to %q", p.items[98].title)
	}
}
