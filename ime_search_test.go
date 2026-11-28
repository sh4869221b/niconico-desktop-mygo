package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// Tester models the platform's IME filter. This protects the app's toolbar
// integration, but does not replace Microsoft IME/IMM32 testing on Windows.
const imeSearchInputLabel = "キーワード、タグ、動画IDまたはURL"

func TestSearchWaitsForIMECommitAndNextEnter(t *testing.T) {
	for _, commitBeforeEnd := range []bool{false, true} {
		name := "composition-end-before-commit"
		if commitBeforeEnd {
			name = "commit-before-composition-end"
		}
		t.Run(name, func(t *testing.T) {
			start := routeFor(home)
			b := &browser{navigation: []screenRoute{start}, closed: true, searchDefault: searchSortPreference{Sort: "v", Ascending: true}}
			view := ui.NewTester(b.toolbar, 1260, 160)
			view.Key(ui.Ctrl, ui.KeyL)
			if !view.Focused(imeSearchInputLabel) {
				t.Fatal("Ctrl+L did not focus the actual search input")
			}
			view.Compose("はつねみく", 5)
			view.Key(0, ui.KeyEnter)
			if len(b.navigation) != 1 || b.navigation[0] != start || b.epoch != 0 {
				t.Fatal("candidate confirmation started navigation")
			}
			if commitBeforeEnd {
				view.Type("初音ミク")
				view.Compose("", 0)
			} else {
				view.Compose("", 0)
				view.Type("初音ミク")
			}
			view.Frame()
			input, ok := view.Access(imeSearchInputLabel)
			if !ok || input.Value != "初音ミク" || len(b.navigation) != 1 || b.epoch != 0 {
				t.Fatalf("commit navigated or lost text: %q, %d routes", input.Value, len(b.navigation))
			}
			view.Key(0, ui.KeyEnter)
			want := b.searchDefault.apply(routeFor(resolve("初音ミク", false)))
			if b.index != 1 || len(b.navigation) != 2 || b.navigation[1] != want || b.epoch != 1 {
				t.Fatalf("second Enter did not navigate once to the keyword search: %+v", b.navigation)
			}
			view.Frame()
			view.KeyUp(ui.KeyEnter)
			if b.epoch != 1 {
				t.Fatal("redraw/key release submitted twice")
			}
			// The explicit tag action still searches the committed text as a tag.
			if err := view.Click("タグ検索"); err != nil {
				t.Fatal(err)
			}
			want = b.searchDefault.apply(routeFor(resolve("初音ミク", true)))
			if b.navigation[b.index] != want || b.epoch != 2 {
				t.Fatal("tag button lost the committed text or sort preference")
			}
		})
	}
}

func TestCancelledIMECompositionDoesNotSearch(t *testing.T) {
	b := &browser{index: -1, closed: true}
	view := ui.NewTester(b.toolbar, 1260, 160)
	view.Key(ui.Ctrl, ui.KeyL)
	view.Compose("はつねみく", 5)
	view.Key(0, ui.KeyEscape)
	view.Compose("", 0)
	view.Frame()
	input, ok := view.Access(imeSearchInputLabel)
	if !ok || input.Value != "" || len(b.navigation) != 0 || b.epoch != 0 {
		t.Fatal("cancelled composition changed text or navigated")
	}
}
