package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const seriesVideoPageSize = 100

type seriesResponse struct {
	Meta struct{ Status int }
	Data struct {
		TotalCount *int
		HasNext    *bool
		Detail     struct {
			Title, Description string
			Owner              publisherMetadata
		}
		Items []struct {
			ID                  json.RawMessage
			Title, ThumbnailURL string
			IsPublic            *bool
			Video               apiVideo
		}
	}
}

func (c catalog) seriesResponse(ctx context.Context, path string, query url.Values, signedIn bool) (seriesResponse, error) {
	data, err := c.request(ctx, apiHost, path, query, signedIn)
	if err != nil {
		return seriesResponse{}, err
	}
	response, err := decodeJSON[seriesResponse](data)
	if err != nil {
		return response, err
	}
	if response.Meta.Status != 0 && response.Meta.Status != http.StatusOK {
		return response, catalogHTTPStatus(response.Meta.Status)
	}
	if response.Meta.Status == 0 || response.Data.Items == nil {
		return response, errors.New("シリーズの応答形式を読み取れませんでした。")
	}
	return response, nil
}

func seriesDenied(err error) bool {
	return errors.Is(err, catalogHTTPStatus(http.StatusUnauthorized)) || errors.Is(err, catalogHTTPStatus(http.StatusForbidden))
}

func (c catalog) series(ctx context.Context, r screenRoute) (catalogPage, error) {
	directory := r.kind == screenUserSeries
	page := catalogPage{title: "シリーズ", emptyNotice: "表示できるシリーズ動画はありません。"}
	path := "/v2/series/" + r.key
	requestSize := seriesVideoPageSize
	if directory {
		path = "/v1/users/" + r.key + "/series"
		page.title, page.emptyNotice = "シリーズ一覧", "公開されているシリーズはありません。"
		requestSize = pageSize
	}
	query := url.Values{"page": {strconv.Itoa(r.page + 1)}, "pageSize": {strconv.Itoa(requestSize)}}
	result, err := c.seriesResponse(ctx, path, query, false)
	if seriesDenied(err) && c.session != nil && ctx.Err() == nil {
		result, err = c.seriesResponse(ctx, path, query, true)
	}
	if seriesDenied(err) || errors.Is(err, errSignIn) {
		return page, errors.New("シリーズを閲覧できません。公開設定やログイン状態を確認してください。")
	}
	if errors.Is(err, catalogHTTPStatus(http.StatusNotFound)) {
		return page, errors.New("シリーズ情報が見つかりません。非公開または削除されている場合があります。")
	}
	if err != nil {
		return page, err
	}
	data := result.Data
	if data.HasNext != nil {
		page.more = *data.HasNext
	} else if data.TotalCount != nil {
		page.more = hasMore(nil, r.page, *data.TotalCount, requestSize)
	} else {
		// Hidden or unavailable rows still occupy positions in the server page.
		page.more = len(data.Items) >= requestSize
	}
	if directory {
		hasPrivate := false
		for _, entry := range data.Items {
			id := metadataID(entry.ID)
			if entry.IsPublic != nil && !*entry.IsPublic {
				hasPrivate = true
				continue
			}
			if !numeric(id, 12) {
				continue
			}
			title := strings.TrimSpace(entry.Title)
			if title == "" {
				title = fmt.Sprintf("シリーズ %s", id)
			}
			link := home + "user/" + r.key + "/series/" + id
			page.items = append(page.items, card{link: link, title: title, thumbnail: normalizeThumbnailURL(entry.ThumbnailURL)})
		}
		if hasPrivate && len(page.items) == 0 {
			page.emptyNotice = "この投稿者のシリーズは非公開です。"
		}
		return page, nil
	}
	if title := strings.TrimSpace(data.Detail.Title); title != "" {
		page.title = title
	}
	page.summary, page.links, page.description = plainDescription(data.Detail.Description)
	page.publisher = data.Detail.Owner.publisher("user")
	page.author = page.publisher.name
	for _, entry := range data.Items {
		if item, ok := entry.Video.card(); ok {
			page.items = append(page.items, item)
		} else {
			page.items = append(page.items, unavailableVideo())
		}
	}
	return page, nil
}
