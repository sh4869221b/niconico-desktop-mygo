package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSearchInputFollowsTagsAndHistory(t *testing.T) {
	const tag = " VOCALOID + / #1 "
	const field = "キーワード、タグ、動画IDまたはURL"
	b := &browser{index: -1, closed: true}
	b.navigate(home + "search/初音ミク")
	view := ui.NewTester(b.draw, 960, 650)
	check := func(q string, byTag bool) {
		t.Helper()
		input, ok := view.Access(field)
		button, tagged := view.Access("タグ検索")
		if !ok || input.Value != q || !tagged || button.Checked != byTag {
			t.Fatalf("field %q, tag button checked %t, want %q, %t", input.Value, button.Checked, q, byTag)
		}
	}
	check("初音ミク", false)
	b.navigate(home + "watch/sm9")
	b.result = catalogPage{title: "動画詳細", tags: []string{tag}}
	view.Frame()
	if err := view.Click("タグ検索 " + tag); err != nil {
		t.Fatal(err)
	}
	check(tag, true)
	view.Key(ui.Ctrl, ui.KeyL)
	view.Key(0, ui.KeyEnter)
	r := b.navigation[b.index]
	if p := accountSearchParameters(r); p.Get("tag") != tag || p.Has("keyword") {
		t.Fatalf("Enter changed the exact tag into another search: %v", p)
	}
	for _, button := range []string{"戻る", "戻る"} {
		if err := view.Click(button); err != nil {
			t.Fatal(err)
		}
	}
	check("初音ミク", false)
	for _, button := range []string{"進む", "進む"} {
		if err := view.Click(button); err != nil {
			t.Fatal(err)
		}
	}
	check(tag, true)
	player := ui.NewTester(func(c *ui.Context) {
		b.drawVideoTags(c, []string{"プレーヤーのタグ"}, "")
	}, 640, 120)
	if err := player.Click("タグ検索 プレーヤーのタグ"); err != nil {
		t.Fatal(err)
	}
	view.Frame()
	check("プレーヤーのタグ", true)
}

func TestSearchInputPreservesAnUnsubmittedDraft(t *testing.T) {
	b := &browser{index: -1, closed: true}
	b.navigate(home + "search/keyword")
	const field = "キーワード、タグ、動画IDまたはURL"
	view := ui.NewTester(b.draw, 960, 650)
	view.Key(ui.Ctrl, ui.KeyL)
	view.Key(ui.Ctrl, ui.KeyA)
	view.Type("保存していない下書き")
	b.navigate(home + "tag/another")
	view.Frame()
	if input, _ := view.Access(field); input.Value != "another" {
		t.Fatalf("draft obscured the current tag: %q", input.Value)
	}
	b.navigate(home)
	view.Frame()
	if input, _ := view.Access(field); input.Value != "保存していない下書き" {
		t.Fatalf("draft was lost: %q", input.Value)
	}
}

func TestSearchInputTracksOfficialSearchRoute(t *testing.T) {
	b := &browser{}
	r := routeFor(home + "tag/公式%20タグ?unsupported=1")
	b.showSearchRoute(r)
	if !r.officialBrowse || !b.searchInput.byTag || b.searchInput.field != "公式 タグ" {
		t.Fatalf("unsupported filters did not preserve the displayed search: %+v", b.searchInput)
	}
}
