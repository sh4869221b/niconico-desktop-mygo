package main

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestLibraryOffersSearchAndWideView(t *testing.T) {
	b := &browser{index: -1, showLibrary: true, closed: true}
	v := ui.NewTester(b.draw, 1000, 700)
	if _, ok := v.Find("ライブラリ内を検索"); !ok {
		t.Fatal("saved collections cannot be searched")
	}
	if err := v.Click("ライブラリを広く表示"); err != nil {
		t.Fatal(err)
	}
	if _, ok := v.Find("通常表示に戻る"); !ok {
		t.Fatal("wide library view not offered")
	}
}

func TestLibraryFiltersWithoutChangingSavedOrder(t *testing.T) {
	items := []entry{{URL: home + "watch/sm1", Title: "猫 CAT", Kind: "動画"}, {URL: home + "search/CAT", Title: "CATを探す", Kind: "キーワード検索"}, {URL: home + "tag/CAT", Title: "タグの猫", Kind: "タグ検索"}}
	got := filterLibrary(items, "cat", "すべての種類")
	if len(got) != 3 {
		t.Fatal("case insensitive title / URL search failed")
	}
	got = filterLibrary(items, "cat", "キーワード検索")
	if len(got) != 1 || got[0] != items[1] {
		t.Fatal("kind filter failed")
	}
	if len(items) != 3 || items[0].Title != "猫 CAT" {
		t.Fatal("filter changed saved collection")
	}
	b := &browser{index: -1, closed: true, showLibrary: true, data: library{Pins: items}, libraryQuery: "cat", libraryKind: "キーワード検索"}
	v := ui.NewTester(b.draw, 1000, 700)
	if !v.HasText("キーワード検索の登録") || v.HasText("猫 CAT") {
		t.Fatal("filtered saved unit not clear")
	}
	if err := v.Click("ライブラリを広く表示"); err != nil {
		t.Fatal(err)
	}
	if !v.HasText(home + "search/CAT") {
		t.Fatal("wide view did not expose saved URL")
	}
	if err := v.Click("通常表示に戻る"); err != nil {
		t.Fatal(err)
	}
	if b.libraryExpanded {
		t.Fatal("cannot leave wide view")
	}
}
