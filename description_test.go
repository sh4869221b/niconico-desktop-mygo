package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDescriptionRecognizesOnlySupportedReferences(t *testing.T) {
	text, links, parts := plainDescription(`動画はsm9、NM122、(so876)。mylist/52
		<a href="http://www.nicovideo.jp/watch/sm44?from=32">途中から</a>
		https://nico.ms/sm77　https://www.nicovideo.jp/user/122/mylist/34?page=2
		https://example.invalid/path/sm5 example.invalid/mylist/8 //bad.example/so41
		https://www.nicovideo.jp.evil.invalid/watch/sm44
		xsm10 sm11a a-sm11 sm9.png 542387 <a href="https://example.invalid/">sm555</a>
		<script>sm54</script><style>sm55</style>`)
	var want = []string{
		home + "watch/sm9", home + "watch/nm122", home + "watch/so876", home + "mylist/52",
		home + "watch/sm44?from=32", home + "watch/sm77", home + "user/122/mylist/34?page=2",
	}
	var actual []string
	for _, link := range links {
		actual = append(actual, link.link)
	}
	if !slices.Equal(actual, want) {
		t.Fatalf("supported links %v, want %v", actual, want)
	}
	actual = actual[:0]
	for _, part := range parts {
		if part.link.link != "" {
			actual = append(actual, part.link.link)
		}
	}
	if !slices.Equal(actual, want) || !strings.Contains(text, "542387") || strings.Contains(text, "sm54") {
		t.Fatalf("overview display %v or text %q", actual, text)
	}
}

func TestOverviewLinksUseAppNavigation(t *testing.T) {
	r := routeFor(home + "watch/sm9")
	p := catalogPage{title: "動画"}
	p.summary, p.links, p.description = plainDescription("前の動画はsm77、マイリストはmylist/88です。")
	app := &browser{navigation: []screenRoute{r}, result: p, playMode: nativePlayback, closed: true}
	view := ui.NewTester(app.draw, 640, 520)
	if err := view.Click("sm77"); err != nil {
		t.Fatal(err)
	}
	if app.playbackRequest != 1 || app.navigation[app.index] != r {
		t.Fatal("overview video did not use the player entry point")
	}
	if err := view.Click("mylist/88"); err != nil {
		t.Fatal(err)
	}
	if got := app.navigation[app.index]; got.kind != screenList || got.key != "88" {
		t.Fatalf("overview list link opened %+v", got)
	}
}
