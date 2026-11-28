package main

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
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

func TestExpandedLibraryRevealsToolbarDestinations(t *testing.T) {
	for _, tc := range []struct {
		label string
		want  screenKind
	}{
		{"検索", screenSearch}, {"ホーム", screenHome}, {"マイリスト", screenOwnLists}, {"ログイン", screenHome},
	} {
		t.Run(tc.label, func(t *testing.T) {
			b := &browser{index: -1, closed: true, showLibrary: true, closeHelpAcknowledged: true}
			b.navigate(home)
			b.searchInput.field = "検索対象"
			b.libraryExpanded = true
			v := ui.NewTester(b.draw, 1260, 850)
			if err := v.Click(tc.label); err != nil {
				t.Fatal(err)
			}
			if b.libraryExpanded || b.navigation[b.index].kind != tc.want || b.loginHelp != (tc.label == "ログイン") {
				t.Fatalf("destination was hidden: expanded=%v route=%v login=%v", b.libraryExpanded, b.navigation[b.index], b.loginHelp)
			}
		})
	}
}

func TestLibraryFilterRestartsTheScrollPosition(t *testing.T) {
	b := &browser{index: -1, closed: true, showLibrary: true, libraryExpanded: true, closeHelpAcknowledged: true}
	for i := range 100 {
		item, _ := makeEntry(fmt.Sprintf("%swatch/sm%d", home, i+100), fmt.Sprintf("対象 %03d", i))
		b.data.Pins = append(b.data.Pins, item)
	}
	first := libraryEntryLabel(b.data.Pins[0], false)
	later := libraryEntryLabel(b.data.Pins[20], false)
	v := ui.NewTester(b.draw, 1260, 850)
	start, ok := v.Access(first)
	if !ok || start.Offscreen {
		t.Fatal("missing first saved item")
	}
	for range 3 {
		v.Scroll(start.Bounds.X+20, start.Bounds.Y+10, 0, 370)
	}
	passed, _ := v.Access(first)
	shown, _ := v.Access(later)
	if !passed.Offscreen || shown.Offscreen {
		t.Fatalf("test did not scroll the library: first=%+v later=%+v", passed, shown)
	}
	if err := v.Click("ライブラリ内を検索"); err != nil {
		t.Fatal(err)
	}
	v.Type("対象")
	fresh, ok := v.Access(first)
	if !ok || fresh.Offscreen || fresh.Bounds.Y < start.Bounds.Y-4 || fresh.Bounds.Y > start.Bounds.Y+4 {
		t.Fatalf("new filter inherited the old offset: first row %+v, want y=%v", fresh, start.Bounds.Y)
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
