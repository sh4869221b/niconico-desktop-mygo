package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSavedPositionAndIdentity(t *testing.T) {
	for _, raw := range []string{home + "watch/sm9?from=45", home + "search/test?sort=f&order=a&page=3", home + "user/42/video?page=2", home + "mylist/123?page=4"} {
		e, ok := makeEntry(raw, "保存名")
		if !ok {
			t.Fatal(raw)
		}
		r := routeFor(e.openURL())
		want := routeFor(raw)
		if r.page != want.page || r.startTime() != want.startTime() {
			t.Fatalf("%s => %+v", raw, e)
		}
		l := library{Pins: []entry{e}}
		p := filepath.Join(t.TempDir(), "l.json")
		if err := saveLibrary(p, l); err != nil {
			t.Fatal(err)
		}
		got, err := loadLibrary(p)
		if err != nil || got.Pins[0] != e {
			t.Fatalf("%+v %v", got, err)
		}
	}
	e, _ := makeEntry(home+"watch/sm9?from=45", "x")
	plain, _ := makeEntry(home+"watch/sm9", "x")
	if e.URL != plain.URL {
		t.Fatal("position changed identity")
	}
	legacy := entry{URL: plain.URL, Title: "旧登録"}
	if legacy.openURL() != plain.URL {
		t.Fatal(legacy)
	}
	legacy.Target = home + "watch/sm10"
	if legacy.openURL() != plain.URL {
		t.Fatal("unrelated target accepted")
	}
	b := browser{navigation: []screenRoute{routeFor(home + "search/test?sort=f&page=3")}, result: catalogPage{title: "test"}}
	if !strings.Contains(b.currentTitle(), "投稿日時") || !strings.Contains(b.currentTitle(), "3ページ") {
		t.Fatal(b.currentTitle())
	}
}
