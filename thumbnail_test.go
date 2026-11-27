package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCatalogThumbnails(t *testing.T) {
	const preview = "https://img.cdn.nimg.jp/s/nicovideo/thumbnails/9/9.original/r320x180l.webp?key=fixture"
	for _, tc := range []struct{ target, response string }{
		{home + "search/猫", `{"items":[{"id":"sm9","thumbnail":{"listingUrl":"` + preview + `"}}]}`},
		{home + "mylist/3", `{"mylist":{"items":[{"watchId":"sm9","video":{"title":"動画","thumbnail":{"listingUrl":"` + preview + `"}}}]}}`},
		{mylists + "#/3", `{"mylist":{"items":[{"video":{"id":"sm9","thumbnail":{"listingUrl":"` + preview + `"}}}]}}`},
		{home + "user/123/mylist", `{"mylists":[{"id":3,"isPublic":true,"sampleItems":[{"video":{"thumbnail":{"listingUrl":"` + preview + `"}}}]}]}`},
		{mylists, `{"mylists":[{"id":3,"sampleItems":[{"video":{"thumbnail":{"listingUrl":"` + preview + `"}}}]}]}`},
	} {
		t.Run(tc.target, func(t *testing.T) {
			c := newCatalog(func(string) (*http.Cookie, error) {
				return &http.Cookie{Name: "user_session", Value: "fixture", Secure: true}, nil
			})
			c.http.Transport = requestFunc(func(r *http.Request) (*http.Response, error) {
				if route := routeFor(tc.target); (route.kind == screenUserLists || route.kind == screenOwnLists) && r.URL.Query().Get("sampleItemCount") != "1" {
					t.Fatal("directory request did not ask for cover samples")
				}
				return testResponse(200, `{"meta":{"status":200},"data":`+tc.response+`}`), nil
			})
			page, err := c.load(t.Context(), routeFor(tc.target))
			if err != nil || len(page.items) != 1 || page.items[0].thumbnail != preview {
				t.Fatalf("thumbnail data = %+v, %v", page, err)
			}
		})
	}
}

func TestThumbnailFetch(t *testing.T) {
	const target = "https://nicovideo.cdn.nimg.jp/thumbnails/9/9"
	for _, raw := range []string{
		"https://example.com/thumbnails/9/9", "https://nicovideo.cdn.nimg.jp.evil.example/thumbnails/9/9",
		"https://user@nicovideo.cdn.nimg.jp/thumbnails/9/9", "https://nicovideo.cdn.nimg.jp:446/thumbnails/9/9",
	} {
		if normalizeThumbnailURL(raw) != "" {
			t.Errorf("accepted %q", raw)
		}
	}
	if got := normalizeThumbnailURL("http" + target[5:]); got != target {
		t.Fatalf("HTTP thumbnail normalized to %q", got)
	}
	img := image.NewRGBA(image.Rect(0, 0, 640, 360))
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	s := newThumbnails(nil)
	calls := 0
	s.client.Transport = requestFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != target || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Fatalf("unexpected thumbnail request: %s", r.URL)
		}
		if calls == 2 {
			resp := testResponse(302, "")
			resp.Header.Set("Location", "https://example.com/")
			return resp, nil
		}
		return testResponse(200, data.String()), nil
	})
	decoded := fetchThumbnail(t.Context(), s.client, target)
	if decoded == nil {
		t.Fatal("thumbnail did not decode")
	}
	if w, h := decoded.Size(); w != 320 || h != 180 {
		t.Fatalf("thumbnail dimensions %d x %d", w, h)
	}
	if fetchThumbnail(t.Context(), s.client, target) != nil || calls != 2 {
		t.Fatal("redirected thumbnail was followed")
	}
}

func TestThumbnailViewportAndCache(t *testing.T) {
	s := newThumbnails(nil)
	var contexts []context.Context
	for i := range thumbnailWorkers {
		ctx, cancel := context.WithCancel(t.Context())
		contexts = append(contexts, ctx)
		s.active[strconv.Itoa(i)] = cancel
	}
	s.setVisible([]card{{thumbnail: "2"}, {thumbnail: "new"}, {thumbnail: "2"}})
	if len(s.active) != thumbnailWorkers || len(s.wanted) != 2 || s.wanted[0] != "2" || s.wanted[1] != "new" {
		t.Fatalf("unexpected viewport: %v, %d jobs", s.wanted, len(s.active))
	}
	for i, ctx := range contexts {
		if (ctx.Err() == nil) != (i == 2) {
			t.Errorf("request %d cancel state: %v", i, ctx.Err())
		}
	}
	for i := range thumbnailCacheSize {
		s.store(strconv.Itoa(i), nil)
	}
	_ = s.image("0")
	s.store("new", nil)
	if _, exists := s.cache["1"]; exists || len(s.cache) != thumbnailCacheSize {
		t.Fatal("cache did not evict the least recently used entry")
	}
}

func TestCatalogRendersThumbnailInLightAndDark(t *testing.T) {
	const target = "https://nicovideo.cdn.nimg.jp/thumbnails/9/9"
	want := color.RGBA{R: 0x29, G: 0xcf, B: 0x78, A: 255}
	preview := image.NewRGBA(image.Rect(0, 0, 112, 63))
	for y := range 63 {
		for x := range 112 {
			preview.SetRGBA(x, y, want)
		}
	}
	for _, theme := range []*ui.Theme{ui.LightTheme(), ui.DarkTheme()} {
		r := routeFor(home + "mylist/3")
		app := browser{
			navigation: []screenRoute{r},
			result: catalogPage{title: "マイリスト", items: []card{
				{link: home + "watch/sm9", title: "画像のある動画", thumbnail: target},
				{link: home + "watch/sm10", title: "画像のない動画"},
			}},
			thumbnails:  newThumbnails(nil),
			showLibrary: true,
			closed:      true,
		}
		app.thumbnails.store(target, ui.NewBitmap(preview))
		view := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme)
			app.draw(c)
		}, 640, 520)
		rendered := view.Image()
		found := false
		for y := range rendered.Bounds().Dy() {
			for x := range rendered.Bounds().Dx() {
				found = found || rendered.RGBAAt(x, y) == want
			}
		}
		if !found {
			t.Fatal("thumbnail pixels are missing")
		}
		if err := view.Click("画像のない動画"); err != nil {
			t.Fatal(err)
		}
		if app.navigation[app.index].target != home+"watch/sm10" {
			t.Fatal("missing thumbnail prevented opening a video")
		}
	}
}
