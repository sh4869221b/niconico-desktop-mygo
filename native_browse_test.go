package main

import (
	"path/filepath"
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
	click("https://nico.ms/sm1")
	checkRoute(r.target)
	if app.playbackRequest != 1 {
		t.Fatal("the inline video link did not open the selected player")
	}
	// Reach an item that was outside the initial viewport, after resizing.
	view.SetSize(1000, 600)
	for i := 0; i < 22; i++ {
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
	for i := 0; i < 260 && next <= len(p.items); i++ {
		view.Key(0, ui.KeyTab)
		if view.Focused(p.items[next-1].title) {
			next++
		}
	}
	if next != len(p.items)+1 {
		t.Fatalf("Tab did not reach item %d", next)
	}
	view.Key(ui.Shift, ui.KeyTab)
	view.Key(ui.Shift, ui.KeyTab)
	if !view.Focused(p.items[98].title) {
		t.Fatalf("Shift+Tab did not return to %q", p.items[98].title)
	}
}

func TestCatalogThumbnailOpensCorrectVirtualRow(t *testing.T) {
	r, catalog := browseFixture(screenList, 100)
	p, err := catalog.load(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	app := browser{navigation: []screenRoute{r}, result: p, closed: true, playMode: nativePlayback}
	view := ui.NewTester(app.draw, 960, 620)
	if err := view.RightClick(p.items[0].title + " を再生"); err != nil {
		t.Fatal(err)
	}
	if app.playbackRequest != 0 {
		t.Fatal("right-click opened a video")
	}
	for range 26 {
		view.Scroll(550, 340, 0, 460)
	}
	if err := view.Click(p.items[99].title + " を再生"); err != nil {
		t.Fatal(err)
	}
	if app.playbackRequest != 1 {
		t.Fatal("thumbnail did not use the player's opening path")
	}
	if app.navigation[app.index] != r {
		t.Fatal("thumbnail changed the main page")
	}
}

func TestThumbnailGestureIgnoresDragAndDoubleClick(t *testing.T) {
	var press thumbnailPress
	for _, tc := range []struct {
		clicks int
		events []ui.InputEvent
		cancel bool
	}{
		{1, []ui.InputEvent{{Kind: ui.InputPointerUp, X: 2}}, false},
		{2, []ui.InputEvent{{Kind: ui.InputPointerUp}}, true},
		{1, []ui.InputEvent{{Kind: ui.InputPointerMove, Y: 12}, {Kind: ui.InputPointerUp}}, true},
		{1, []ui.InputEvent{{Kind: ui.InputScroll}, {Kind: ui.InputPointerUp}}, true},
	} {
		press.input(ui.InputEvent{Kind: ui.InputPointerDown, Clicks: tc.clicks})
		for _, ev := range tc.events {
			press.input(ev)
		}
		if press.canceled != tc.cancel {
			t.Errorf("gesture %+v cancellation was %v", tc, press.canceled)
		}
	}
}

func TestSearchSortAndPagingWithKeyboard(t *testing.T) {
	r := routeFor(home + "tag/kitty?page=3")
	path := filepath.Join(t.TempDir(), "search-sort.json")
	app := browser{navigation: []screenRoute{r}, result: catalogPage{more: true}, searchSortPath: path, closed: true}
	view := ui.NewTester(app.draw, 640, 520)
	keyboardSelect := func(label string, moves int) {
		t.Helper()
		for i := 0; i < 24 && !view.Focused(label); i++ {
			view.Key(0, ui.KeyTab)
		}
		if !view.Focused(label) {
			t.Fatalf("could not focus %q", label)
		}
		view.Key(0, ui.KeySpace)
		for range moves {
			view.Key(0, ui.KeyDown)
		}
		view.Key(0, ui.KeyEnter)
	}
	check := func(page int, ascending bool) {
		t.Helper()
		got := app.navigation[app.index]
		sort, asc := got.searchSort()
		if got.key != "kitty" || got.page != page || sort.apiKey != "viewCount" || asc != ascending ||
			accountSearchParameters(got).Get("tag") != "kitty" {
			t.Fatalf("unexpected search route: %+v", got)
		}
	}
	keyboardSelect("並べ替え", 3)
	check(0, false)
	if err := view.Click("次へ"); err != nil {
		t.Fatal(err)
	}
	check(1, false)
	keyboardSelect("並び順", 1)
	check(0, true)
	if err := view.Click("次へ"); err != nil {
		t.Fatal(err)
	}
	check(1, true)
	if err := view.Click("最初のページ"); err != nil {
		t.Fatal(err)
	}
	check(0, true)
	if err := view.Click("戻る"); err != nil {
		t.Fatal(err)
	}
	check(1, true)
	restarted := browser{index: -1, searchDefault: loadSearchSort(path), closed: true}
	restarted.navigate(home + "search/explicit?sort=f&order=d")
	if got := accountSearchParameters(restarted.navigation[restarted.index]); got.Get("sortKey") != "registeredAt" || got.Get("sortOrder") != "desc" {
		t.Fatalf("explicit query was changed: %v", got)
	}
	restarted.navigate(home + "tag/fresh?page=3")
	if got := accountSearchParameters(restarted.navigation[restarted.index]); got.Get("sortKey") != "viewCount" || got.Get("sortOrder") != "asc" || got.Get("page") != "3" {
		t.Fatalf("restarted application's sort was not restored: %v", got)
	}
	restarted.moveNative(-1)
	if got := accountSearchParameters(restarted.navigation[restarted.index]); got.Get("sortKey") != "registeredAt" {
		t.Fatalf("history's sort was replaced: %v", got)
	}
	if pref := loadSearchSort(path); pref != (searchSortPreference{Sort: "v", Ascending: true}) {
		t.Fatalf("navigation overwrote the saved sort: %+v", pref)
	}
}

func TestPostedVideosAndMylistsCanReturnToFirstPage(t *testing.T) {
	for _, target := range []string{home + "user/123/video?page=5", home + "mylist/456?page=5"} {
		r := routeFor(target)
		app := browser{navigation: []screenRoute{r}, closed: true}
		view := ui.NewTester(app.draw, 640, 520)
		if err := view.Click("最初のページ"); err != nil {
			t.Fatal(err)
		}
		if got := app.navigation[app.index]; got.page != 0 || got.kind != r.kind || got.key != r.key || got.target != r.link {
			t.Fatalf("jump from %s went to %+v", target, got)
		}
	}
}
