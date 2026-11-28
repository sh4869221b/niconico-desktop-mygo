package main

import (
	"github.com/egoist/mygo/ui"
	"net/http"
	"testing"
)

func TestEmptyPublicListsKeepAuthor(t *testing.T) {
	c := newCatalog(nil)
	c.http.Transport = requestFunc(func(*http.Request) (*http.Response, error) {
		return testResponse(200, `{"meta":{"status":200},"data":{"mylists":[],"totalCount":0}}`), nil
	})
	r := routeFor(home + "user/123/mylist")
	p, err := c.load(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	b := browser{navigation: []screenRoute{r}, closed: true}
	posts := catalogPage{publisher: videoPublisher{name: "Color"}}
	b.applyCatalogContext(routeFor(home+"user/123/video"), &posts)
	b.applyCatalogContext(r, &p)
	b.result = p
	v := ui.NewTester(b.draw, 1260, 850)
	if !v.HasText("Color の公開マイリスト") || !v.HasText("公開されているマイリストはありません。") {
		t.Fatal(v.Texts())
	}
	if err := v.Click("投稿動画"); err != nil {
		t.Fatal(err)
	}
	other := catalogPage{}
	b.applyCatalogContext(routeFor(home+"user/456/mylist"), &other)
	if other.title != "ユーザー456 の公開マイリスト" {
		t.Fatal(other.title)
	}
}
