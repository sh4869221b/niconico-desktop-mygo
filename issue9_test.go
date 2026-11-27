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
	v.Key(0, ui.KeyTab)
	if b.field != "初音ミク" {
		t.Fatal(b.field)
	}
	if b.navigation[b.index].kind != screenHome {
		t.Fatal("example searched before user chose search")
	}
}
