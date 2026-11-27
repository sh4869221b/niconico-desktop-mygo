package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const apiHost = "nvapi.nicovideo.jp"
const pageSize = 24
const userVideoPageSize = 100
const publicMylistPageSize = 100
const maxCatalogPage = 4166 // zero based, preserves the accepted range of existing browse links
const responseLimit = 4 << 20

var errSignIn = errors.New("ニコニコへのログインが必要です")
var nicoLink = regexp.MustCompile(`https://[a-zA-Z0-9.]+/[^\s<>"]+`)
var watchSegment = regexp.MustCompile(`^(?:(?:sm|nm|so)?[0-9]{1,20})$`)

type screenKind uint8

const (
	screenOfficial screenKind = iota
	screenHome
	screenSearch
	screenUser
	screenUserLists
	screenList
	screenOwnLists
	screenOwnList
	screenVideo
)

type screenRoute struct {
	kind           screenKind
	link           string // canonical link used for identity and API parameters
	target         string // URL opened by the player or the official page button
	key            string
	page           int  // zero based
	officialBrowse bool // recognized browse page whose conditions require the official UI
}

func routeFor(raw string) screenRoute {
	r := screenRoute{target: raw}
	input := allowedURL(raw)
	if input == nil {
		return r
	}
	if siteHost(input) && strings.Trim(input.Path, "/") == "" {
		r.kind, r.link = screenHome, home
		return r
	}
	link, kind := canonical(raw)
	if link == mylists {
		r.kind, r.link = screenOwnLists, link
		return r
	}
	if id, ok := strings.CutPrefix(link, mylists+"#/"); ok {
		r.kind, r.link, r.key = screenOwnList, link, id
		return r
	}
	if link == "" {
		return r
	}
	u, _ := url.Parse(link)
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	r.link, r.key = link, parts[len(parts)-1]
	switch kind {
	case "タグ検索", "キーワード検索":
		page, ok := nativeSearchPage(input)
		if !ok {
			r.officialBrowse = true
			return r
		}
		_, term, _ := strings.Cut(strings.TrimLeft(u.EscapedPath(), "/"), "/")
		r.kind, r.page = screenSearch, page
		r.key, _ = url.PathUnescape(term)
	case "動画":
		if watchSegment.MatchString(r.key) {
			r.kind = screenVideo
			// Use the desktop watch page for mobile and short links too.
			u.RawQuery, u.ForceQuery = input.RawQuery, input.ForceQuery
			u.Fragment, u.RawFragment = input.Fragment, input.RawFragment
			r.target = u.String()
		}
	case "ユーザー":
		if numeric(parts[1], 9) {
			page, ok := nativeUserVideoPage(input)
			if !ok {
				r.officialBrowse = true
				return r
			}
			r.kind, r.key, r.page = screenUser, parts[1], page
		}
	case "マイリスト":
		if len(parts) == 3 && parts[0] == "user" && numeric(parts[1], 9) {
			r.kind, r.key = screenUserLists, parts[1]
		} else if goNicoListURL(link) != "" && numeric(r.key, 12) {
			page, ok := desktopListPage(input)
			if !ok {
				r.officialBrowse = true
				return r
			}
			r.kind, r.page = screenList, page
		}
	}
	return r
}

func pageIndex(value string) (int, bool) {
	page, err := strconv.Atoi(value)
	return page - 1, numeric(value, len(value)) && err == nil && page > 0 && page <= maxCatalogPage+1
}

func nativeUserVideoPage(u *url.URL) (int, bool) {
	if u.RawQuery != "" && !strings.HasSuffix(strings.TrimRight(u.Path, "/"), "/video") {
		return 0, false
	}
	return desktopListPage(u)
}

