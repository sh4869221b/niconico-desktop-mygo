package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

type requestFunc func(*http.Request) (*http.Response, error)

func (f requestFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testResponse(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestNativeRoutes(t *testing.T) {
	for _, tt := range []struct {
		u    string
		kind screenKind
		key  string
	}{
		{home, screenHome, ""}, {login, screenOfficial, ""},
		{home + "watch/sm9?x=1", screenVideo, "sm9"},
		{"https://nico.ms/SM9", screenVideo, "sm9"},
		{home + "tag/C%2B%2B%20%2F%20%23", screenSearch, "C++ / #"},
		{home + "user/123/video", screenUser, "123"},
		{home + "user/123/mylist", screenUserLists, "123"},
		{home + "user/123/mylist/16", screenList, "16"},
		{mylists, screenOwnLists, ""},
		{mylists + "/?x=1#/52", screenOwnList, "52"},
		{"https://www.nicovideo.jp.evil.test/watch/sm9", screenOfficial, ""},
	} {
		if r := routeFor(tt.u); r.kind != tt.kind || r.key != tt.key {
			t.Errorf("route(%s) = %#v", tt.u, r)
		}
	}
	if e, ok := makeEntry(mylists+"/?ref=x#/52", "リスト"); !ok || e.URL != mylists+"#/52" {
		t.Fatalf("pin = %#v (%v)", e, ok)
	}
}

func TestRouteTargetsAndSavedIdentity(t *testing.T) {
	watch := home + "watch/sm9"
	for _, input := range []string{watch, "https://sp.nicovideo.jp/watch/SM9", "https://nico.ms/sm9"} {
		target := watch + "?from=60&ref=test#player"
		r := routeFor(resolve(input+"?from=60&ref=test#player", false))
		if r.kind != screenVideo || r.link != watch || r.target != target || r.startTime() != time.Minute {
			t.Errorf("watch route for %q = %+v (start %s)", input, r, r.startTime())
		}
		if e, ok := makeEntry(r.target, "動画"); !ok || e.URL != watch {
			t.Errorf("saved watch = %+v, %v", e, ok)
		}
	}
	for _, from := range []string{"-5", "x", "9223372037", "18446744073709551616"} {
		if got := routeFor(watch + "?from=" + from).startTime(); got != 0 {
			t.Errorf("from=%s yielded %s", from, got)
		}
	}

	for _, path := range []string{"search/kitty", "tag/kitty"} {
		base := home + path
		input := base + "?sort=v&order=a&page=3#results"
		r := routeFor(resolve(input, false))
		if r.kind != screenSearch || r.page != 2 || r.target != input || r.link != base+"?order=a&sort=v" {
			t.Errorf("search route = %+v", r)
		}
		if p := searchParameters(r); p.Get("_offset") != "64" || p.Get("_sort") != "+viewCounter" {
			t.Errorf("snapshot search = %v", p)
		}
		if p := accountSearchParameters(r); p.Get("page") != "3" || p.Get("sortKey") != "viewCount" {
			t.Errorf("account search = %v", p)
		}
		for _, page := range []struct {
			index int
			query string
		}{{0, ""}, {3, "4"}} {
			next := r.withPage(page.index)
			u, _ := url.Parse(next.target)
			if next.page != page.index || next.link != r.link || u.Query().Get("page") != page.query || u.Query().Get("sort") != "v" || u.Fragment != "results" {
				t.Errorf("changed search page = %+v", next)
			}
		}
	}
}

func TestFilteredSearchRemainsOfficial(t *testing.T) {
	c := newCatalog(func(string) (*http.Cookie, error) {
		t.Fatal("official search read a session cookie for the native API")
		return nil, nil
	})
	c.http.Transport = requestFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("official search used the native catalog")
		return nil, nil
	})
	for _, path := range []string{"search/kitty", "tag/kitty"} {
		for _, query := range []string{"page=3&l_range=1", "f_range=0", "sort=unknown", "order=other", "page=0", "page=-1", "page=x", "page=4168", "page=1&page=3", "sort=f&order=%zz"} {
			input := home + path + "?" + query
			r := routeFor(resolve(input, false))
			if r.kind != screenOfficial || r.target != input {
				t.Fatalf("search routed away from %q: %+v", input, r)
			}
			if _, err := c.load(t.Context(), r); err != nil {
				t.Fatal(err)
			}
			if e, ok := makeEntry(input, "絞り込み"); !ok || e.URL != input || routeFor(e.URL).kind != screenOfficial {
				t.Errorf("saved official search = %+v, %v", e, ok)
			}
		}
	}
}

