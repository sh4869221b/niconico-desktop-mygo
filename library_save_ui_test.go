package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func newLibrarySaveTestBrowser(t *testing.T, data library) (*browser, func()) {
	t.Helper()
	root := t.TempDir()
	b := &browser{index: 0, showLibrary: true, closeHelpAcknowledged: true,
		libraryPath: filepath.Join(root, "library.json"), historySettingsPath: filepath.Join(root, "history.json"),
		data: data, navigation: []screenRoute{routeFor(home + "watch/sm9")}}
	if err := saveLibrary(b.libraryPath, data); err != nil {
		t.Fatal(err)
	}
	callbacks := make(chan func())
	stopped := make(chan struct{})
	b.startLibrarySaves(func(callback func()) {
		done := make(chan struct{})
		select {
		case <-stopped:
			return
		case callbacks <- func() {
			defer close(done)
			callback()
		}:
		}
		select {
		case <-done:
		case <-stopped:
		}
	})
	t.Cleanup(func() {
		close(b.librarySaves.queue.stop)
		close(stopped)
	})
	return b, func() {
		t.Helper()
		select {
		case callback := <-callbacks:
			callback()
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for a library transaction result")
		}
	}
}

func holdLibraryTestLock(t *testing.T, path string) func() {
	t.Helper()
	acquired, release, result := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		result <- withSettingsLock(path, func() error {
			close(acquired)
			<-release
			return nil
		})
	}()
	select {
	case <-acquired:
	case err := <-result:
		t.Fatalf("could not hold the library lock: %v", err)
	}
	released := false
	unlock := func() {
		t.Helper()
		if !released {
			released = true
			close(release)
			if err := <-result; err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Cleanup(unlock)
	return unlock
}

func clickLibraryControl(t *testing.T, view *ui.Tester, label string) {
	t.Helper()
	if err := view.Click(label); err != nil {
		t.Fatal(err)
	}
}

func TestPinWaitsForDurableSaveWhileToolbarRemainsUsable(t *testing.T) {
	b, receive := newLibrarySaveTestBrowser(t, library{})
	unlock := holdLibraryTestLock(t, b.libraryPath)
	view := ui.NewTester(b.draw, 1260, 850)
	clickLibraryControl(t, view, "☆ 登録")
	if b.librarySaves.pending != 1 || !view.HasText("保存を待っています") ||
		view.HasText("を登録しました") || len(b.data.Pins) != 0 {
		t.Fatalf("an uncommitted registration was exposed as saved: %v", view.Texts())
	}
	clickLibraryControl(t, view, "登録・履歴")
	if b.showLibrary {
		t.Fatal("the sidebar toggle could not run while the library lock was held")
	}
	if data, err := loadLibrary(b.libraryPath); err != nil || len(data.Pins) != 0 {
		t.Fatalf("the blocked transaction changed the file: %+v (%v)", data, err)
	}
	unlock()
	receive()
	view.Frame()
	if b.librarySaves.pending != 0 || !view.HasText("動画を登録しました") || !view.HasText("★ 登録済み") || b.showLibrary {
		t.Fatalf("the successful write was not presented: %v", view.Texts())
	}
	if data, err := loadLibrary(b.libraryPath); err != nil || len(data.Pins) != 1 || data.Pins[0].URL != b.current() ||
		!slices.Equal(b.data.Pins, data.Pins) {
		t.Fatalf("the saved result differed from the file: %+v (%v)", data, err)
	}
}

func TestPinQueueRetriesFailureAndRereadsBeforeApplyingRapidEdits(t *testing.T) {
	b, receive := newLibrarySaveTestBrowser(t, library{})
	const corrupt = "{"
	if err := os.WriteFile(b.libraryPath, []byte(corrupt), 0600); err != nil {
		t.Fatal(err)
	}
	view := ui.NewTester(b.draw, 1260, 850)
	clickLibraryControl(t, view, "☆ 登録")
	first := b.current()
	b.navigation[0] = routeFor(home + "watch/sm10")
	view.Frame()
	clickLibraryControl(t, view, "☆ 登録")
	b.updatePinTitle(first, "最初の登録のタイトル")
	receive()
	view.Frame()
	if !b.librarySaves.failed || b.librarySaves.pending != 3 || b.requestLibraryClose() ||
		len(b.data.Pins) != 0 || view.HasText("を登録しました") {
		t.Fatalf("failed edits were discarded or acknowledged: %v", view.Texts())
	}
	if data, err := os.ReadFile(b.libraryPath); err != nil || string(data) != corrupt {
		t.Fatalf("retryable failure overwrote the malformed original: %q (%v)", data, err)
	}
	external, _ := makeEntry(home+"watch/sm11", "別のプロセス")
	if err := saveLibrary(b.libraryPath, library{Pins: []entry{external}}); err != nil {
		t.Fatal(err)
	}
	clickLibraryControl(t, view, "保存を再試行")
	for range 3 {
		receive()
	}
	firstEntry, _ := makeEntry(first, "最初の登録のタイトル")
	secondEntry, _ := makeEntry(b.current(), b.currentTitle())
	want := []entry{secondEntry, firstEntry, external}
	got, err := loadLibrary(b.libraryPath)
	if err != nil || !slices.Equal(got.Pins, want) || !slices.Equal(b.data.Pins, want) ||
		b.librarySaves.pending != 0 || b.librarySaves.failed {
		t.Fatalf("retry did not preserve accepted order and external data: %+v (%v)", got.Pins, err)
	}
}

func TestAsyncPinUndoRereadsAndPreservesTheOriginalPosition(t *testing.T) {
	a, _ := makeEntry(home+"watch/sm1", "A")
	middle, _ := makeEntry(home+"watch/sm2", "B")
	c, _ := makeEntry(home+"watch/sm3", "C")
	b, receive := newLibrarySaveTestBrowser(t, library{Pins: []entry{a, middle, c}})
	view := ui.NewTester(b.draw, 1260, 850)
	clickLibraryControl(t, view, libraryDeleteLabel(middle, false))
	if b.undoRemoval != nil || b.librarySaves.pending != 1 {
		t.Fatal("undo was available before the removal was durable")
	}
	receive()
	external, _ := makeEntry(home+"watch/sm4", "別のプロセス")
	if _, _, err := persistLibraryEdits(b.libraryPath, nil, func(data *library) bool {
		data.Pins = putFirst(data.Pins, external, 500)
		return true
	}); err != nil {
		t.Fatal(err)
	}
	view.Frame()
	clickLibraryControl(t, view, "元に戻す")
	b.undoLibraryRemoval()
	if !b.undoPending || b.librarySaves.pending != 1 {
		t.Fatal("a second undo was accepted before the first finished")
	}
	receive()
	got, err := loadLibrary(b.libraryPath)
	if err != nil || !slices.Equal(got.Pins, []entry{external, a, middle, c}) ||
		!slices.Equal(b.data.Pins, got.Pins) || b.undoRemoval != nil || b.undoPending {
		t.Fatalf("undo did not retain placement and an intervening write: %+v (%v)", got.Pins, err)
	}
}

func TestLibraryCloseWaitsForPinAndFinalPlaybackProgress(t *testing.T) {
	b, receive := newLibrarySaveTestBrowser(t, library{})
	p := &playerWindow{}
	b.player = p
	r := routeFor(b.current())
	b.recordVideo(p, r.link, "fixture")
	receive()
	p.recordWebSnapshot([]any{r.target, r.key, 30.0, true, 123.0, false}, r)
	unlock := holdLibraryTestLock(t, b.libraryPath)
	b.pinCurrent()
	if b.requestLibraryClose() || b.libraryCloseReady || b.librarySaves.pending != 2 {
		t.Fatal("closing did not wait for both the pin and final progress")
	}
	b.pinCurrent()
	if b.librarySaves.pending != 2 {
		t.Fatal("the closing app accepted another edit")
	}
	unlock()
	receive()
	if b.libraryCloseReady || b.librarySaves.pending != 1 {
		t.Fatal("the app allowed close before the final transaction")
	}
	receive()
	got, err := loadLibrary(b.libraryPath)
	if err != nil || len(got.Pins) != 1 || len(got.History) != 1 ||
		!got.History[0].PositionKnown || got.History[0].PositionSeconds != 30 ||
		!b.libraryCloseReady || !b.requestLibraryClose() {
		t.Fatalf("the close did not settle both transactions: %+v (%v)", got, err)
	}
}

func TestLibraryCloseTimeoutLeavesAcceptedEditAvailable(t *testing.T) {
	b, receive := newLibrarySaveTestBrowser(t, library{})
	b.pinCurrent()
	if b.requestLibraryClose() {
		t.Fatal("closing succeeded while the edit was outstanding")
	}
	b.librarySaves.closeBy = time.Now().Add(-time.Second)
	b.finishLibraryClose(b.librarySaves)
	if b.libraryCloseReady || b.librarySaves.pending != 1 || !b.librarySaves.closeBy.IsZero() {
		t.Fatal("timing out discarded the edit or closed the app")
	}
	receive()
	got, err := loadLibrary(b.libraryPath)
	if err != nil || len(got.Pins) != 1 || b.libraryCloseReady || !b.requestLibraryClose() {
		t.Fatalf("a timed-out close did not allow saving then closing: %+v (%v)", got, err)
	}
}

func TestQueuedHistoryRecordingChoicePrecedesFollowingProgress(t *testing.T) {
	b, receive := newLibrarySaveTestBrowser(t, library{})
	p := &playerWindow{}
	b.player = p
	r := routeFor(b.current())
	b.recordVideo(p, r.link, "fixture")
	b.setHistoryRecording(false)
	p.recordWebSnapshot([]any{r.target, r.key, 30.0, true, 123.0, false}, r)
	b.recordPlaybackProgress(p, time.Now(), true)
	for range 3 {
		receive()
	}
	got, err := loadLibrary(b.libraryPath)
	if err != nil || len(got.History) != 1 || got.History[0].PositionKnown ||
		!b.historyDisabled || b.historySettingPending {
		t.Fatalf("progress ran before the queued privacy setting: %+v (%v)", got.History, err)
	}
}
