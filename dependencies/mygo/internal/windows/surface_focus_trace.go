//go:build windows && (amd64 || arm64)

package windows

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"github.com/egoist/mygo/internal/platform"
)

// Temporary diagnostics for niconico-desktop-mygo #57. Only IDs, window
// handles and state are recorded. A diagnostic session creates a new file.
var focusTrace = sync.OnceValue(func() *json.Encoder {
	path := os.Getenv("MYGO_UIA57_TRACE")
	if !filepath.IsAbs(path) {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil
	}
	return json.NewEncoder(f)
})

var (
	procGetGUIThreadInfo = user32.NewProc("GetGUIThreadInfo")
	focusTraceMu         sync.Mutex
	focusTraceCount      int
)

type focusTraceRecord struct {
	UTC        time.Time           `json:"utc"`
	Event      string              `json:"event"`
	Surface    uintptr             `json:"surface_hwnd"`
	Parent     uintptr             `json:"parent_hwnd"`
	Other      uintptr             `json:"other_hwnd,omitempty"`
	Foreground uintptr             `json:"foreground_hwnd"`
	Focus      *uintptr            `json:"foreground_focus_hwnd,omitempty"`
	TreeFocus  uint64              `json:"tree_focus,string"`
	Node       uint64              `json:"node,string"`
	Role       platform.AccessRole `json:"role"`
	State      bool                `json:"state"`
}

func (s *surface) traceFocus(event string, other uintptr, treeFocus uint64, node *uiaElement, state bool) {
	enc := focusTrace()
	if enc == nil {
		return
	}
	focusTraceMu.Lock()
	defer focusTraceMu.Unlock()
	if focusTraceCount >= 12000 {
		return
	}
	focusTraceCount++
	record := focusTraceRecord{UTC: time.Now().UTC(), Event: event, Surface: s.hwnd, Other: other, TreeFocus: treeFocus, State: state}
	if s.w != nil {
		record.Parent = s.w.hwnd
	}
	if node != nil {
		record.Node, record.Role = node.n.ID, node.n.Role
	}
	// Thread ID zero asks for the foreground input queue, which can be
	// owned by WebView2 or a process other than this provider.
	var info struct {
		size, flags                                   uint32
		active, focus, capture, menu, moveSize, caret uintptr
		caretRect                                     rect
	}
	info.size = uint32(unsafe.Sizeof(info))
	record.Foreground, _, _ = procGetForegroundWindow.Call()
	if ok, _, _ := procGetGUIThreadInfo.Call(0, uintptr(unsafe.Pointer(&info))); ok != 0 {
		record.Focus = &info.focus
	}
	_ = enc.Encode(record)
}
