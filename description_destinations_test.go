package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
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
func TestDescriptionSearchPunctuation(t *testing.T) {
	for _, tc := range []struct{ mode, term, prose string }{
		{"tag", "U.S.A.", "U.S.A"},
		{"search", "速報!", "速報"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			// Keep the punctuation literal in the HTML and prose URL.
			target := home + tc.mode + "/" + tc.term
			_, links, parts := plainDescription(`<a href="` + target + `">作品を探す</a>`)
			if len(links) != 1 || links[0].link != target {
				t.Fatalf("explicit destination changed: %#v, want %q", links, target)
			}
			b := &browser{index: -1, closed: true}
			view := ui.NewTester(func(c *ui.Context) { b.drawDescription(c, parts) }, 900, 300)
			view.Key(0, ui.KeyTab)
			if !view.Focused("作品を探す") {
				t.Fatal("explicit search link could not be focused")
			}
			view.Key(0, ui.KeyEnter)
			if b.index < 0 || b.navigation[b.index].key != tc.term || b.searchInput.byTag != (tc.mode == "tag") {
				t.Fatalf("click selected the wrong search: %+v", b.navigation)
			}
			_, bare, _ := plainDescription("参照: " + target)
			if len(bare) != 1 || routeFor(bare[0].link).key != tc.prose {
				t.Fatalf("bare link kept prose punctuation: %#v", bare)
			}
		})
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

func TestDescriptionKnownLinksKeepTheirLimitWithExternalCopy(t *testing.T) {
	var description strings.Builder
	for i := 1; i <= 25; i++ {
		fmt.Fprintf(&description, `<a href="%swatch/sm%d">動画</a> `, home, i)
	}
	description.WriteString(`<a href="https://example.invalid/">外部</a>`)
	_, links, parts := plainDescription(description.String())
	if len(links) != 24 {
		t.Fatalf("got %d internal links, want 24", len(links))
	}
	var external, omitted bool
	for _, part := range parts {
		if part.link.link == home+"watch/sm25" {
			t.Fatal("an omitted internal link became a copy action")
		}
		external = external || part.link.external
		omitted = omitted || part.omitted
	}
	if !external || !omitted {
		t.Fatal("external copy or internal link limit notice is missing")
	}
}
