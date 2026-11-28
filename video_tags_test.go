package main

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

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
	tags, _, err := playbackDetails(t.Context(), c, r, watchData{})
	if err != nil || !slices.Equal(tags, want) || calls != 1 {
		t.Fatalf("metadata fallback: %q %v, requests %d", tags, err, calls)
	}
	var watch watchData
	if err := json.Unmarshal([]byte(`{"tag":{"items":[{"name":" 初音ミク + ロック / #1 "},{"name":"タグ２"},{"name":"タグ２"}]},"owner":{"id":123,"nickname":"投稿者名"}}`), &watch); err != nil {
		t.Fatal(err)
	}
	tags, publisher, err := playbackDetails(t.Context(), c, r, watch)
	if err != nil || !slices.Equal(tags, want) || calls != 1 || publisher != (videoPublisher{"投稿者名", home + "user/123/video"}) {
		t.Fatalf("watch tags: %q %v, requests %d", tags, err, calls)
	}
}

func TestPlaybackPublisherUsesChannelAndKeepsTagsOnMissingFallback(t *testing.T) {
	var watch watchData
	if err := json.Unmarshal([]byte(`{"tag":{"items":[{"name":"音楽"}]},"channel":{"id":"ch2647028","name":"公式チャンネル"},"owner":{"id":123,"nickname":"別の投稿者"}}`), &watch); err != nil {
		t.Fatal(err)
	}
	route := routeFor(home + "watch/so38016254")
	_, publisher, err := playbackDetails(t.Context(), catalog{}, route, watch)
	if err != nil || publisher != (videoPublisher{"公式チャンネル", "https://ch.nicovideo.jp/ch2647028/video"}) || allowedURL(publisher.videos) == nil {
		t.Fatalf("unexpected channel publisher: %+v, %v", publisher, err)
	}
	watch.Channel.Name = ""
	watch.Owner.Nickname = ""
	c := newCatalog(nil)
	c.http.Transport = requestFunc(func(r *http.Request) (*http.Response, error) {
		return testResponse(503, ""), nil
	})
	tags, publisher, err := playbackDetails(t.Context(), c, route, watch)
	if err != nil || !slices.Equal(tags, []string{"音楽"}) || publisher.name != "" || publisher.videos == "" {
		t.Fatalf("missing metadata lost an available channel link or tags: %+v, %v, %v", publisher, tags, err)
	}
}

func TestSearchResultMetadataUsesTheSearchResponse(t *testing.T) {
	name := strings.Repeat("長い投稿者名", 20)
	tag := strings.Repeat("長いタグ", 28) + " + #1"
	c := newCatalog(nil)
	calls := 0
	c.http.Transport = requestFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != apiHost || r.URL.Path != accountSearchPath {
			t.Fatalf("unexpected extra metadata request: %s", r.URL)
		}
		body := `{"meta":{"status":200},"data":{"items":[
			{"id":"sm9","title":"タグ付き動画","owner":{"id":123,"name":"` + name + `","ownerType":"user"},"tags":[{"name":"` + tag + `"},"別タグ",{},"別タグ"]},
			{"id":"sm10","title":"名前なし","owner":{"id":"456","ownerType":"user"}},
			{"id":"so11","title":"公式動画","owner":{"id":"ch50","name":"公式","ownerType":"channel"}}]}}`
		return testResponse(200, body), nil
	})
	r := routeFor(home + "search/fixture")
	p, err := c.search(t.Context(), r)
	if err != nil || calls != 1 || len(p.items) != 3 || !slices.Equal(p.items[0].tags, []string{tag, "別タグ"}) ||
		p.items[0].ownerName != name || p.items[1].owner != home+"user/456/video" || p.items[2].owner != "https://ch.nicovideo.jp/ch50/video" {
		t.Fatalf("unexpected search result: %+v, %v, %d requests", p, err, calls)
	}
	b := &browser{navigation: []screenRoute{r}, result: p, closed: true}
	view := ui.NewTester(b.draw, 640, 640)
	for _, target := range []struct{ label, text string }{{name + "の投稿動画", name}, {"タグ検索 " + tag, tag}} {
		info, ok := view.Access(target.label)
		if !ok || !info.Focusable || info.Description != target.text || info.Bounds.W <= 0 || info.Bounds.W > 240 {
			t.Fatalf("metadata has no bounded button with full accessible text: %+v", info)
		}
		view.Move(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
		view.Advance(time.Second)
		if _, ok := view.AccessByRole(ui.RoleTooltip, target.text); !ok {
			t.Fatal("hover did not reveal full metadata")
		}
	}
	view.Move(0, 0)
	for _, target := range []struct{ label, text string }{{name + "の投稿動画", name}, {"タグ検索 " + tag, tag}} {
		for i := 0; i < 40 && !view.Focused(target.label); i++ {
			view.Key(0, ui.KeyTab)
		}
		if !view.Focused(target.label) {
			t.Fatal("metadata link is not keyboard focusable")
		}
		view.Advance(time.Second)
		if _, ok := view.AccessByRole(ui.RoleTooltip, target.text); !ok {
			t.Fatal("keyboard focus did not reveal full metadata")
		}
	}
	view.Key(0, ui.KeyEnter)
	if target := b.navigation[b.index]; target.kind != screenSearch || target.key != tag || accountSearchParameters(target).Get("tag") != tag {
		t.Fatalf("keyboard activation did not open exact tag search: %+v", target)
	}
	b.moveNative(-1)
	// Keep the narrow-width checks above. Give the list room for its second
	// row after the search range and source explanation wrap.
	view.SetSize(640, 850)
	if err := view.Click("投稿動画"); err != nil {
		t.Fatal(err)
	}
	if target := b.navigation[b.index]; target.kind != screenUser || target.key != "456" {
		t.Fatalf("missing name lost the existing link: %+v", target)
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
