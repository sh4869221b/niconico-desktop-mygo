package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestSearchDisplaysAvailableOrderingMetric(t *testing.T) {
	for _, snapshot := range []bool{false, true} {
		c := newCatalog(nil)
		c.http.Transport = requestFunc(func(*http.Request) (*http.Response, error) {
			body := `{"meta":{"status":200},"data":{"items":[{"id":"sm9","registeredAt":"2026-11-01T12:30:00+09:00","lastCommentTime":"2026-11-02T13:00:00+09:00","count":{"like":0,"mylist":42}}]}}`
			if snapshot {
				body = `{"meta":{"status":200},"data":[{"contentId":"sm9","startTime":"2026-11-01T12:30:00+09:00","lastCommentTime":"2026-11-02T13:00:00+09:00","likeCounter":0,"mylistCounter":42}]}`
			}
			return testResponse(200, body), nil
		})
		r := routeFor(home + "search/test?sort=f")
		var p catalogPage
		var err error
		if snapshot {
			p, err = c.publicSearch(t.Context(), r)
		} else {
			p, err = c.accountSearch(t.Context(), r, false)
		}
		if err != nil || len(p.items) != 1 {
			t.Fatal(p, err)
		}
		for key, want := range map[string]string{"f": "2026/11/01 12:30 +09:00", "n": "2026/11/02 13:00 +09:00", "likeCount": "いいね！数：0", "m": "マイリスト登録数：42"} {
			got := p.items[0].searchMetric(routeFor(home + "search/test?sort=" + key))
			if !strings.Contains(got, want) {
				t.Fatal(key, got)
			}
		}
	}
	if got := (card{}).searchMetric(routeFor(home + "search/test?sort=f")); got != "" {
		t.Fatal(got)
	}
}
