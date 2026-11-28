package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestResultStatusAnnouncesEachTransitionOnce(t *testing.T) {
	b := &browser{navigation: []screenRoute{routeFor(home + "search/test")}, closed: true}
	v := ui.NewTester(b.draw, 960, 560)
	v.UseScreenReader()
	check := func(want string) {
		t.Helper()
		v.Frame()
		if got := v.Announcements(); !slices.Equal(got, []string{want}) {
			t.Fatalf("announcements %v, want %q", got, want)
		}
		v.Frame()
		v.Frame()
		if got := v.Announcements(); len(got) != 0 {
			t.Fatalf("repeated every frame: %v", got)
		}
	}
	b.busy = true
	b.epoch++
	check("読み込み中…")
	if a, ok := v.Access("結果領域"); !ok || !a.Busy {
		t.Fatal("result region is not busy")
	}
	b.busy = false
	b.resultErr = errors.New("通信失敗")
	check("通信失敗")
	b.busy = true
	b.epoch++
	check("読み込み中…")
	b.busy = false
	b.resultErr = nil
	b.result.items = []card{{title: "first"}, {title: "second"}}
	check("このページに2件の結果を読み込みました。")
	if a, ok := v.Access("結果領域"); !ok || a.Busy {
		t.Fatal("result region remained busy")
	}
	b.reportStatus("保存しました。")
	check("保存しました。")
}

func TestRepeatedLibraryOperationsAnnounceEveryResult(t *testing.T) {
	for _, broken := range []bool{false, true} {
		name := "saved"
		if broken {
			name = "failed"
		}
		t.Run(name, func(t *testing.T) {
			first, _ := makeEntry(home+"watch/sm1", "同名の動画")
			second, _ := makeEntry(home+"watch/sm2", first.Title)
			b := &browser{index: -1, closed: true, closeHelpAcknowledged: true, showLibrary: true, libraryPath: filepath.Join(t.TempDir(), "library.json"), data: library{Pins: []entry{first, second}}}
			if err := saveLibrary(b.libraryPath, b.data); err != nil {
				t.Fatal(err)
			}
			label := libraryDeleteLabel(first, false)
			want := label + "しました。"
			if broken {
				if err := os.WriteFile(b.libraryPath, []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
				want = "保存に失敗しました。次に登録や履歴を操作したときに再試行します。"
			}
			v := ui.NewTester(b.draw, 1260, 850)
			v.UseScreenReader()
			for range 2 {
				if err := v.Click(label); err != nil {
					t.Fatal(err)
				}
				if got := v.Announcements(); !slices.Equal(got, []string{want}) {
					t.Fatalf("operation announced %v, want %q once", got, want)
				}
				v.Frame()
				if got := v.Announcements(); len(got) != 0 {
					t.Fatalf("idle redraw announced %v", got)
				}
			}
		})
	}
}

func TestOfficialNavigationAnnouncesLoadingOnceForItsCallbacks(t *testing.T) {
	b := &browser{index: -1, closeHelpAcknowledged: true, official: true}
	v := ui.NewTester(b.draw, 960, 560)
	v.UseScreenReader()
	for range 2 {
		b.beginOfficialLoading()
		// Attach/load, WillNavigate, and DidNavigate all update the same navigation.
		for range 3 {
			b.loading()
			v.Frame()
		}
		if got := v.Announcements(); !slices.Equal(got, []string{"読み込み中…"}) {
			t.Fatalf("one navigation queued %v", got)
		}
		b.finishOfficialPage(home)
	}
}
