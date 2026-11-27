package main

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestVideoTagsUseMatchingMetadata(t *testing.T) {
	want := []string{" 初音ミク + ロック / #1 ", "タグ２"}
	c := newCatalog(nil)
	calls := 0
	c.http.Transport = requestFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "ext.nicovideo.jp" || r.URL.Path != "/api/getthumbinfo/sm9" {
			t.Fatal(r.URL)
		}
		return testResponse(200, `<r status="ok"><thumb><video_id>sm9</video_id><title>動画</title><tags domain="jp"><tag> 初音ミク + ロック / #1 </tag><tag>タグ２</tag></tags><tags domain="tw"><tag>他の言語</tag></tags></thumb></r>`), nil
	})
	r := routeFor(home + "watch/sm9")
	tags, err := playbackTags(t.Context(), c, r, watchData{})
	if err != nil || !slices.Equal(tags, want) || calls != 1 {
		t.Fatalf("metadata fallback: %q %v, requests %d", tags, err, calls)
	}
	var watch watchData
	if err := json.Unmarshal([]byte(`{"tags":{"items":[{"name":" 初音ミク + ロック / #1 "},{"name":"タグ２"},{"name":"タグ２"}]}}`), &watch); err != nil {
		t.Fatal(err)
	}
	tags, err = playbackTags(t.Context(), c, r, watch)
	if err != nil || !slices.Equal(tags, want) || calls != 1 {
		t.Fatalf("watch tags: %q %v, requests %d", tags, err, calls)
	}
}

func TestVideoTagsOpenExactTagSearchWithoutStoppingPlayer(t *testing.T) {
	const tag = " 初音ミク + ロック / #1 "
	for _, playerView := range []bool{false, true} {
		r := routeFor(home + "watch/sm9")
		app := &browser{navigation: []screenRoute{r}, result: catalogPage{title: "動画", tags: []string{tag}}, closed: true}
		stream, err := testNativePlaybackStream()
		if err != nil {
			t.Fatal(err)
		}
		defer stream.Close()
		p := &playerWindow{native: &nativePlayer{stream: stream, recorded: true, tags: []string{tag}, tagNotice: "タグはありません。"}}
		app.player = p
		view := ui.NewTester(func(c *ui.Context) {
			if playerView {
				app.drawPlayer(p, c)
			} else {
				app.draw(c)
			}
		}, 760, 520)
		if err := view.Click("タグ検索 " + tag); err != nil {
			t.Fatal(err)
		}
		target := app.navigation[app.index]
		if target.kind != screenSearch || target.key != tag || accountSearchParameters(target).Get("tag") != tag ||
			app.player != p || !stream.Playing() {
			t.Fatalf("unexpected tag route or player state: %+v", target)
		}
	}
}
