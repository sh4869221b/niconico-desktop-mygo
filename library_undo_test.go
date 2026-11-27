package main

import (
	"github.com/egoist/mygo/ui"
	"path/filepath"
	"testing"
)

func TestLibraryDeletionNamesTarget(t *testing.T) {
	b := &browser{index: -1, closed: true, showLibrary: true, libraryPath: filepath.Join(t.TempDir(), "library.json"), data: library{Pins: []entry{{URL: home + "watch/sm9", Title: "作品名", Kind: "動画"}}}}
	v := ui.NewTester(b.draw, 1000, 700)
	if err := v.Click("作品名を登録から削除"); err != nil {
		t.Fatal(err)
	}
	if len(b.data.Pins) != 0 {
		t.Fatal("item not removed")
	}
}

func TestLibraryUndoRestoresOriginalOrderAndPersistence(t *testing.T) {
	for _, history := range []bool{false, true} {
		entries := []entry{}
		for _, id := range []string{"sm1", "sm2", "sm3"} {
			e, _ := makeEntry(home+"watch/"+id, id)
			entries = append(entries, e)
		}
		b := &browser{index: -1, history: history, libraryPath: filepath.Join(t.TempDir(), "library.json"), player: &playerWindow{}}
		if history {
			b.data.History = entries
		} else {
			b.data.Pins = entries
		}
		b.remove(entries[1])
		// Switching the visible collection must not change the undo destination.
		b.history = !history
		v := ui.NewTester(b.draw, 1000, 700)
		if err := v.Click("元に戻す"); err != nil {
			t.Fatal(err)
		}
		saved, err := loadLibrary(b.libraryPath)
		if err != nil {
			t.Fatal(err)
		}
		got := saved.Pins
		if history {
			got = saved.History
		}
		if len(got) != 3 || got[1].URL != entries[1].URL {
			t.Fatalf("order not restored: %+v", got)
		}
		if b.player.historySuppressed[entries[1].URL] {
			t.Fatal("undo left history suppressed")
		}
	}
}
