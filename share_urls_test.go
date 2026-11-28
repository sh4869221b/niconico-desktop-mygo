package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
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
	p := &playerWindow{target: r.target, playedWatch: r, playedVideo: "sm9", playedPosition: &webPlaybackPosition{seconds: 123.9}, document: playerDocument{video: r.link, loaded: true}}
	live := reopenPage{url: r.target}
	v := ui.NewTester(func(c *ui.Context) { p.drawSharing(c, live) }, 800, 300)
	if err := v.Click("この時点のURLをコピー"); err != nil {
		t.Fatal(err)
	}
	if v.Clipboard() != home+"watch/sm9?from=123" {
		t.Fatal("incorrect timed URL", v.Clipboard())
	}
	p.document.video = home + "watch/sm10"
	_, timed := playerShareURLs(p, live)
	if timed != "" {
		t.Fatal("old video's time reused")
	}
	p.playedPosition = nil
	_, timed = playerShareURLs(p, live)
	if timed != "" {
		t.Fatal("unobserved position fabricated")
	}
}

func TestPlayerSharingRejectsOldWebViewStateDuringNavigation(t *testing.T) {
	const old, next = home + "watch/sm9", home + "watch/sm10"
	for _, tc := range []struct {
		name  string
		live  reopenPage
		setup func(*playerWindow)
		want  string
	}{
		{"reloading", reopenPage{url: old}, func(p *playerWindow) { p.document.begin() }, next},
		{"live loading", reopenPage{url: old, loading: true}, func(*playerWindow) {}, next},
		{"renderer gone", reopenPage{url: old}, func(p *playerWindow) { p.document.loaded = false }, next},
		{"retained failed document", reopenPage{url: old}, func(p *playerWindow) { p.failedWebTarget = next }, next},
		{"changed before observing", reopenPage{url: next}, func(*playerWindow) {}, next},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &playerWindow{target: next, playedWatch: routeFor(old), playedVideo: "sm9", playedPosition: &webPlaybackPosition{seconds: 40}, document: playerDocument{video: old, loaded: true}}
			tc.setup(p)
			v := ui.NewTester(func(c *ui.Context) { p.drawSharing(c, tc.live) }, 800, 300)
			if err := v.Click("動画URLをコピー"); err != nil || v.Clipboard() != tc.want {
				t.Fatalf("ordinary copy = %q (%v), want %q", v.Clipboard(), err, tc.want)
			}
			_, timed := playerShareURLs(p, tc.live)
			if timed != "" {
				t.Fatalf("old sample was shareable: %q", timed)
			}
		})
	}
}

func TestOrdinaryShareDoesNotOfferAccountPages(t *testing.T) {
	for _, raw := range []string{login, mylists, mylists + "#/1"} {
		if got, _ := ordinaryShareURL(raw); got != "" {
			t.Fatalf("account page exposed as a share URL: %q", got)
		}
	}
}