func desktopListPage(u *url.URL) (int, bool) {
	if u.RawQuery == "" {
		return 0, true
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(query) != 1 || len(query["page"]) != 1 ||
		!strings.EqualFold(u.Hostname(), "www.nicovideo.jp") {
		return 0, false
	}
	return pageIndex(query.Get("page"))
}

func nativeSearchPage(u *url.URL) (int, bool) {
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return 0, false
	}
	page := 0
	for key, values := range query {
		if len(values) != 1 {
			return 0, false
		}
		value := values[0]
		switch key {
		case "page":
			var ok bool
			if page, ok = pageIndex(value); !ok {
				return 0, false
			}
		case "sort":
			if _, ok := findSearchSort(value); !ok || value == "" {
				return 0, false
			}
		case "order":
			if value != "a" && value != "d" {
				return 0, false
			}
		default:
			return 0, false
		}
	}
	return page, true
}

func (r screenRoute) withPage(page int) screenRoute {
	r.page = page
	if r.kind == screenSearch || r.kind == screenUser || r.kind == screenList {
		u, _ := url.Parse(r.target)
		if r.kind == screenUser || r.kind == screenList {
			u.Host = "www.nicovideo.jp"
		}
		if r.kind == screenUser {
			u.Path, u.RawPath = "/user/"+r.key+"/video", ""
		}
		query := u.Query()
		if page == 0 {
			query.Del("page")
		} else {
			query.Set("page", strconv.Itoa(page+1))
		}
		u.RawQuery = query.Encode()
		r.target = u.String()
	}
	return r
}

