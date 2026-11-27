package main

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestDescriptionTagAndSearchDestinations(t *testing.T) {
	for _, mode := range []string{"tag", "search"} {
		target := searchURL(mode, "初音ミク+A/B")
		_, links, _ := plainDescription(`<a href="` + target + `">作品を探す</a>`)
		if len(links) != 1 || links[0].link != target {
			t.Fatal("official search destination lost", mode, links)
		}
	}
}
func TestDescriptionExternalDestinationIsExplicitCopy(t *testing.T) {
	_, _, parts := plainDescription(`<a href="https://example.invalid/path?q=1">作者のサイト</a>`)
	b := &browser{index: -1, closed: true}
	v := ui.NewTester(func(c *ui.Context) { b.drawDescription(c, parts) }, 900, 300)
	if !v.HasText("https://example.invalid/path?q=1") {
		t.Fatal("external anchor hides destination")
	}
	if err := v.Click("外部URLをコピー: https://example.invalid/path?q=1"); err != nil {
		t.Fatal(err)
	}
	if v.Clipboard() != "https://example.invalid/path?q=1" || len(v.OpenedURLs()) != 0 || b.playbackRequest != 0 || b.index != -1 {
		t.Fatal("external destination did more than copy")
	}
}

func TestExternalDescriptionRejectsUnsafeURLs(t *testing.T) {
	for _, raw := range []string{"javascript:alert(1)", "data:text/html,hello", "file:///tmp/test", "https://user:pass@example.invalid", "https://example.invalid/\nhello", "https://example.invalid/\u202Etxt", "https://example.invalid\\evil"} {
		if got := descriptionExternalURL(raw); got != "" {
			t.Fatal("unsafe copy target accepted", got)
		}
	}
	got := descriptionExternalURL("https://例え.jp/path")
	if got != "https://xn--r8jz45g.jp/path" {
		t.Fatal("international hostname not displayed as ASCII", got)
	}
}
