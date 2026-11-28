package main

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestHomeSearchExample(t *testing.T) {
	b := browser{navigation: []screenRoute{routeFor(home)}, showLibrary: true, closed: true}
	v := ui.NewTester(b.draw, 1260, 850)
	if err := v.Click("例：初音ミクを検索欄に入れる"); err != nil {
		t.Fatal(err)
	}
	v.Frame()
	const label = "キーワード、タグ、動画IDまたはURL"
	input, ok := v.Access(label)
	if !ok || input.Value != "初音ミク" || !v.Focused(label) {
		t.Fatalf("search example did not fill and focus the toolbar: %+v", input)
	}
	if b.navigation[b.index].kind != screenHome {
		t.Fatal("example searched before user chose search")
	}
}
