package main

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSearchSortPreferenceFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "search-sort.json")
	if loadSearchSort(path) != (searchSortPreference{}) {
		t.Fatal("missing settings did not use the default sort")
	}
	for _, data := range []string{`{"sort":"unsupported","ascending":true}`, `{"player":"native"}`, `{"sort":"hotLikeAndMylist","ascending":true}`, `{"sort":5}`, `{broken`, strings.Repeat(" ", 4097)} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if loadSearchSort(path) != (searchSortPreference{}) {
			t.Fatal("old or invalid settings did not use the default sort")
		}
	}
	pref := searchSortPreference{Sort: "v", Ascending: true}
	for _, suffix := range []string{"?sort=", "?sort=hot", "?sort=f&order=d", "?order=d"} {
		route := routeFor(home + "search/fresh" + suffix)
		if route.kind != screenSearch || pref.apply(route) != route {
			t.Fatalf("explicit ordering was overwritten: %s", suffix)
		}
	}
}

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
			if !tc.snapshot || r.URL.Host != "snapshot.search.nicovideo.jp" || !slices.Contains(strings.Split(q.Get("fields"), ","), "tags") ||
				q.Get("_sort") != "+likeCounter" || q.Get("_offset") != "32" || q.Get("targets") != "tagsExact" {
				t.Fatalf("unexpected snapshot request: %s", r.URL)
			}
			return testResponse(200, `{"meta":{"status":200},"data":[{"contentId":"sm9","tags":"初音ミク 曲　名 初音ミク","thumbnailUrl":"https://nicovideo.cdn.nimg.jp/thumbnails/9/9"}]}`), nil
		})
		p, err := c.load(t.Context(), routeFor(home+"tag/kitty?page=2&"+tc.query))
		if tc.snapshot {
			if err != nil || calls != 2 || p.searchMode != "スナップショット検索" || len(p.items) != 1 || p.items[0].thumbnail == "" ||
				!slices.Equal(p.items[0].tags, []string{"初音ミク", "曲　名"}) {
				t.Fatalf("snapshot result %+v, %v, %d requests", p, err, calls)
			}
		} else if err == nil || calls != 1 {
			t.Fatalf("unsupported snapshot sort fell back: %+v, %v, %d requests", p, err, calls)
		}
	}
}
