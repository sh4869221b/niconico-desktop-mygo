package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func nextLibrarySave(t *testing.T, q *librarySaveQueue) librarySaveResult {
	t.Helper()
	select {
	case result := <-q.results:
		return result
	case <-time.After(5 * time.Second):
		t.Fatal("timed out awaiting the save worker")
		return librarySaveResult{}
	}
}

func TestLibrarySaveQueueRetriesTheHeadBeforeLaterEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	const broken = "{"
	if err := os.WriteFile(path, []byte(broken), 0600); err != nil {
		t.Fatal(err)
	}
	q := newLibrarySaveQueue(path, nil)
	t.Cleanup(func() { close(q.stop) })
	s := &librarySaveSession{queue: q}
	first, _ := makeEntry(home+"watch/sm10", "最初の名前")
	other, _ := makeEntry(home+"watch/sm20", "別端末から登録")
	if !s.submit(librarySaveRequest{label: "append", edit: func(l *library) bool {
		l.Pins = append(l.Pins, first)
		return true
	}}) || !s.submit(librarySaveRequest{label: "rename", edit: func(l *library) bool {
		for i := range l.Pins {
			if l.Pins[i] == first {
				l.Pins[i].Title = "変更後"
				return true
			}
		}
		return false
	}}) {
		t.Fatal("the queue did not accept both edits")
	}
	failed := nextLibrarySave(t, q)
	s.receive(failed)
	if failed.err == nil || failed.request.label != "append" || s.pending != 2 || s.requestClose(time.Now()) {
		t.Fatalf("the failed head was acknowledged or allowed to close: %+v", failed)
	}
	if content, err := os.ReadFile(path); err != nil || string(content) != broken {
		t.Fatalf("failed load replaced the original: %q (%v)", content, err)
	}
	if err := saveLibrary(path, library{Pins: []entry{other}}); err != nil {
		t.Fatal(err)
	}
	s.retry()
	for _, label := range []string{"append", "rename"} {
		result := nextLibrarySave(t, q)
		s.receive(result)
		if result.err != nil || result.request.label != label || !result.changed {
			t.Fatalf("unexpected acknowledgement: %s, changed=%v (%v)", result.request.label, result.changed, result.err)
		}
	}
	first.Title = "変更後"
	latest, err := loadLibrary(path)
	if err != nil || !slices.Equal(latest.Pins, []entry{other, first}) || s.pending != 0 || !s.requestClose(time.Now()) {
		t.Fatalf("unexpected durable state or pending acknowledgements: %+v (%v)", latest, err)
	}
}

func TestLibraryCloseWaitIsBoundedWithoutDiscardingAcceptedEdits(t *testing.T) {
	q := &librarySaveQueue{requests: make(chan librarySaveRequest, librarySaveQueueCapacity)}
	s := &librarySaveSession{queue: q}
	now := time.Unix(100, 0)
	request := librarySaveRequest{edit: func(*library) bool { return false }}
	if !s.submit(request) || s.requestClose(now) || s.submit(request) {
		t.Fatal("close did not wait for the accepted operation or prevent further submissions")
	}
	if s.finishClose(now.Add(libraryCloseWait)) || s.pending != 1 || !s.closeBy.IsZero() {
		t.Fatal("timeout should leave the window available and keep the queued operation")
	}
	if s.requestClose(now.Add(libraryCloseWait)) {
		t.Fatal("a second close acknowledged an operation still in flight")
	}
	s.receive(librarySaveResult{})
	if !s.finishClose(now.Add(libraryCloseWait)) || s.pending != 0 || s.message != "保存しました。" {
		t.Fatal("durable acknowledgement did not finish a waiting close")
	}
}
