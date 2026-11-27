package main

import (
	"encoding/json"
	"io"
	"net/url"
	"os"
)

const searchPageSize = 32
const snapshotResultLimit = 100000

type searchOrdering struct {
	urlKey, label, apiKey string
	// The two ranking modes have no direction and no Snapshot equivalent.
	snapshotKey string
}

// The desktop keyword and tag pages expose the same options in this order.
var searchSorts = [...]searchOrdering{
	{"", "ニコニコで人気", "hot", ""},
	{"personalized", "あなたへのおすすめ", "personalized", ""},
	{"f", "投稿日時", "registeredAt", "startTime"},
	{"v", "再生数", "viewCount", "viewCounter"},
	{"n", "コメント日時", "lastCommentTime", "lastCommentTime"},
	{"likeCount", "いいね！数", "likeCount", "likeCounter"},
	{"r", "コメント数", "commentCount", "commentCounter"},
	{"m", "マイリスト登録数", "mylistCount", "mylistCounter"},
	{"l", "再生時間", "duration", "lengthSeconds"},
}

var searchSortLabels = func() []string {
	labels := make([]string, 0, len(searchSorts))
	for _, sort := range searchSorts {
		labels = append(labels, sort.label)
	}
	return labels
}()

func findSearchSort(key string) (searchOrdering, bool) {
	if key == "hotLikeAndMylist" || key == "hot" {
		return searchSorts[0], true
	}
	for _, sort := range searchSorts {
		if sort.urlKey == key {
			return sort, true
		}
	}
	return searchOrdering{}, false
}

func (r screenRoute) searchSort() (searchOrdering, bool) {
	u, _ := url.Parse(r.link)
	q := u.Query()
	sort, _ := findSearchSort(q.Get("sort"))
	return sort, sort.snapshotKey != "" && q.Get("order") == "a"
}

func (r screenRoute) withSearchSort(sort searchOrdering, ascending bool) screenRoute {
	u, _ := url.Parse(r.target)
	q := u.Query()
	q.Del("page")
	q.Del("order")
	q.Del("sort")
	if sort.urlKey != "" {
		q.Set("sort", sort.urlKey)
	}
	if sort.snapshotKey != "" && ascending {
		q.Set("order", "a")
	}
	u.RawQuery = q.Encode()
	return routeFor(u.String())
}

type searchSortPreference struct {
	Sort      string `json:"sort"`
	Ascending bool   `json:"ascending"`
}

func loadSearchSort(path string) searchSortPreference {
	f, err := os.Open(path)
	if err != nil {
		return searchSortPreference{}
	}
	defer f.Close()
	const limit = 4096
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	var pref searchSortPreference
	if err != nil || len(data) > limit || json.Unmarshal(data, &pref) != nil {
		return searchSortPreference{}
	}
	sort, ok := findSearchSort(pref.Sort)
	if !ok {
		return searchSortPreference{}
	}
	return searchSortPreference{Sort: sort.urlKey, Ascending: sort.snapshotKey != "" && pref.Ascending}
}

func (p searchSortPreference) apply(r screenRoute) screenRoute {
	if r.kind != screenSearch || p == (searchSortPreference{}) {
		return r
	}
	u, _ := url.Parse(r.target)
	if q := u.Query(); q.Has("sort") || q.Has("order") {
		return r
	}
	sort, _ := findSearchSort(p.Sort)
	return r.withSearchSort(sort, p.Ascending).withPage(r.page)
}

func (b *browser) rememberSearchSort(sort searchOrdering, ascending bool) {
	pref := searchSortPreference{Sort: sort.urlKey, Ascending: sort.snapshotKey != "" && ascending}
	if pref == b.searchDefault {
		return
	}
	data, err := json.Marshal(pref)
	if err == nil && b.searchSortPath != "" {
		err = writeSettings(b.searchSortPath, data)
	}
	b.searchSortNotice = ""
	if err != nil || b.searchSortPath == "" {
		b.searchSortNotice = "並べ替えを保存できませんでした。"
	}
	b.searchDefault = pref
}
