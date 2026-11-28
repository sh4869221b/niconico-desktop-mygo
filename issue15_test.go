package main

import (
	"github.com/egoist/mygo/ui"
	"strings"
	"testing"
)

func TestSearchRangeAndPageBounds(t *testing.T) {
	total := 120
	p := catalogPage{total: &total, rawCount: 32}
	r := routeFor(home + "search/x?page=2")
	if got := p.searchRange(r); got != "33–64件 / 全120件" {
		t.Fatal(got)
	}
	for _, s := range []string{"0", "-1", "5", "abc", ""} {
		if _, ok := p.searchPageNumber(s); ok {
			t.Fatal(s)
		}
	}
	if n, ok := p.searchPageNumber("4"); !ok || n != 3 {
		t.Fatal(n, ok)
	}
	p.total = nil
	if !strings.Contains(p.searchRange(r), "総件数不明") {
		t.Fatal(p.searchRange(r))
	}
	total = 200000
	p.total = &total
	p.searchMode = "スナップショット検索"
	if p.lastSearchPage() != 3124 || !strings.Contains(p.searchRange(r), "取得上限100000件") {
		t.Fatal(p.lastSearchPage(), p.searchRange(r))
	}
	if _, ok := p.searchPageNumber("3126"); ok {
		t.Fatal("beyond snapshot ceiling")
	}
}

func TestSearchJumpKeepsConditions(t *testing.T) {
	total := 120
	r := routeFor(home + "search/x?sort=v&order=a&page=2")
	b := browser{navigation: []screenRoute{r}, result: catalogPage{total: &total, rawCount: 32}, closed: true}
	v := ui.NewTester(b.draw, 1260, 850)
	if err := v.Click("移動先ページ"); err != nil {
		t.Fatal(err)
	}
	v.Key(ui.Ctrl, ui.KeyA)
	v.Type("4")
	if err := v.Click("指定ページへ"); err != nil {
		t.Fatal(err)
	}
	got := b.navigation[b.index]
	sort, asc := got.searchSort()
	if got.page != 3 || sort.urlKey != "v" || !asc {
		t.Fatal(got)
	}
}
