package main

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

const accountSearchPath = "/v2/search/video"

func accountSearchParameters(r screenRoute) url.Values {
	u, _ := url.Parse(r.link)
	key := "keyword"
	if strings.HasPrefix(u.Path, "/tag/") {
		key = "tag"
	}
	sort, ascending := r.searchSort()
	order := "desc"
	if sort.snapshotKey == "" {
		order = "none"
	} else if ascending {
		order = "asc"
	}
	return url.Values{
		key: {r.key}, "pageSize": {strconv.Itoa(searchPageSize)}, "page": {strconv.Itoa(r.page + 1)},
		"sortKey": {sort.apiKey}, "sortOrder": {order}, "channelVideoListingStatus": {"included"},
		"selectContentType": {"long"}, "sensitiveContents": {"mask"}, "_language": {"ja-jp"},
	}
}

func (c catalog) accountSearch(ctx context.Context, r screenRoute, private bool) (catalogPage, error) {
	p := catalogPage{title: r.key}
	data, err := c.request(ctx, apiHost, accountSearchPath, accountSearchParameters(r), private)
	if err != nil {
		return p, err
	}
	result, err := decodeJSON[struct {
		Meta struct{ Status int }
		Data struct {
			TotalCount *int
			HasNext    *bool
			Items      []apiVideo
		}
	}](data)
	if err != nil {
		return p, err
	}
	if private && result.Meta.Status == 403 {
		return p, errAccessDenied
	}
	if private && result.Meta.Status == 401 {
		return p, errSignIn
	}
	if result.Meta.Status != 200 || result.Data.Items == nil {
		return p, errors.New("検索結果を取得できませんでした")
	}
	p.searchMode = "公開検索"
	if private {
		p.searchMode = "ログイン検索"
	}
	p.total, p.rawCount = result.Data.TotalCount, len(result.Data.Items)
	p.more = r.page < p.lastSearchPage() && (p.total != nil || p.rawCount == searchPageSize)
	if result.Data.HasNext != nil {
		p.more = *result.Data.HasNext && r.page < p.lastSearchPage()
	}
	for _, v := range result.Data.Items {
		if item, ok := v.card(); ok {
			p.items = append(p.items, item)
		} else {
			p.items = append(p.items, unavailableVideo())
		}
	}
	return p, nil
}
