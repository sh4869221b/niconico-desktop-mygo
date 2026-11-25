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
	modes := map[string]string{
		"f": "registeredAt", "v": "viewCount", "r": "lastCommentTime",
		"m": "mylistCount", "n": "commentCount", "l": "duration",
	}
	sortKey, explicit := modes[u.Query().Get("sort")]
	order := "desc"
	if !explicit {
		sortKey = "registeredAt"
	} else if u.Query().Get("order") == "a" {
		order = "asc"
	}
	return url.Values{key: {r.key}, "pageSize": {strconv.Itoa(pageSize)},
		"page": {strconv.Itoa(r.page + 1)}, "sortKey": {sortKey}, "sortOrder": {order}}
}

func (c catalog) accountSearch(ctx context.Context, r screenRoute) (catalogPage, error) {
	p := catalogPage{title: r.key}
	data, err := c.request(ctx, apiHost, accountSearchPath, accountSearchParameters(r), true)
	if err != nil {
		return p, err
	}
	result, err := decodeJSON[struct {
		Meta struct{ Status int }
		Data struct {
			TotalCount int
			HasNext    *bool
			Items      []apiVideo
		}
	}](data)
	if err != nil {
		return p, err
	}
	if result.Meta.Status == 401 {
		return p, errSignIn
	}
	if result.Meta.Status != 200 || result.Data.Items == nil {
		return p, errors.New("ログイン検索の結果を取得できませんでした")
	}
	p.searchMode = "ログイン検索"
	p.more = hasMore(result.Data.HasNext, r.page, result.Data.TotalCount)
	for _, v := range result.Data.Items {
		if item, ok := v.card(); ok {
			p.items = append(p.items, item)
		}
	}
	return p, nil
}