func TestUserVideoPagesMatchTheOfficialPostingList(t *testing.T) {
	base := home + "user/21962843/video"
	route := routeFor(base + "?page=3")
	if route.kind != screenUser || route.page != 2 || route.target != base+"?page=3" {
		t.Fatalf("posting page 3: %+v", route)
	}
	c := newCatalog(nil)
	for index, current := range []screenRoute{route, route.withPage(route.page + 1)} {
		wantPage := strconv.Itoa(index + 3)
		if current.target != base+"?page="+wantPage {
			t.Fatalf("official page target = %q", current.target)
		}
		c.http.Transport = requestFunc(func(r *http.Request) (*http.Response, error) {
			q := r.URL.Query()
			if r.URL.Path != "/v3/users/21962843/videos" || q.Get("page") != wantPage || q.Get("pageSize") != "100" ||
				q.Get("sortKey") != "registeredAt" || q.Get("sortOrder") != "desc" || q.Get("selectContentType") != "long" || q.Get("sensitiveContents") != "mask" {
				t.Fatalf("posting list request = %s", r.URL)
			}
			return testResponse(http.StatusOK, `{"meta":{"status":200},"data":{"totalCount":301,"items":[]}}`), nil
		})
		page, err := c.load(t.Context(), current)
		if err != nil || page.more != (index == 0) {
			t.Fatalf("page %s, has next %v, error %v", wantPage, page.more, err)
		}
	}
	if previous := route.withPage(0); previous.target != base || previous.page != 0 {
		t.Fatalf("first page: %+v", previous)
	}
	for _, raw := range []string{base + "?page=3&sortKey=viewCount", base + "?page=2&page=3", home + "user/21962843?page=3", "https://sp.nicovideo.jp/user/21962843/video?page=3"} {
		r := routeFor(raw)
		if r.kind != screenOfficial || !r.browsesInMain() || r.target != raw {
			t.Errorf("posting list filters were not handed off intact: %+v", r)
		}
		if e, ok := makeEntry(raw, "投稿一覧"); !ok || e.URL != raw {
			t.Errorf("saved posting list filters: %+v", e)
		}
	}
}

func TestPublicMylistKeepsOfficialSortAndPageBoundaries(t *testing.T) {
	const base = home + "mylist/7473771"
	const sorted = base + "?page=2&sortKey=addedAt&sortOrder=asc"
	for _, target := range []string{sorted, home + "user/219575/mylist/7473771?page=2&sortKey=addedAt&sortOrder=asc"} {
		route := routeFor(target)
		if route.kind != screenOfficial || !route.browsesInMain() || route.target != target {
			t.Fatalf("sorted list conditions did not reach the official view: %+v", route)
		}
		if pin, ok := makeEntry(target, "マイリスト"); !ok || pin.URL != target {
			t.Fatalf("saved list conditions = %+v", pin)
		}
	}
	for _, base := range []string{base, home + "user/219575/mylist/7473771"} {
		next := routeFor(base).withPage(1)
		if next.target != base+"?page=2" || routeFor(next.target).page != 1 || routeFor(next.target).kind != screenList {
			t.Fatalf("official target after Next = %+v", next)
		}
		c := newCatalog(nil)
		c.http.Transport = requestFunc(func(r *http.Request) (*http.Response, error) {
			q := r.URL.Query()
			if r.URL.Path != "/v2/mylists/7473771" || q.Get("page") != "2" || q.Get("pageSize") != "100" ||
				q.Get("sensitiveContents") != "mask" || q.Has("sortKey") || q.Has("sortOrder") {
				t.Fatalf("default list request disagrees with official browser: %s", r.URL)
			}
			return testResponse(http.StatusOK, `{"meta":{"status":200},"data":{"mylist":{"name":"一覧","totalItemCount":156,"items":[]}}}`), nil
		})
		if page, err := c.load(t.Context(), next); err != nil || page.more {
			t.Fatalf("after the last 56 items: %+v, %v", page, err)
		}
	}
}

