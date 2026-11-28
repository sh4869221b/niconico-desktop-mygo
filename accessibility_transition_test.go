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

func TestPlaybackWindowFocusReturnKeepsTabAndF6InNativeRegions(t *testing.T) {
	const search, tag = "キーワード、タグ、動画IDまたはURL", "タグ検索 音楽"
	start := routeFor(home + "watch/sm1097445")
	b := &browser{
		navigation: []screenRoute{start}, closed: true, closeHelpAcknowledged: true,
		result: catalogPage{title: "動画詳細", tags: []string{"音楽"}},
	}
	v := ui.NewTester(b.draw, 1260, 850)
	v.UseScreenReader()
	assertFocus := func(name string) {
		t.Helper()
		access, ok := v.Access(name)
		if !v.Focused(name) || !ok || !access.Focused {
			t.Fatalf("keyboard and accessible focus did not agree on %q: %+v", name, access)
		}
		if name != search {
			if field, _ := v.Access(search); field.Focused {
				t.Fatal("search box retained accessibility focus after focus left it")
			}
		}
	}
	v.Key(ui.Ctrl, ui.KeyL)
	assertFocus(search)
	v.Key(0, ui.KeyF6)
	assertFocus("動画詳細")
	for range 20 {
		if v.Focused("再生") {
			break
		}
		v.Key(0, ui.KeyTab)
	}
	assertFocus("再生")
	// The native app window loses focus while the official player is open.
	v.SetFocused(false)
	v.SetFocused(true)
	assertFocus("再生")
	v.Key(0, ui.KeyTab)
	assertFocus(tag)
	v.Key(0, ui.KeyEnter)
	if route := b.navigation[b.index]; route.kind != screenSearch || route.key != "音楽" {
		t.Fatalf("the focused tag did not open its search: %+v", route)
	}
}
