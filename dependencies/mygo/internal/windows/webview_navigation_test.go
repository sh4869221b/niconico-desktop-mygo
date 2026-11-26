//go:build windows && (amd64 || arm64)

package windows

import (
	"runtime"
	"slices"
	"syscall"
	"testing"
	"unsafe"

	"github.com/egoist/mygo/internal/platform"
)

type navigationRecorder struct {
	platform.WindowHandler
	starts, commits, failures []string
	finished, canceled        int
}

func (*navigationRecorder) WillNavigate(platform.Navigation) bool { return true }
func (r *navigationRecorder) NavigationStarted(u string)          { r.starts = append(r.starts, u) }
func (r *navigationRecorder) NavigationCommitted(u string)        { r.commits = append(r.commits, u) }
func (r *navigationRecorder) LoadFailed(u string, _ int, _ string) {
	r.failures = append(r.failures, u)
}
func (r *navigationRecorder) LoadFinished()       { r.finished++ }
func (r *navigationRecorder) NavigationCanceled() { r.canceled++ }

type navigationArgsFixture struct {
	vtable *uintptr
	id     uint64
	url    string
	code   int32
	flag   int32
	cancel bool
}

func (f *navigationArgsFixture) call(fn func(uintptr, uintptr)) {
	var pin runtime.Pinner
	pin.Pin(f)
	defer pin.Unpin()
	fn(0, uintptr(unsafe.Pointer(f)))
}

func TestWebView2NavigationEventsAreCorrelated(t *testing.T) {
	getID := syscall.NewCallback(func(this, out uintptr) uintptr {
		*(*uint64)(native(out)) = (*navigationArgsFixture)(native(this)).id
		return sOK
	})
	getURL := syscall.NewCallback(func(this, out uintptr) uintptr {
		*(*uintptr)(native(out)) = coTaskString(utf16z((*navigationArgsFixture)(native(this)).url))
		return sOK
	})
	getFlag := syscall.NewCallback(func(this, out uintptr) uintptr {
		*(*int32)(native(out)) = (*navigationArgsFixture)(native(this)).flag
		return sOK
	})
	getCode := syscall.NewCallback(func(this, out uintptr) uintptr {
		*(*int32)(native(out)) = (*navigationArgsFixture)(native(this)).code
		return sOK
	})
	putCancel := syscall.NewCallback(func(this, value uintptr) uintptr {
		(*navigationArgsFixture)(native(this)).cancel = value != 0
		return sOK
	})
	noInterface := syscall.NewCallback(func(_, _, _ uintptr) uintptr { return eNoInterface })
	getUserInitiated := syscall.NewCallback(func(_, out uintptr) uintptr {
		*(*int32)(native(out)) = 1
		return sOK
	})
	// Vtable slots follow the WebView2 SDK's base event argument interfaces.
	starting := [...]uintptr{0: noInterface, 3: getURL, 4: getUserInitiated, 5: getFlag, 8: putCancel, 9: getID}
	content := [...]uintptr{4: getID}
	completed := [...]uintptr{3: getFlag, 4: getCode, 5: getID}
	webview := [...]uintptr{wvGetSource: getURL}
	recorder := &navigationRecorder{}
	source := &navigationArgsFixture{vtable: &webview[0]}
	var pin runtime.Pinner
	pin.Pin(source)
	defer pin.Unpin()
	w := &window{opts: &platform.WindowOptions{}, webview: uintptr(unsafe.Pointer(source)), h: recorder, zoom: 1}
	start := func(id uint64, target string, redirect int32) bool {
		f := &navigationArgsFixture{vtable: &starting[0], id: id, url: target, flag: redirect}
		f.call(w.navigationStarting)
		return !f.cancel
	}
	commit := func(id uint64) {
		(&navigationArgsFixture{vtable: &content[0], id: id}).call(w.contentLoading)
	}
	finish := func(id uint64, success, code int32) {
		(&navigationArgsFixture{vtable: &completed[0], id: id, flag: success, code: code}).call(w.navigationCompleted)
	}

	const a, b = "https://example.test/a", "https://example.test/b"
	const base = uint64(1) << 40 // The SDK returns a 64-bit ID.
	if !start(base+1, a, 0) || !start(base+2, b, 0) {
		t.Fatal("a new load was canceled")
	}
	source.url = a
	finish(base+2, 0, 13) // the true B failure before Source has changed from A
	if w.IsLoading() || !slices.Equal(recorder.failures, []string{b}) {
		t.Fatalf("the active failure did not report B: %v", recorder.failures)
	}
	finish(base+1, 1, 0)

	start(base+3, a, 0)
	start(base+4, b, 0)
	source.url = b
	commit(base + 3)
	commit(base + 4)
	finish(base+3, 0, 13) // the same Source B now accompanies the old A failure
	if !w.IsLoading() || len(recorder.failures) != 1 || !slices.Equal(recorder.commits, []string{b}) {
		t.Fatal("a stale event reached the current B load")
	}
	finish(base+4, 1, 0)
	if w.IsLoading() || recorder.finished != 1 {
		t.Fatal("the current success was not reported exactly once")
	}

	start(base+5, b, 0)
	w.programmatic = true
	start(base+6, b, 0) // a second load of exactly the same URL
	finish(base+5, 1, 0)
	finish(base+5, 0, 13)
	if !w.IsLoading() || recorder.finished != 1 || len(recorder.failures) != 1 {
		t.Fatal("same-URL completion was correlated by URL")
	}
	if start(base+5, a, 1) || !start(base+6, a, 1) {
		t.Fatal("redirect was not matched to its original navigation ID")
	}
	finish(base+6, 0, 13)
	if !slices.Equal(recorder.failures, []string{b, a}) || len(recorder.starts) != 6 {
		t.Fatalf("redirect lifecycle: starts %v, failures %v", recorder.starts, recorder.failures)
	}
	start(base+7, b, 0)
	finish(base+7, 0, 14)
	if w.IsLoading() || recorder.canceled != 1 || len(recorder.failures) != 2 {
		t.Fatal("current cancellation did not settle independently of failure")
	}
}