func TestNativeCatalog(t *testing.T) {
	tests := []struct {
		route, path, response, title, first string
		more                                bool
	}{
		{home + "tag/%E7%8C%AB", accountSearchPath,
			`{"meta":{"status":200},"data":{"hasNext":true,"items":[{"id":"sm9","title":"検索結果","duration":75,"count":{"view":123}}]}}`, "猫", home + "watch/sm9", true},
		{home + "user/123/video", "/v3/users/123/videos", `{"meta":{"status":200},"data":{"totalCount":1,"items":[{"essential":{"id":"sm9","title":"動画","owner":{"id":"123","ownerType":"user","name":"投稿者"}}}]}}`, "投稿者 の投稿動画", home + "watch/sm9", false},
		{home + "user/123/mylist", "/v1/users/123/mylists", `{"meta":{"status":200},"data":{"mylists":[{"id":4,"name":"private"},{"id":3,"name":"public","isPublic":true}]}}`, "公開マイリスト", home + "mylist/3", false},
		{home + "mylist/3", "/v2/mylists/3", `{"meta":{"status":200},"data":{"mylist":{"name":"公開 3","hasNext":true,"items":[{"watchId":"sm9","video":{"id":"sm9","title":"動画"}}]}}}`, "公開 3", home + "watch/sm9", true},
		{mylists, "/v1/users/me/mylists", `{"meta":{"status":200},"data":{"mylists":[{"id":4,"name":"私のリスト"}]}}`, "自分のマイリスト", mylists + "#/4", false},
		{mylists + "#/4", "/v1/users/me/mylists/4", `{"meta":{"status":200},"data":{"mylist":{"name":"非公開 4","items":[{"video":{"id":"sm9","title":"動画"}}]}}}`, "非公開 4", home + "watch/sm9", false},
	}
	for _, v := range tests {
		t.Run(v.path, func(t *testing.T) {
			private := strings.Contains(v.path, "/me/")
			authCalls := 0
			c := newCatalog(func(raw string) (*http.Cookie, error) {
				authCalls++
				u, err := url.Parse(raw)
				if err != nil || !sessionEndpoint(u) || u.Path != v.path {
					t.Errorf("unexpected cookie URL %q", raw)
				}
				return &http.Cookie{Name: "user_session", Value: "fixture", Secure: true}, nil
			})
			c.http.Transport = requestFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodGet || r.URL.Path != v.path || (r.Header.Get("Cookie") != "") != private {
					t.Errorf("unexpected request to %s or auth state", r.URL.Path)
				}
				if routeFor(v.route).kind == screenSearch && (r.URL.Query().Get("tag") != "猫" || r.URL.Query().Get("sortKey") != "hot" || r.URL.Query().Get("sortOrder") != "none") {
					t.Error(r.URL.RawQuery)
				}
				return testResponse(200, v.response), nil
			})
			if routeFor(v.route).kind == screenSearch {
				c.session = nil
			}
			p, err := c.load(context.Background(), routeFor(v.route))
			if err != nil || p.title != v.title || len(p.items) != 1 || p.items[0].link != v.first || p.more != v.more || (authCalls == 1) != private {
				t.Fatalf("page = %#v; err=%v calls=%d", p, err, authCalls)
			}
		})
	}
}

func TestSessionScope(t *testing.T) {
	route := routeFor(mylists)
	c := newCatalog(nil)
	c.http.Transport = requestFunc(func(*http.Request) (*http.Response, error) { t.Fatal("request without a session"); return nil, nil })
	_, err := c.load(context.Background(), route)
	if !errors.Is(err, errSignIn) {
		t.Fatal(err)
	}
	c = newCatalog(func(s string) (*http.Cookie, error) {
		return &http.Cookie{Name: "user_session", Value: "fixture", Secure: true}, nil
	})
	calls := 0
	c.http.Transport = requestFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls != 1 || r.URL.Host != apiHost || r.Header.Get("Cookie") != "user_session=fixture" {
			t.Fatalf("scope violation, path=%q", r.URL.Path)
		}
		resp := testResponse(http.StatusFound, "")
		resp.Header.Set("Location", "https://example.invalid/")
		return resp, nil
	})
	if _, err = c.load(context.Background(), route); err == nil || calls != 1 {
		t.Fatal("unexpected redirect result")
	}
	c.http.Transport = requestFunc(func(*http.Request) (*http.Response, error) { return testResponse(http.StatusUnauthorized, ""), nil })
	if _, err = c.load(context.Background(), route); !errors.Is(err, errSignIn) {
		t.Fatal(err)
	}
}

