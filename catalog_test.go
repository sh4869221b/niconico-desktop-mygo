package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
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
		input := base + "?sort=v&order=a&page=3&f_range=0#results"
		r := routeFor(resolve(input, false))
		if r.kind != screenSearch || r.page != 2 || r.target != input || r.link != base+"?order=a&sort=v" {
			t.Errorf("search route = %+v", r)
		}
		if p := searchParameters(r); p.Get("_offset") != "48" || p.Get("_sort") != "+viewCounter" {
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
			if next.page != page.index || next.link != r.link || u.Query().Get("page") != page.query || u.Query().Get("f_range") != "0" || u.Fragment != "results" {
				t.Errorf("changed search page = %+v", next)
			}
		}
		for _, page := range []string{"0", "-1", "x", "4168"} {
			if r := routeFor(base + "?page=" + page); r.page != 0 || r.target != base+"?page="+page {
				t.Errorf("unsupported page = %+v", r)
			}
		}
	}
}

func TestNativeCatalog(t *testing.T) {
	tests := []struct {
		route, path, response, title, first string
		more                                bool
	}{
		{home + "tag/%E7%8C%AB", "/api/v2/snapshot/video/contents/search",
			`{"meta":{"status":200,"totalCount":25},"data":[{"contentId":"sm9","title":"検索結果","lengthSeconds":75,"viewCounter":123}]}`, "猫", home + "watch/sm9", true},
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
				if routeFor(v.route).kind == screenSearch && (r.URL.Query().Get("targets") != "tagsExact" || r.URL.Query().Get("q") != "猫" || r.URL.Query().Get("_sort") != "-startTime") {
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
