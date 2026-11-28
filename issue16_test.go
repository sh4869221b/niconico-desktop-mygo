package main

import (
	"github.com/egoist/mygo/ui"
	"net/http"
	"testing"
)

func TestRankingFailureShowsRecoveryWithoutChangingOrder(t *testing.T) {
	r := routeFor(home + "search/test")
	c := newCatalog(nil)
	calls := 0
	c.http.Transport = requestFunc(func(*http.Request) (*http.Response, error) { calls++; return testResponse(503, ""), nil })
	p, err := c.search(t.Context(), r)
	if err == nil || calls != 1 {
		t.Fatal(err, calls)
	}
	b := browser{navigation: []screenRoute{r}, result: p, resultErr: err, closed: true}
	v := ui.NewTester(b.draw, 1260, 850)
	if !v.HasText("同じ条件で「再試行」") {
		t.Fatal(v.Texts())
	}
	if b.navigation[0] != r {
		t.Fatal("ranking silently replaced")
	}
	b.resultErr = nil
	b.result.searchMode = "スナップショット検索"
	v.Frame()
	if !v.HasText("検索インデックスの更新には時差") {
		t.Fatal(v.Texts())
	}
}
