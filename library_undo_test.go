package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestLibraryDeletionNamesTarget(t *testing.T) {
	b := &browser{index: -1, closed: true, showLibrary: true, libraryPath: filepath.Join(t.TempDir(), "library.json"), data: library{Pins: []entry{{URL: home + "watch/sm9", Title: "作品名", Kind: "動画"}}}}
	if err := saveLibrary(b.libraryPath, b.data); err != nil {
		t.Fatal(err)
	}
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
		if err := saveLibrary(b.libraryPath, b.data); err != nil {
			t.Fatal(err)
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
		if !slices.Equal(got, entries) {
			t.Fatalf("order not restored: %+v", got)
		}
		if b.player.historySuppressed[entries[1].URL] {
			t.Fatal("undo left history suppressed")
		}
	}
}

func TestUndoLastRowKeepsOrderAfterNewEntry(t *testing.T) {
	a, _ := makeEntry(home+"watch/sm1", "A")
	bEntry, _ := makeEntry(home+"watch/sm2", "B")
	cEntry, _ := makeEntry(home+"watch/sm3", "C")
	d, _ := makeEntry(home+"watch/sm4", "D")
	e, _ := makeEntry(home+"watch/sm5", "E")
	for _, test := range []struct {
		name           string
		history        bool
		removePrevious bool
	}{
		{name: "pins"},
		{name: "history", history: true},
		{name: "pins after predecessor removal", removePrevious: true},
		{name: "history after predecessor removal", history: true, removePrevious: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			b := &browser{index: -1, closed: true, showLibrary: true, history: test.history, libraryPath: filepath.Join(t.TempDir(), "library.json")}
			if test.history {
				b.data.History = []entry{a, bEntry, cEntry}
			} else {
				b.data.Pins = []entry{a, bEntry, cEntry}
			}
			if err := saveLibrary(b.libraryPath, b.data); err != nil {
				t.Fatal(err)
			}
			v := ui.NewTester(b.draw, 1000, 700)
			if err := v.Click(libraryDeleteLabel(cEntry, test.history)); err != nil {
				t.Fatal(err)
			}
			other := &browser{libraryPath: b.libraryPath}
			_, saved := other.changeLibrary(func(data *library) bool {
				items, limit := &data.Pins, 500
				if test.history {
					items, limit = &data.History, 100
				}
				*items = putFirst(*items, d, limit)
				if test.removePrevious {
					*items = slices.DeleteFunc(*items, func(item entry) bool { return item.URL == bEntry.URL })
					*items = putFirst(*items, e, limit)
				}
				return true
			})
			if !saved {
				t.Fatal("could not save the new entry")
			}
			if err := v.Click("元に戻す"); err != nil {
				t.Fatal(err)
			}
			latest, err := loadLibrary(b.libraryPath)
			if err != nil {
				t.Fatal(err)
			}
			got := latest.Pins
			if test.history {
				got = latest.History
			}
			want := []entry{d, a, bEntry, cEntry}
			if test.removePrevious {
				want = []entry{e, d, a, cEntry}
			}
			if !slices.Equal(got, want) {
				t.Fatalf("restored order: %+v, want %+v", got, want)
			}
		})
	}
}

func TestLibraryUndoPersistsAfterStorageRecovers(t *testing.T) {
	a, _ := makeEntry(home+"watch/sm1", "A")
	bEntry, _ := makeEntry(home+"watch/sm2", "B")
	c, _ := makeEntry(home+"watch/sm3", "C")
	path := filepath.Join(t.TempDir(), "library.json")
	p := &playerWindow{}
	b := &browser{libraryPath: path, data: library{History: []entry{a, bEntry}}, history: true, player: p}
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	b.remove(bEntry)
	b.undoLibraryRemoval()
	if len(b.pendingLibraryEdits) != 2 || p.historySuppressed[bEntry.URL] || !slices.Equal(b.data.History, []entry{a, bEntry}) {
		t.Fatalf("undo did not restore the local history: %+v", b.data.History)
	}
	if err := saveLibrary(path, library{History: []entry{a, bEntry}}); err != nil {
		t.Fatal(err)
	}
	b.recordVideo(p, c.URL, c.Title)
	latest, err := loadLibrary(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []entry{c, a, bEntry}; len(b.pendingLibraryEdits) != 0 || !slices.Equal(latest.History, want) {
		t.Fatalf("persisted history: %+v, want %+v", latest.History, want)
	}
}
