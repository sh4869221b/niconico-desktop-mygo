package main

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestVideoDetailsHaveOrdinaryShareURL(t *testing.T) {
	b := &browser{navigation: []screenRoute{routeFor(home + "watch/sm9?from=24")}, closed: true}
	v := ui.NewTester(b.draw, 1000, 700)
	if err := v.Click("動画URLをコピー"); err != nil {
		t.Fatal(err)
	}
	if v.Clipboard() != home+"watch/sm9" {
		t.Fatal("ordinary share URL missing", v.Clipboard())
	}
}

func TestSearchSharePreservesDisplayedConditions(t *testing.T) {
	raw := home + "tag/C%2B%2B?sort=v&order=a&page=4"
	b := &browser{navigation: []screenRoute{routeFor(raw)}, closed: true}
	v := ui.NewTester(b.draw, 1000, 700)
	if err := v.Click("現在の検索条件をコピー"); err != nil {
		t.Fatal(err)
	}
	if v.Clipboard() != raw {
		t.Fatal("page or search conditions lost", v.Clipboard())
	}
}
func TestPlayerSharesOnlyVerifiedPosition(t *testing.T) {
	r := routeFor(home + "watch/sm9")
	p := &playerWindow{target: r.target, playedWatch: r, playedVideo: "sm9", playedPosition: &webPlaybackPosition{seconds: 123.9}, document: playerDocument{video: r.link}}
	b := &browser{}
	v := ui.NewTester(func(c *ui.Context) { b.drawPlayerSharing(p, c) }, 800, 300)
	if err := v.Click("この時点のURLをコピー"); err != nil {
		t.Fatal(err)
	}
	if v.Clipboard() != home+"watch/sm9?from=123" {
		t.Fatal("incorrect timed URL", v.Clipboard())
	}
	p.document.video = home + "watch/sm10"
	_, timed := playerShareURLs(p)
	if timed != "" {
		t.Fatal("old video's time reused")
	}
	p.playedPosition = nil
	_, timed = playerShareURLs(p)
	if timed != "" {
		t.Fatal("unobserved position fabricated")
	}
}