func (r screenRoute) startTime() time.Duration {
	u, _ := url.Parse(r.target)
	seconds, err := strconv.ParseUint(u.Query().Get("from"), 10, 64)
	if err != nil || seconds > uint64((1<<63-1)/time.Second) {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

type card struct {
	link, title, summary, owner, thumbnail string
}

type catalogPage struct {
	title, summary, author string
	searchMode             string
	items, links           []card
	more                   bool
}

type catalog struct {
	http *http.Client
	// Reads a matching cookie from this app's profile only, on approved account API routes.
	session func(target string) (*http.Cookie, error)
}

func newCatalog(session func(string) (*http.Cookie, error)) catalog {
	return catalog{session: session, http: &http.Client{
		Timeout: 12 * time.Second,
		// Do not forward credentials, even on a redirect to another Niconico host.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func sessionEndpoint(u *url.URL) bool {
	if u.Scheme != "https" || u.Host != apiHost || u.User != nil {
		return false
	}
	if u.Path == accountSearchPath {
		return true
	}
	const base = "/v1/users/me/mylists"
	id, ok := strings.CutPrefix(u.Path, base+"/")
	return u.Path == base || ok && numeric(id, 12)
}

func (c catalog) request(ctx context.Context, host, path string, q url.Values, private bool) ([]byte, error) {
	switch host {
	case apiHost, "snapshot.search.nicovideo.jp", "ext.nicovideo.jp":
	default:
		return nil, errors.New("未対応の接続先です")
	}
	target := url.URL{Scheme: "https", Host: host, Path: path, RawQuery: q.Encode()}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, errors.New("リクエストを作成できません")
	}
	req.Header.Set("User-Agent", "NiconicoDesktopMyGo/native-ui")
	if host == apiHost {
		req.Header.Set("X-Frontend-Id", "6")
		req.Header.Set("X-Frontend-Version", "0")
		req.Header.Set("Accept", "application/json")
	}
	if private {
		if !sessionEndpoint(&target) || c.session == nil {
			return nil, errSignIn
		}
		cookie, err := c.session(target.String())
		if err != nil || cookie == nil || cookie.Name != "user_session" || !cookie.Secure ||
			cookie.Value == "" || cookie.Valid() != nil {
			return nil, errSignIn
		}
		req.AddCookie(cookie)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, errors.New("通信に失敗しました")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		if private && (res.StatusCode == 401 || res.StatusCode == 403) {
			return nil, errSignIn
		}
		return nil, fmt.Errorf("ニコニコから取得できませんでした（HTTP %d）", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, responseLimit+1))
	if err != nil || len(data) > responseLimit {
		return nil, errors.New("応答を読み取れませんでした")
	}
	return data, nil
}

func decodeJSON[T any](b []byte) (T, error) {
	var result T
	if err := json.Unmarshal(b, &result); err != nil {
		return result, errors.New("未対応の応答形式です")
	}
	return result, nil
}

func makeVideo(id, title string, duration, views, comments int) (card, bool) {
	if !watchSegment.MatchString(id) {
		return card{}, false
	}
	item := card{link: home + "watch/" + id, title: strings.TrimSpace(title)}
	if item.title == "" {
		item.title = id
	}
	item.summary = fmt.Sprintf("%d:%02d  ·  再生 %d  ·  コメント %d", duration/60, duration%60, views, comments)
	return item, true
}

func searchParameters(r screenRoute) url.Values {
	u, _ := url.Parse(r.link)
	targets := "title,description,tags"
	if strings.HasPrefix(u.Path, "/tag/") {
		targets = "tagsExact"
	}
	sort, ascending := r.searchSort()
	direction := "-"
	if ascending {
		direction = "+"
	}
	return url.Values{"q": {r.key}, "targets": {targets},
		"fields": {"contentId,title,lengthSeconds,viewCounter,commentCounter,thumbnailUrl"},
		"_sort":  {direction + sort.snapshotKey}, "_limit": {strconv.Itoa(searchPageSize)}, "_offset": {strconv.Itoa(r.page * searchPageSize)},
		"_context": {"NiconicoDesktopMyGo"}}
}

func (c catalog) search(ctx context.Context, r screenRoute) (catalogPage, error) {
	if c.session != nil {
		result, err := c.accountSearch(ctx, r, true)
		if !errors.Is(err, errSignIn) {
			return result, err
		}
	}
	result, err := c.accountSearch(ctx, r, false)
	sort, _ := r.searchSort()
	if err != nil && ctx.Err() == nil && sort.snapshotKey != "" && r.page*searchPageSize < snapshotResultLimit {
		return c.publicSearch(ctx, r)
	}
	return result, err
}

func (c catalog) publicSearch(ctx context.Context, r screenRoute) (catalogPage, error) {
	data, err := c.request(ctx, "snapshot.search.nicovideo.jp", "/api/v2/snapshot/video/contents/search", searchParameters(r), false)
	p := catalogPage{title: r.key, searchMode: "スナップショット検索"}
	if err != nil {
		return p, err
	}
	type row struct {
		ContentID, Title, ThumbnailURL             string
		LengthSeconds, ViewCounter, CommentCounter int
	}
	result, err := decodeJSON[struct {
		Meta struct{ Status, TotalCount int }
		Data []row
	}](data)
	if err != nil || result.Meta.Status != 200 || result.Data == nil {
		return p, errors.New("検索結果を取得できませんでした")
	}
	p.more = (r.page+1)*searchPageSize < min(result.Meta.TotalCount, snapshotResultLimit)
	for _, item := range result.Data {
		if v, ok := makeVideo(item.ContentID, item.Title, item.LengthSeconds, item.ViewCounter, item.CommentCounter); ok {
			v.thumbnail = normalizeThumbnailURL(item.ThumbnailURL)
			p.items = append(p.items, v)
		}
	}
	return p, nil
}

type apiVideo struct {
	ID, Title               string
	Duration                int
	Count                   struct{ View, Comment int }
	Owner                   struct{ ID, Name, OwnerType string }
	Thumbnail               struct{ URL, MiddleURL, ListingURL string }
	RequireSensitiveMasking bool
}

func (v apiVideo) thumbnailURL() string {
	if !v.RequireSensitiveMasking {
		for _, raw := range []string{v.Thumbnail.ListingURL, v.Thumbnail.MiddleURL, v.Thumbnail.URL} {
			if target := normalizeThumbnailURL(raw); target != "" {
				return target
			}
		}
	}
	return ""
}

func (v apiVideo) card() (card, bool) {
	x, ok := makeVideo(strings.ToLower(v.ID), v.Title, v.Duration, v.Count.View, v.Count.Comment)
	x.thumbnail = v.thumbnailURL()
	if numeric(v.Owner.ID, 9) && v.Owner.OwnerType == "user" {
		x.owner = home + "user/" + v.Owner.ID + "/video"
	}
	return x, ok
}

func (c catalog) list(ctx context.Context, r screenRoute) (catalogPage, error) {
	private := r.kind == screenOwnList || r.kind == screenOwnLists
	directory := r.kind == screenUserLists || r.kind == screenOwnLists
	var path, title string
	switch r.kind {
	case screenUser:
		path, title = "/v3/users/"+r.key+"/videos", "投稿動画"
	case screenUserLists:
		path, title = "/v1/users/"+r.key+"/mylists", "公開マイリスト"
	case screenOwnLists:
		path, title = "/v1/users/me/mylists", "自分のマイリスト"
	case screenOwnList:
		path, title = "/v1/users/me/mylists/"+r.key, "マイリスト"
	default:
		path, title = "/v2/mylists/"+r.key, "マイリスト"
	}
	p := catalogPage{title: title}
	limit := pageSize
	switch r.kind {
	case screenUser:
		limit = userVideoPageSize
	case screenList:
		limit = publicMylistPageSize
	}
	query := url.Values{
		"page": {strconv.Itoa(r.page + 1)}, "pageSize": {strconv.Itoa(limit)},
	}
	if r.kind == screenUser {
		// Match the desktop posting list's default page size and ordering.
		query.Set("q", "")
		query.Set("sortKey", "registeredAt")
		query.Set("sortOrder", "desc")
		query.Set("_language", "ja-jp")
		query.Set("sensitiveContents", "mask")
		query.Set("selectContentType", "long")
	} else if r.kind == screenList {
		// Omitting sortKey and sortOrder uses the list owner's default, as in the official page.
		query.Set("sensitiveContents", "mask")
	} else if directory {
		query.Set("sampleItemCount", "1")
		query.Set("sensitiveContents", "mask")
	}
	data, err := c.request(ctx, apiHost, path, query, private)
	if err != nil {
		return p, err
	}
	type row struct {
		WatchID          string
		Essential, Video apiVideo
	}
	result, err := decodeJSON[struct {
		Meta struct{ Status int }
		Data struct {
			TotalCount int
			HasNext    *bool
			Items      []row
			Mylists    []struct {
				ID          int64
				Name        string
				IsPublic    bool
				ItemsCount  int
				SampleItems []row
			}
			Mylist *struct {
				Name, Description string
				TotalItemCount    int
				HasNext           *bool
				Items             []row
			}
		}
	}](data)
	if err != nil || result.Meta.Status != 200 {
		return p, errors.New("一覧を取得できませんでした")
	}
	if directory {
		if result.Data.Mylists == nil {
			return p, errors.New("マイリスト一覧の応答形式が変わっています")
		}
		p.more = hasMore(result.Data.HasNext, r.page, result.Data.TotalCount, limit)
		for _, list := range result.Data.Mylists {
			id := strconv.FormatInt(list.ID, 10)
			if !numeric(id, 12) || !private && !list.IsPublic {
				continue
			}
			link := home + "mylist/" + id
			if private {
				link = mylists + "#/" + id
			}
			label := "公開"
			if !list.IsPublic {
				label = "非公開"
			}
			item := card{link: link, title: list.Name, summary: fmt.Sprintf("%s  ·  %d 件", label, list.ItemsCount)}
			for _, sample := range list.SampleItems {
				if item.thumbnail = sample.Video.thumbnailURL(); item.thumbnail != "" {
					break
				}
			}
			p.items = append(p.items, item)
		}
		return p, nil
	}
	rows := result.Data.Items
	if r.kind == screenUser {
		p.more = hasMore(result.Data.HasNext, r.page, result.Data.TotalCount, limit)
	} else if m := result.Data.Mylist; m != nil {
		if m.Name != "" {
			p.title = m.Name
		}
		p.summary, p.links = plainDescription(m.Description)
		p.more, rows = hasMore(m.HasNext, r.page, m.TotalItemCount, limit), m.Items
	} else {
		return p, errors.New("マイリストの応答形式が変わっています")
	}
	for _, item := range rows {
		v := item.Video
		if r.kind == screenUser {
			v = item.Essential
		}
		if v.ID == "" && v.Title != "" {
			v.ID = item.WatchID
		}
		if x, ok := v.card(); ok {
			if r.kind == screenUser && p.title == title && v.Owner.Name != "" {
				p.title = v.Owner.Name + " の投稿動画"
			}
			p.items = append(p.items, x)
		}
	}
	return p, nil
}

func hasMore(flag *bool, page, total, pageSize int) bool {
	if flag != nil {
		return *flag
	}
	return (page+1)*pageSize < total
}

func (c catalog) video(ctx context.Context, r screenRoute) (catalogPage, error) {
	p := catalogPage{title: r.key}
	data, err := c.request(ctx, "ext.nicovideo.jp", "/api/getthumbinfo/"+r.key, nil, false)
	if err != nil {
		return p, err
	}
	var result struct {
		Status string `xml:"status,attr"`
		Thumb  struct {
			ID           string `xml:"video_id"`
			Title        string `xml:"title"`
			Description  string `xml:"description"`
			UserID       string `xml:"user_id"`
			UserNickname string `xml:"user_nickname"`
		} `xml:"thumb"`
	}
	if err := xml.Unmarshal(data, &result); err != nil || result.Status != "ok" || result.Thumb.Title == "" {
		return p, errors.New("公開されている動画情報を取得できませんでした")
	}
	if id := result.Thumb.ID; id != "" && videoID.MatchString(r.key) && !strings.EqualFold(id, r.key) {
		return p, errors.New("指定した動画と異なる動画情報を受け取りました")
	}
	p.title = result.Thumb.Title
	p.summary, p.links = plainDescription(result.Thumb.Description)
	if numeric(result.Thumb.UserID, 9) {
		p.author = result.Thumb.UserNickname
		p.links = append([]card{{link: home + "user/" + result.Thumb.UserID + "/video", title: "投稿者の動画一覧"}, {link: home + "user/" + result.Thumb.UserID + "/mylist", title: "投稿者の公開マイリスト"}}, p.links...)
	}
	return p, nil
}

func plainDescription(value string) (string, []card) {
	var text strings.Builder
	var links []card
	seen := map[string]bool{}
	add := func(raw string) {
		raw = strings.TrimRight(raw, "。．、！？：；,.)]}!?;）】」")
		link, kind := canonical(raw)
		if len(links) >= 24 || (kind != "動画" && kind != "マイリスト" && kind != "ユーザー") {
			return
		}
		r := routeFor(raw)
		if r.kind == screenVideo {
			u, _ := url.Parse(r.target)
			if from := u.Query().Get("from"); from != "" {
				link += "?" + url.Values{"from": {from}}.Encode()
			}
		}
		if seen[link] {
			return
		}
		seen[link] = true
		links = append(links, card{link: r.target, title: kind + " · " + strings.TrimPrefix(link, home)})
	}
	z := html.NewTokenizer(strings.NewReader(value))
	var ignored string
	for {
		switch z.Next() {
		case html.ErrorToken:
			return strings.TrimSpace(text.String()), links
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			if ignored != "" {
				continue
			}
			if t.Data == "script" || t.Data == "style" {
				ignored = t.Data
				continue
			}
			if t.Data == "br" || t.Data == "p" || t.Data == "div" {
				text.WriteByte('\n')
			}
			if t.Data == "a" {
				for _, a := range t.Attr {
					if a.Key == "href" {
						link := a.Val
						if strings.HasPrefix(link, "/") && !strings.HasPrefix(link, "//") {
							link = strings.TrimSuffix(home, "/") + link
						}
						add(link)
					}
				}
			}
		case html.EndTagToken:
			if t := z.Token(); t.Data == ignored {
				ignored = ""
			}
		case html.TextToken:
			if ignored != "" {
				continue
			}
			s := string(z.Text())
			text.WriteString(s)
			for _, link := range nicoLink.FindAllString(s, 24) {
				add(link)
			}
		}
	}
}

func (c catalog) load(ctx context.Context, r screenRoute) (catalogPage, error) {
	switch r.kind {
	case screenSearch:
		return c.search(ctx, r)
	case screenUser, screenUserLists, screenList, screenOwnLists, screenOwnList:
		return c.list(ctx, r)
	case screenVideo:
		return c.video(ctx, r)
	default:
		return catalogPage{}, nil
	}
}
