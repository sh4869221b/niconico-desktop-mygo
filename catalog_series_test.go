package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSeriesRoutesAndSessionScope(t *testing.T) {
	for _, tc := range []struct {
		link, key string
		kind      screenKind
		page      int
	}{
		{home + "user/123/series?page=2", "123", screenUserSeries, 1},
		{home + "user/123/series/456?page=3", "456", screenSeries, 2},
		{"https://sp.nicovideo.jp/series/456", "456", screenSeries, 0},
		{"https://sp.nicovideo.jp/series/456?page=2", "456", screenSeries, 1},
		{"https://nico.ms/series/456", "456", screenSeries, 0},
		{"https://ch.nicovideo.jp/ch123/series", "", screenOfficial, 0},
		{home + "user/1234567890/series", "series", screenOfficial, 0},
	} {
		r := routeFor(tc.link)
		if r.kind != tc.kind || r.key != tc.key || r.page != tc.page {
			t.Errorf("%s opened %+v", tc.link, r)
		}
		if r.page > 0 && r.withPage(0).target != r.link {
			t.Errorf("first page lost the series route: %+v", r.withPage(0))
		}
	}
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/v1/users/123/series", true},
		{"/v2/series/456", true},
		{"/v1/users/me/series", false},
		{"/v1/users/123/series/456", false},
		{"/v2/series/456/extra", false},
	} {
		u := &url.URL{Scheme: "https", Host: apiHost, Path: tc.path}
		if sessionEndpoint(u) != tc.want {
			t.Errorf("incorrect cookie scope for %q", tc.path)
		}
	}
	if (publisherFor("channel", "ch123", "公式")).seriesURL() != "" {
		t.Fatal("channel was treated as a regular user directory")
	}
}

func TestSeriesLoadPagesAndPlayThroughCatalog(t *testing.T) {
	entries := make([]struct {
		ID    int
		Title string
	}, pageSize+1)
	for i := range entries {
		entries[i].ID, entries[i].Title = 456+i, "シリーズのタイトル"
	}
	directoryJSON, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	c := newCatalog(nil)
	c.http.Transport = requestFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != apiHost || r.Header.Get("Cookie") != "" {
			t.Fatal("public series request used the wrong host or a cookie")
		}
		switch r.URL.Path {
		case "/v1/users/123/series":
			if r.URL.RawQuery != "" {
				t.Fatal("unexpected server pagination on the directory")
			}
			return testResponse(200, `{"meta":{"status":200},"data":{"items":`+string(directoryJSON)+`}}`), nil
		case "/v2/series/456":
			q := r.URL.Query()
			if q.Get("pageSize") != "100" || (q.Get("page") != "1" && q.Get("page") != "2") {
				t.Fatalf("video page request: %v", q)
			}
			return testResponse(200, `{"meta":{"status":200},"data":{"totalCount":101,"detail":{"title":"シリーズのタイトル","owner":{"id":123,"name":"投稿者名"}},"items":[{"video":{"id":"sm9","title":"シリーズの動画","duration":81}}]}}`), nil
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
			return nil, nil
		}
	})
	r := routeFor(home + "user/123/video")
	b := &browser{navigation: []screenRoute{r}, closed: true, playMode: nativePlayback}
	view := ui.NewTester(b.draw, 800, 650)
	click := func(label string) screenRoute {
		t.Helper()
		if err := view.Click(label); err != nil {
			t.Fatal(err)
		}
		return b.navigation[b.index]
	}
	if calls != 0 {
		t.Fatal("series was fetched before opening it")
	}
	directory := click("シリーズ")
	if directory.kind != screenUserSeries || directory.key != "123" {
		t.Fatalf("wrong directory route: %+v", directory)
	}
	first, err := c.load(t.Context(), directory)
	if err != nil || len(first.items) != pageSize || !first.more {
		t.Fatalf("first directory page: %d items, next %t, %v", len(first.items), first.more, err)
	}
	second, err := c.load(t.Context(), directory.withPage(1))
	if err != nil || len(second.items) != 1 || second.more || second.items[0].link == first.items[0].link {
		t.Fatalf("second directory page: %+v, %v", second, err)
	}
	b.result = first
	view.Frame()
	detail := click("シリーズのタイトル")
	if detail.kind != screenSeries || detail.key != "456" {
		t.Fatalf("wrong series route: %+v", detail)
	}
	page, err := c.load(t.Context(), detail)
	if err != nil || len(page.items) != 1 || !page.more || page.publisher.name != "投稿者名" {
		t.Fatalf("series video page: %+v, %v", page, err)
	}
	b.result = page
	view.Frame()
	click("シリーズの動画 を再生")
	if b.playbackRequest != 1 || b.navigation[b.index] != detail {
		t.Fatal("series row did not take the usual playback entry point")
	}
	page, err = c.load(t.Context(), click("次へ"))
	if err != nil || page.more {
		t.Fatalf("final video page: %+v, %v", page, err)
	}
}

func TestSeriesEmptyDeniedAndFailedResponses(t *testing.T) {
	for _, tc := range []struct {
		name, body, message string
		status              int
	}{
		{"empty", `{"meta":{"status":200},"data":{"items":[]}}`, "公開されているシリーズはありません", 200},
		{"private row", `{"meta":{"status":200},"data":{"items":[{"id":456,"isPublic":false}]}}`, "非公開", 200},
		{"forbidden", "", "閲覧できません", 403},
		{"not found", `{"meta":{"status":404}}`, "見つかりません", 200},
		{"bad shape", `{"meta":{"status":200},"data":{}}`, "応答形式", 200},
		{"service error", "", "HTTP 503", 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newCatalog(nil)
			c.http.Transport = requestFunc(func(r *http.Request) (*http.Response, error) {
				return testResponse(tc.status, tc.body), nil
			})
			r := routeFor(home + "user/123/series")
			p, err := c.load(t.Context(), r)
			b := &browser{navigation: []screenRoute{r}, result: p, resultErr: err, closed: true}
			view := ui.NewTester(b.draw, 800, 650)
			if !view.HasText(tc.message) || len(p.items) != 0 || (err == nil) != (tc.name == "empty" || tc.name == "private row") {
				t.Fatalf("unexpected series state: items %d, error %v, view %v", len(p.items), err, view.Texts())
			}
		})
	}
}

func TestSeriesRetriesAnAccessDenialWithTheAppSession(t *testing.T) {
	const endpoint = "https://" + apiHost + "/v2/series/456"
	c := newCatalog(func(target string) (*http.Cookie, error) {
		if !strings.HasPrefix(target, endpoint+"?") {
			t.Fatal("session read outside the series endpoint")
		}
		return &http.Cookie{Name: "user_session", Value: "fixture", Secure: true}, nil
	})
	requests := 0
	c.http.Transport = requestFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			if r.Header.Get("Cookie") != "" {
				t.Fatal("the first request was not public")
			}
			return testResponse(200, `{"meta":{"status":403}}`), nil
		}
		if _, err := r.Cookie("user_session"); err != nil {
			t.Fatal("series access retry did not use the app's session")
		}
		return testResponse(200, `{"meta":{"status":200},"data":{"items":[]}}`), nil
	})
	_, err := c.load(t.Context(), routeFor(home+"series/456"))
	if err != nil || requests != 2 {
		t.Fatalf("retry returned %v after %d requests", err, requests)
	}
}
