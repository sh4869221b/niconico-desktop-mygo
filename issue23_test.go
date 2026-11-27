package main

import (
	"github.com/egoist/mygo/ui"
	"path/filepath"
	"strings"
	"testing"
)

func TestPinStateAndHistorySelection(t *testing.T) {
	r := routeFor(home + "watch/sm9")
	b := browser{navigation: []screenRoute{r}, history: true, libraryPath: filepath.Join(t.TempDir(), "library.json"), result: catalogPage{title: "作品"}}
	b.pinCurrent()
	if !b.history || len(b.data.Pins) != 1 || !strings.Contains(b.status, "登録しました") {
		t.Fatalf("history=%v pins=%v status=%s", b.history, b.data.Pins, b.status)
	}
	v := ui.NewTester(b.draw, 1260, 850)
	if err := v.Click("★ 登録済み"); err != nil {
		t.Fatal(err)
	}
	if b.status != "登録済みです。" || !b.history {
		t.Fatal(b.status)
	}
	b.data.Pins = make([]entry, 500)
	b.pinCurrent()
	if !strings.Contains(b.status, "500件") || b.history || !b.showLibrary {
		t.Fatal(b.status)
	}
}
