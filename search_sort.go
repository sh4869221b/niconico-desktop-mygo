package main

import "net/url"

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
