package main

import (
	"net/http"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUnavailableRowsKeepTheirPosition(t *testing.T) {
	c := newCatalog(nil)
	c.http.Transport = requestFunc(func(*http.Request) (*http.Response, error) {
		return testResponse(200, `{"meta":{"status":200},"data":{"mylist":{"name":"list","items":[{"video":{"id":"sm9","title":"先頭"}},{"video":{}},{"video":{"id":"sm10","title":"末尾"}}]}}}`), nil
	})
	r := routeFor(home + "mylist/1")
	p, err := c.load(t.Context(), r)
	if err != nil || len(p.items) != 3 {
		t.Fatalf("%+v %v", p, err)
	}
	if p.items[0].title != "先頭" || p.items[1].link != "" || p.items[2].title != "末尾" {
		t.Fatal(p.items)
	}
	b := browser{navigation: []screenRoute{r}, result: p, closed: true}
	v := ui.NewTester(b.draw, 1260, 850)
	if !v.HasText("現在表示できない動画") {
		t.Fatal(v.Texts())
	}
	if _, ok := v.AccessByRole(ui.RoleButton, "現在表示できない動画"); ok {
		t.Fatal("unavailable row is actionable")
	}
}

func TestUnavailableSnapshotRowsDoNotReduceThePageCount(t *testing.T) {
	c := newCatalog(nil)
	c.http.Transport = requestFunc(func(*http.Request) (*http.Response, error) {
		return testResponse(200, `{"meta":{"status":200,"totalCount":4},"data":[{"contentId":"sm9","title":"先頭"},{},null,{"contentId":"sm10","title":"末尾"}]}`), nil
	})
	r := routeFor(searchURL("search", "検索語"))
	p, err := c.publicSearch(t.Context(), r)
	if err != nil || p.rawCount != 4 || len(p.items) != 4 {
		t.Fatalf("snapshot lost rows: count=%d rows=%v err=%v", p.rawCount, p.items, err)
	}
	if p.items[0].title != "先頭" || p.items[1].link != "" || p.items[2].link != "" || p.items[3].title != "末尾" {
		t.Fatal("snapshot changed the order", p.items)
	}
	b := browser{navigation: []screenRoute{r}, result: p, closed: true, closeHelpAcknowledged: true}
	v := ui.NewTester(b.draw, 1260, 850)
	if err := v.Click("末尾"); err != nil || b.navigation[b.index].key != "sm10" {
		t.Fatalf("last displayed video was not selectable: %v %v", b.navigation, err)
	}
}
