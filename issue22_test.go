package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestFillPinTitle(t *testing.T) {
	raw := home + "watch/sm9"
	for _, old := range []string{raw, "sm9", "自分の名前"} {
		e, _ := makeEntry(raw, old)
		l := library{Pins: []entry{e}}
		fetched, _ := makeEntry(raw, "取得した作品名")
		changed := l.fillPinTitle(fetched)
		if changed != (old == raw) {
			t.Fatalf("old %q changed %v", old, changed)
		}
		path := filepath.Join(t.TempDir(), "library.json")
		if err := saveLibrary(path, l); err != nil {
			t.Fatal(err)
		}
		got, err := loadLibrary(path)
		if err != nil {
			t.Fatal(err)
		}
		want := "取得した作品名"
		if old != raw {
			want = old
		}
		if got.Pins[0].Title != want {
			t.Fatal(got)
		}
	}
}

func TestTruncatedPinPlaceholderIsVisibleAndCanBeReplaced(t *testing.T) {
	r := routeFor(searchURL("tag", strings.Repeat("日本語", 20)))
	b := browser{navigation: []screenRoute{r}, closed: true, closeHelpAcknowledged: true, libraryPath: filepath.Join(t.TempDir(), "library.json")}
	b.pinCurrent()
	if len(b.data.Pins) != 1 || b.data.Pins[0].Title == b.data.Pins[0].URL || !b.data.Pins[0].hasPlaceholderTitle() {
		t.Fatalf("expected a truncated placeholder: %+v", b.data.Pins)
	}
	v := ui.NewTester(b.draw, 1260, 850)
	if !v.HasText("仮の名前:") {
		t.Fatal("the saved list does not identify the placeholder")
	}
	b.result.title = "取得した名前"
	want := b.currentTitle()
	b.pinCurrent()
	loaded, err := loadLibrary(b.libraryPath)
	if err != nil || len(loaded.Pins) != 1 || loaded.Pins[0].Title != want || loaded.Pins[0].hasPlaceholderTitle() {
		t.Fatalf("fetched name did not replace the placeholder: %+v (%v)", loaded.Pins, err)
	}
}

func TestPlayerRepairsPinTitleWithUnchangedOrSuppressedHistory(t *testing.T) {
	const raw, title = home + "watch/sm9", "取得した作品名"
	for _, suppressed := range []bool{false, true} {
		placeholder, _ := makeEntry(raw, "")
		known, _ := makeEntry(raw, title)
		p := &playerWindow{historySuppressed: map[string]bool{raw: suppressed}}
		b := &browser{player: p, libraryPath: filepath.Join(t.TempDir(), "library.json"), data: library{Pins: []entry{placeholder}}}
		if !suppressed {
			b.data.History = []entry{known}
		}
		if err := saveLibrary(b.libraryPath, b.data); err != nil {
			t.Fatal(err)
		}
		b.recordVideo(p, raw, title)
		got, err := loadLibrary(b.libraryPath)
		if err != nil || len(got.Pins) != 1 || got.Pins[0].Title != title {
			t.Fatalf("suppressed=%v saved pin: %+v (%v)", suppressed, got.Pins, err)
		}
		if suppressed && len(got.History) != 0 || !suppressed && (len(got.History) != 1 || got.History[0] != known) {
			t.Fatalf("unrelated history changed: %+v", got.History)
		}
	}
}

func TestPinBeforeAndAfterMetadata(t *testing.T) {
	r := routeFor(home + "watch/sm9")
	b := browser{navigation: []screenRoute{r}, libraryPath: filepath.Join(t.TempDir(), "library.json")}
	b.pinCurrent()
	if b.data.Pins[0].Title != r.link {
		t.Fatal("expected URL placeholder")
	}
	b.result.title = "作品名"
	b.pinCurrent()
	if len(b.data.Pins) != 1 || b.data.Pins[0].Title != "作品名" {
		t.Fatal(b.data.Pins)
	}
}
