package main

import (
	"net/http"
	"net/url"
	"testing"
)

func TestOfficialSearchOrdering(t *testing.T) {
	for _, mode := range []string{"search", "tag"} {
		for _, tc := range []struct{ urlKey, apiKey, snapshot string }{
			{"", "hot", ""}, {"hotLikeAndMylist", "hot", ""}, {"personalized", "personalized", ""},
			{"f", "registeredAt", "startTime"}, {"v", "viewCount", "viewCounter"},
			{"n", "lastCommentTime", "lastCommentTime"}, {"likeCount", "likeCount", "likeCounter"},
			{"r", "commentCount", "commentCounter"}, {"m", "mylistCount", "mylistCounter"},
			{"l", "duration", "lengthSeconds"},
		} {
			for _, order := range []string{"", "a", "d"} {
				q := url.Values{"page": {"2"}}
				if tc.urlKey != "" {
					q.Set("sort", tc.urlKey)
				}
				if order != "" {
					q.Set("order", order)
				}
				r := routeFor(searchURL(mode, "a/b") + "?" + q.Encode())
				if r.kind != screenSearch || r.page != 1 || r.key != "a/b" {
					t.Fatalf("invalid search route: %+v", r)
				}
				p := accountSearchParameters(r.withPage(2))
				key := "keyword"
				if mode == "tag" {
					key = "tag"
				}
				apiOrder, prefix := "desc", "-"
				if tc.snapshot == "" {
					apiOrder = "none"
				} else if order == "a" {
					apiOrder, prefix = "asc", "+"
				}
				if p.Get(key) != "a/b" || p.Get("page") != "3" || p.Get("sortKey") != tc.apiKey || p.Get("sortOrder") != apiOrder {
					t.Fatalf("%s mapped to %v", r.target, p)
				}
				if tc.snapshot != "" && searchParameters(r).Get("_sort") != prefix+tc.snapshot {
					t.Fatalf("%s mapped to %v", r.target, searchParameters(r))
				}
			}
		}
	}
}

func TestGuestSearchSnapshotFallbackKeepsSort(t *testing.T) {
	for _, tc := range []struct {
		query    string
		snapshot bool
	}{
		{"sort=likeCount&order=a", true},
		{"sort=personalized", false},
		{"", false},
	} {
		c := newCatalog(nil)
		calls := 0
		c.http.Transport = requestFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.Header.Get("Cookie") != "" {
				t.Fatal("unexpected cookie in guest search")
			}
			if r.URL.Host == apiHost {
				return testResponse(503, ""), nil
			}
			q := r.URL.Query()
			if !tc.snapshot || r.URL.Host != "snapshot.search.nicovideo.jp" ||
				q.Get("_sort") != "+likeCounter" || q.Get("_offset") != "32" || q.Get("targets") != "tagsExact" {
				t.Fatalf("unexpected snapshot request: %s", r.URL)
			}
			return testResponse(200, `{"meta":{"status":200},"data":[{"contentId":"sm9","thumbnailUrl":"https://nicovideo.cdn.nimg.jp/thumbnails/9/9"}]}`), nil
		})
		p, err := c.load(t.Context(), routeFor(home+"tag/kitty?page=2&"+tc.query))
		if tc.snapshot {
			if err != nil || calls != 2 || p.searchMode != "スナップショット検索" || len(p.items) != 1 || p.items[0].thumbnail == "" {
				t.Fatalf("snapshot result %+v, %v, %d requests", p, err, calls)
			}
		} else if err == nil || calls != 1 {
			t.Fatalf("unsupported snapshot sort fell back: %+v, %v, %d requests", p, err, calls)
		}
	}
}
