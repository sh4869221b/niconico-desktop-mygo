package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// This covers the app's native accessibility tree and keyboard route only.
// Windows UIA focus events and Narrator/NVDA speech need the real-machine run.
func TestTagTransitionKeepsNativeResultKeyboardAccessible(t *testing.T) {
	const tag = "初音ミク + ロック"
	start := routeFor(home + "watch/sm9")
	b := &browser{navigation: []screenRoute{start}, closed: true, result: catalogPage{title: "元の動画", tags: []string{tag}}}
	view := ui.NewTester(b.draw, 1000, 700)
	label := "タグ検索 " + tag
	focus := func(name string) {
		t.Helper()
		for range 80 {
			if view.Focused(name) {
				return
			}
			view.Key(0, ui.KeyTab)
		}
		t.Fatalf("keyboard did not reach %q", name)
	}
	focus(label)
	info, ok := view.Access(label)
	if !ok || info.Role != ui.RoleButton || !info.Focused || !info.Focusable || info.Description != tag {
		t.Fatalf("tag lacks accessible activation: %+v", info)
	}
	view.Key(0, ui.KeyEnter)
	if r := b.navigation[b.index]; r.kind != screenSearch || r.key != tag || accountSearchParameters(r).Get("tag") != tag {
		t.Fatalf("tag opened wrong route: %+v", r)
	}
	// The async loader is disabled. Supply the completed result it would publish.
	b.result = catalogPage{title: "タグ結果", items: []card{{title: "検証結果の動画", link: home + "watch/sm9"}}}
	view.Frame()
	focus("検証結果の動画")
	info, ok = view.Access("検証結果の動画")
	if !ok || !info.Focused || !info.Focusable || info.Bounds.W <= 0 || info.Bounds.H <= 0 {
		t.Fatalf("result lost accessible focus: %+v", info)
	}
	view.Key(0, ui.KeyEnter)
	if b.navigation[b.index].kind != screenVideo || b.navigation[b.index].key != "sm9" {
		t.Fatal("focused result did not activate the video")
	}
}

func TestNativeControlsReturnAfterOfficialModeIsDismissed(t *testing.T) {
	start := routeFor(home + "watch/sm9")
	b := &browser{navigation: []screenRoute{start}, closed: true, official: true, result: catalogPage{title: "戻る動画"}}
	view := ui.NewTester(b.draw, 1000, 700)
	if _, ok := view.Access("再生"); ok {
		t.Fatal("native result was exposed behind official mode")
	}
	if err := view.Click("独自UIに戻る"); err != nil {
		t.Fatal(err)
	}
	if b.official {
		t.Fatal("official mode remained active")
	}
	for range 80 {
		if view.Focused("再生") {
			info, ok := view.Access("再生")
			if !ok || !info.Focused || info.Role != ui.RoleButton {
				t.Fatal("focused Play control missing from native tree")
			}
			return
		}
		view.Key(0, ui.KeyTab)
	}
	t.Fatal("native play control was not keyboard reachable after return")
}
