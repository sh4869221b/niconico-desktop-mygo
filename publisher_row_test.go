package main

import (
	"net/http"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSearchResultPublisherLinksIdentifyTheirOwnRows(t *testing.T) {
	r := routeFor(home + "search/fixture?sort=v")
	c := newCatalog(nil)
	const response = `{"meta":{"status":200},"data":{"items":[
		{"id":"sm10","title":"同じ動画名","owner":{"id":"123","ownerType":"user","name":"投稿者A"}},
		{"id":"sm20","title":"同じ動画名","owner":{"id":"456","ownerType":"user","name":"投稿者B"}},
		{"id":"sm30","title":"投稿者名を取得できない動画","owner":{"id":"789","ownerType":"user"}}
	]}}`
	c.http.Transport = requestFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != apiHost || req.URL.Path != accountSearchPath {
			t.Fatalf("unexpected publisher request: %s", req.URL)
		}
		return testResponse(http.StatusOK, response), nil
	})
	result, err := c.load(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, label, id string }{
		{"投稿者A", "投稿者Aの投稿動画", "123"},
		{"投稿者B", "投稿者Bの投稿動画", "456"},
		{"投稿者", "投稿動画", "789"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			b := &browser{navigation: []screenRoute{r}, result: result, closed: true}
			view := ui.NewTester(b.draw, 960, 650)
			control, ok := view.Access(tc.label)
			if !view.HasText(tc.name) || !ok || !control.Focusable {
				t.Fatalf("publisher has no visible name and accessible control: %+v", control)
			}
			for i := 0; i < 30 && !view.Focused(tc.label); i++ {
				view.Key(0, ui.KeyTab)
			}
			if !view.Focused(tc.label) {
				t.Fatal("could not focus the publisher with the keyboard")
			}
			view.Key(0, ui.KeyEnter)
			if got := b.navigation[b.index]; got.kind != screenUser || got.key != tc.id {
				t.Fatalf("activating %q opened %+v", tc.label, got)
			}
		})
	}
}
