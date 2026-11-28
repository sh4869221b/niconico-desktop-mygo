package main

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestKeyboardCanNGSelectedCommentAndFocusNext(t *testing.T) {
	s, err := testNativePlaybackStream()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Pause()
	n := &nativePlayer{stream: s, recorded: true, allComments: []timedComment{
		{VposMs: 1000, Body: "first", author: commentAuthor{ID: "1"}},
		{VposMs: 2000, Body: "target", author: commentAuthor{ID: "2"}},
		{VposMs: 3000, Body: "next unknown"},
	}}
	n.applyCommentNG(nil)
	b := &browser{commentNG: commentNG{path: filepath.Join(t.TempDir(), "ng.json")}}
	p := &playerWindow{native: n}
	v := ui.NewTester(func(c *ui.Context) { b.drawPlayer(p, c) }, 960, 560)
	for i := 0; i < 30 && !v.Focused("コメント一覧の行"); i++ {
		v.Key(0, ui.KeyTab)
	}
	if !v.Focused("コメント一覧の行") {
		t.Fatal("list not keyboard reachable")
	}
	v.Key(0, ui.KeyHome)
	first, found := v.Access("first")
	if !found || first.Offscreen {
		t.Fatal("first comment is outside the viewport")
	}
	x := int(first.Bounds.X + first.Bounds.W - 12)
	y := int(first.Bounds.Y + first.Bounds.H/2)
	selected := v.Image().RGBAAt(x, y)
	v.Key(0, ui.KeyDown)
	if n.selectedComment != 1 {
		t.Fatalf("arrow did not select target: %d", n.selectedComment)
	}
	if unselected := v.Image().RGBAAt(x, y); unselected == selected {
		t.Fatalf("first comment's selection was obscured by the stripe: %v", unselected)
	}
	v.Key(ui.Shift, ui.KeyF10)
	if len(v.Menu()) != 1 {
		t.Fatal("keyboard menu missing")
	}
	if err := v.ChooseMenuItem("このユーザーを NG"); err != nil {
		t.Fatal(err)
	}
	if !b.commentNG.blocked[commentAuthor{ID: "2"}] || len(n.commentRows) != 2 {
		t.Fatal("did not block selected author")
	}
	if n.selectedComment != 1 || !v.Focused("コメント一覧の行") {
		t.Fatalf("did not focus next row: %d", n.selectedComment)
	}
	v.Key(0, ui.KeyContextMenu)
	if err := v.ChooseMenuItem("このユーザーを NG"); err == nil {
		t.Fatal("unknown author can be blocked")
	}
}

func TestNGFocusFindsNextRowWhenEarlierRowsShareAuthor(t *testing.T) {
	a := commentAuthor{ID: "123"}
	n := &nativePlayer{allComments: []timedComment{{author: a}, {author: a}, {author: commentAuthor{ID: "456"}}, {author: commentAuthor{ID: "789"}}}, selectedComment: 1}
	n.applyCommentNG(nil)
	n.commentList.Selected = &n.selectedComment
	n.pendingNG = commentNGEdit{a, true}
	b := &browser{commentNG: commentNG{path: filepath.Join(t.TempDir(), "ng.json")}}
	b.applyPendingCommentNG(n)
	if n.selectedComment != 0 || n.commentRows[n.selectedComment] != 2 {
		t.Fatalf("skipped next surviving row: %v, selected %d", n.commentRows, n.selectedComment)
	}
}
