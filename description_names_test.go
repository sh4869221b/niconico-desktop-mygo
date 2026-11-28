package main

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestDescriptionAuthorLinkName(t *testing.T) {
	_, _, parts := plainDescription(`<a href="https://www.nicovideo.jp/watch/sm77">次回</a> sm88`)
	b := &browser{closed: true}
	view := ui.NewTester(func(c *ui.Context) { b.drawDescription(c, parts) }, 640, 200)
	view.Key(0, ui.KeyTab)
	if !view.Focused("次回") {
		t.Fatal("author's link name was replaced by its destination ID")
	}
	view.Key(0, ui.KeyEnter)
	if b.playbackRequest != 1 {
		t.Fatal("named link did not open player")
	}
	view.Key(0, ui.KeyTab)
	if !view.Focused("sm88") {
		t.Fatal("bare video ID no longer identifies link")
	}
}
