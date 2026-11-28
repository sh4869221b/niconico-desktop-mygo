package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
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
	pins := make([]entry, 500)
	for i := range pins {
		pins[i], _ = makeEntry(fmt.Sprintf("%swatch/sm%d", home, i+100), "動画")
	}
	if err := saveLibrary(b.libraryPath, library{Pins: pins}); err != nil {
		t.Fatal(err)
	}
	b.pinCurrent()
	if !strings.Contains(b.status, "500件") || b.history || !b.showLibrary {
		t.Fatal(b.status)
	}
}

func TestPinToolbarUsesTheStoredFilteredPageIdentity(t *testing.T) {
	const raw = home + "tag/test?l_range=1"
	unfiltered, _ := makeEntry(home+"tag/test", "絞り込みなし")
	b := &browser{navigation: []screenRoute{routeFor(raw)}, closed: true, closeHelpAcknowledged: true, libraryPath: filepath.Join(t.TempDir(), "library.json")}
	b.data = library{Pins: []entry{unfiltered}}
	if err := saveLibrary(b.libraryPath, b.data); err != nil {
		t.Fatal(err)
	}
	v := ui.NewTester(b.draw, 1260, 850)
	if err := v.Click("☆ 登録"); err != nil {
		t.Fatal("unfiltered pin marked the filtered page as already saved", err)
	}
	if len(b.data.Pins) != 2 || b.data.Pins[0].URL != raw {
		t.Fatalf("filtered page identity was lost: %+v", b.data.Pins)
	}
	if err := v.Click("★ 登録済み"); err != nil || b.status != "登録済みです。" || len(b.data.Pins) != 2 {
		t.Fatalf("filtered page pin was not recognized: %s %v", b.status, err)
	}
}
