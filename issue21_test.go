package main

import (
	"github.com/egoist/mygo/ui"
	"net/http"
	"testing"
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