func TestSessionCookieFilter(t *testing.T) {
	target := "https://" + apiHost + "/v1/users/me/mylists?page=1"
	now := time.Unix(2000, 0)
	list := func(filter mygo.CookieFilter) ([]mygo.Cookie, error) {
		if filter.URL != target || filter.Name != "user_session" {
			t.Fatalf("wrong filter: %#v", filter)
		}
		return []mygo.Cookie{
			{Name: "lang", Domain: ".nicovideo.jp", Value: "ja-jp", Secure: true},
			{Name: "user_session", Domain: "www.nicovideo.jp", Value: "wrong-host", Secure: true},
			{Name: "user_session", Domain: ".nicovideo.jp", Value: "insecure"},
			{Name: "user_session", Domain: ".nicovideo.jp", Value: "expired", Secure: true, Expires: now.Add(-time.Second)},
			{Name: "user_session", Domain: ".nicovideo.jp", Value: "fixture", Secure: true, HTTPOnly: true},
		}, nil
	}
	got, err := readSessionCookie(list, target, now)
	if err != nil || got.Value != "fixture" {
		t.Fatal("no scoped session")
	}
	for _, raw := range []string{"http://" + apiHost + "/v1/users/me/mylists", "https://other.invalid/v1/users/me/mylists", "https://" + apiHost + "/v2/mylists/123", "https://" + apiHost + "/v1/users/me/mylists/1/items"} {
		if _, err := readSessionCookie(list, raw, now); !errors.Is(err, errSignIn) {
			t.Fatal(raw, err)
		}
	}
}

func TestNativeVideoDescription(t *testing.T) {
	c := newCatalog(nil)
	c.http.Transport = requestFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "ext.nicovideo.jp" || r.URL.Path != "/api/getthumbinfo/sm9" || r.Header.Get("Cookie") != "" {
			t.Fatal("incorrect video request")
		}
		return testResponse(200, `<nicovideo_thumb_response status="ok"><thumb><title>動画</title><description><![CDATA[文字<br><script>ignored</script><a href="/mylist/52">一覧</a> https://nico.ms/sm12?from=60。 https://www.nicovideo.jp.evil.test/watch/sm15]]></description><user_id>123</user_id><user_nickname>投稿者</user_nickname></thumb></nicovideo_thumb_response>`), nil
	})
	p, err := c.load(context.Background(), routeFor(home+"watch/sm9"))
	if err != nil || p.title != "動画" || p.author != "投稿者" || len(p.links) != 4 || p.links[2].link != home+"mylist/52" || p.links[3].link != home+"watch/sm12?from=60" || strings.Contains(p.summary, "ignored") {
		t.Fatalf("page = %#v err=%v", p, err)
	}
}

func TestDescriptionLinksKeepSeparateStartTimes(t *testing.T) {
	watch := home + "watch/sm9"
	_, links := plainDescription(`
		<a href="/watch/sm9?from=60&amp;ref=test">1分</a>
		<a href="https://nico.ms/sm9?from=60">同じ開始位置</a>
		https://nico.ms/sm9?from=120。
		https://www.nicovideo.jp/watch/sm9
	`)
	if len(links) != 3 || links[0].link != watch+"?from=60&ref=test" || links[1].link != watch+"?from=120" || links[2].link != watch {
		t.Fatalf("description links = %+v", links)
	}
	if links[0].title == links[1].title {
		t.Fatal("different start positions have identical labels")
	}
	for _, link := range links {
		if e, ok := makeEntry(link.link, "動画"); !ok || e.URL != watch {
			t.Errorf("saved watch = %+v, %v", e, ok)
		}
	}
}

func TestVideoDetailsRejectMismatchedID(t *testing.T) {
	c := newCatalog(nil)
	c.http.Transport = requestFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/getthumbinfo/sm9" || r.Header.Get("Cookie") != "" {
			t.Fatalf("unexpected metadata request: %s", r.URL)
		}
		return testResponse(200, `<nicovideo_thumb_response status="ok"><thumb><video_id>sm12</video_id><title>別の動画</title></thumb></nicovideo_thumb_response>`), nil
	})
	if _, err := c.video(t.Context(), routeFor(home+"watch/sm9")); err == nil {
		t.Fatal("accepted details for another video")
	}
}

func TestLivePublicCatalog(t *testing.T) {
	if os.Getenv("NICO_LIVE_TEST") != "1" {
		t.Skip("opt-in public endpoint smoke test")
	}
	c := newCatalog(nil)
	for _, target := range []string{home + "tag/%E5%88%9D%E9%9F%B3%E3%83%9F%E3%82%AF", home + "search/%E7%8C%AB", home + "user/805442/video", home + "user/805442/mylist", home + "mylist/34076173", home + "watch/sm9"} {
		t.Run(target, func(t *testing.T) {
			r := routeFor(target)
			p, err := c.load(context.Background(), r)
			if err != nil || p.title == "" || r.kind != screenVideo && len(p.items) == 0 {
				t.Fatalf("page empty or not available: %v", err)
			}
		})
	}
}
