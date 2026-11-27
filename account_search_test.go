package main

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestAccountSearchAndGuestFallback(t *testing.T) {
	for _, tc := range []struct {
		name, key     string
		logged        bool
		status, calls int
		want          string
	}{
		{"keyword", "keyword", true, 200, 1, "ログイン検索"},
		{"tag", "tag", true, 200, 1, "ログイン検索"},
		{"logged-out", "keyword", false, 200, 1, "公開検索"},
		{"expired", "tag", true, 401, 2, "公開検索"},
		{"rate-limited", "tag", true, 429, 1, ""},
		{"redirect", "tag", true, 302, 1, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode := "search"
			if tc.key == "tag" {
				mode = "tag"
			}
			route := routeFor(searchURL(mode, "a/b") + "?sort=v&order=a")
			route.page = 2
			reads, requests := 0, 0
			c := newCatalog(func(raw string) (*http.Cookie, error) {
				reads++
				u, e := url.Parse(raw)
				if e != nil || u.Host != apiHost || u.Path != accountSearchPath || !sessionEndpoint(u) {
					t.Fatal("unexpected session destination")
				}
				if !tc.logged {
					return nil, errSignIn
				}
				return &http.Cookie{Name: "user_session", Value: "fixture", Secure: true}, nil
			})
			c.http.Transport = requestFunc(func(r *http.Request) (*http.Response, error) {
				requests++
				q := r.URL.Query()
				if r.Method != http.MethodGet || r.URL.Host != apiHost || r.URL.Path != accountSearchPath || q.Get(tc.key) != "a/b" ||
					q.Get("page") != "3" || q.Get("pageSize") != "32" || q.Get("sortKey") != "viewCount" || q.Get("sortOrder") != "asc" {
					t.Fatalf("unexpected search request: %s", r.URL)
				}
				status, wantCookie := 200, ""
				if requests == 1 && tc.logged {
					status, wantCookie = tc.status, "user_session=fixture"
				}
				if r.Header.Get("Cookie") != wantCookie {
					t.Fatal("unexpected search cookie")
				}
				resp := testResponse(status, `{"meta":{"status":200},"data":{"items":[{"id":"sm9","title":"test"}],"hasNext":true}}`)
				if status == 302 {
					resp.Header.Set("Location", "https://example.invalid/")
				}
				return resp, nil
			})
			p, err := c.search(context.Background(), route)
			if requests != tc.calls || reads != 1 || p.searchMode != tc.want || ((err != nil) != (tc.want == "")) {
				t.Fatalf("mode %q err %v, requests %d", p.searchMode, err, requests)
			}
			if tc.want != "" && (len(p.items) != 1 || p.items[0].link != home+"watch/sm9" || !p.more) {
				t.Fatal("bad decoded page")
			}
		})
	}
}
