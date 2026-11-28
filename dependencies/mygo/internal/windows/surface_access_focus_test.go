//go:build windows && (amd64 || arm64)

package windows

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

func TestUIAFocusPublishesWinEventWithoutUIAEventSubscription(t *testing.T) {
	uiaOnce.Do(initUIA)
	access := &uiaTree{s: &surface{hwnd: 42}}
	access.root = newUIAElement(access)
	access.root.root = true
	t.Cleanup(func() {
		for _, node := range access.nodes {
			node.release()
		}
		access.root.release()
	})
	oldListening, oldNotify := listening, notifyWindowFocus
	t.Cleanup(func() { listening, notifyWindowFocus = oldListening, oldNotify })
	listening = func() bool { return false }
	var focusEvents []uint64
	notifyWindowFocus = func(hwnd uintptr) {
		if hwnd != access.s.hwnd || access.nodes[access.focus] == nil {
			t.Fatal("focus event did not identify the surface's current fragment")
		}
		focusEvents = append(focusEvents, access.focus)
	}
	tree := &platform.AccessTree{Nodes: []platform.AccessNode{
		{ID: 1, Parent: -1, Role: platform.RoleTextField, Label: "Search"},
		{ID: 2, Parent: -1, Role: platform.RoleButton, Label: "Tag"},
		{ID: 3, Parent: -1, Role: platform.RoleHeading, Label: "Results"},
	}, Focus: 1}
	access.quiet = true
	access.update(tree)
	access.quiet = false
	for _, id := range []uint64{2, 2, 3, 0} {
		tree.Focus = id
		access.update(tree)
	}
	if !slices.Equal(focusEvents, []uint64{2, 3}) {
		t.Fatalf("focus events = %v, want tag then results", focusEvents)
	}
}
