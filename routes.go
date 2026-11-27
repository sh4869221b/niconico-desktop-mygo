package main

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const home = "https://www.nicovideo.jp/"
const mylists = home + "my/mylist"
const login = "https://account.nicovideo.jp/login"

var videoID = regexp.MustCompile("(?i)^(sm|nm|so)[0-9]+$")
var shortPath = regexp.MustCompile("(?i)^/?(watch/((sm|nm|so)?[0-9]+)|mylist/[0-9]+|user/[0-9]+(/video|/mylist(/[0-9]+)?)?)$")
var schemePrefix = regexp.MustCompile("(?i)^[a-z][a-z0-9+.-]*:")
var pathPunctuation = strings.NewReplacer("+", "%2B", ":", "%3A", "@", "%40", "=", "%3D", "$", "%24", "&", "%26")
var sortKey = regexp.MustCompile("^[a-z]{1,4}$")

func allowedURL(raw string) *url.URL {
	if len(raw) > 8192 || strings.Contains(raw, "\\") || strings.ContainsFunc(raw, unicode.IsControl) {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Opaque != "" || u.User != nil {
		return nil
	}
	// Allow the explicit default port, but never an empty or non-default port.
	host := strings.ToLower(strings.TrimSuffix(u.Host, ":443"))
	switch host {
	case "nicovideo.jp", "www.nicovideo.jp", "sp.nicovideo.jp", "video.nicovideo.jp", "ch.nicovideo.jp", "account.nicovideo.jp", "nico.ms":
		return u
	}
	return nil
}

func siteHost(u *url.URL) bool {
	switch strings.ToLower(u.Hostname()) {
	case "nicovideo.jp", "www.nicovideo.jp", "sp.nicovideo.jp", "video.nicovideo.jp":
		return true
	}
	return false
}

func searchURL(mode, query string) string {
	return exactSearchURL(mode, strings.TrimSpace(query))
}

func exactSearchURL(mode, query string) string {
	if strings.TrimSpace(query) == "" || query == "." || query == ".." || utf8.RuneCountInString(query) > 500 ||
		strings.ContainsFunc(query, unicode.IsControl) {
		return ""
	}
	return home + mode + "/" + pathPunctuation.Replace(url.PathEscape(query))
}

func resolve(input string, byTag bool) string {
	input = strings.TrimSpace(input)
	if input == "" || utf8.RuneCountInString(input) > 500 || strings.HasPrefix(input, "//") ||
		strings.Contains(input, "://") && (byTag || allowedURL(input) == nil) {
		return ""
	}
	if byTag {
		return searchURL("tag", input)
	}
	if u := allowedURL(input); u != nil {
		return u.String()
	}
	lower := strings.ToLower(input)
	for _, prefix := range []string{"nico.ms/", "nicovideo.jp/", "www.nicovideo.jp/", "sp.nicovideo.jp/"} {
		if strings.HasPrefix(lower, prefix) {
			return resolve("https://"+input, false)
		}
	}
	if schemePrefix.MatchString(input) {
		return ""
	}
	if videoID.MatchString(input) {
		return home + "watch/" + lower
	}
	if shortPath.MatchString(input) {
		return home + strings.TrimPrefix(lower, "/")
	}
	return searchURL("search", input)
}

func canonical(raw string) (string, string) {
	u := allowedURL(raw)
	if u == nil {
		return "", ""
	}
	p := strings.Trim(u.EscapedPath(), "/")
	if strings.EqualFold(u.Hostname(), "nico.ms") {
		if videoID.MatchString(p) {
			return home + "watch/" + strings.ToLower(p), "動画"
		}
		return "", ""
	}
	if !siteHost(u) {
		return "", ""
	}
	if mode, encoded, ok := strings.Cut(p, "/"); ok && (mode == "search" || mode == "tag") && !strings.Contains(encoded, "/") {
		term, err := url.PathUnescape(encoded)
		if err != nil {
			return "", ""
		}
		link := exactSearchURL(mode, term)
		if link == "" {
			return "", ""
		}
		opts := url.Values{}
		val := u.Query().Get("sort")
		if _, known := findSearchSort(val); val != "" && (known || sortKey.MatchString(val)) {
			opts.Set("sort", val)
		}
		if val := u.Query().Get("order"); val == "a" || val == "d" {
			opts.Set("order", val)
		}
		if q := opts.Encode(); q != "" {
			link += "?" + q
		}
		if mode == "tag" {
			return link, "タグ検索"
		}
		return link, "キーワード検索"
	}
	if p == "my/mylist" {
		if id, ok := strings.CutPrefix(u.EscapedFragment(), "/"); ok && numeric(id, 12) {
			return mylists + "#/" + id, "マイリスト"
		}
		return mylists, "マイリスト"
	}
	if !shortPath.MatchString(p) {
		return "", ""
	}
	p = strings.ToLower(p)
	switch {
	case strings.HasPrefix(p, "watch/"):
		return home + p, "動画"
	case strings.Contains(p, "mylist"):
		return home + p, "マイリスト"
	default:
		return home + p, "ユーザー"
	}
}

func numeric(value string, max int) bool {
	return len(value) > 0 && len(value) <= max && strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) < 0
}

func goNicoListURL(raw string) string {
	u := allowedURL(raw)
	if u == nil || !siteHost(u) {
		return ""
	}
	p := strings.Split(strings.TrimRight(u.EscapedPath(), "/"), "/")
	switch {
	case len(p) == 3 && p[1] == "my" && p[2] == "mylist":
		if id, yes := strings.CutPrefix(u.EscapedFragment(), "/"); yes && numeric(id, 12) {
			return home + "mylist/" + id
		}
	case len(p) == 3 && p[1] == "mylist":
		if numeric(p[2], 12) {
			return home + "mylist/" + p[2]
		}
	case (len(p) == 3 || (len(p) == 4 && p[3] == "video")) && p[1] == "user":
		if numeric(p[2], 9) {
			return home + "user/" + p[2]
		}
	case len(p) == 5 && p[1] == "user" && p[3] == "mylist":
		if numeric(p[2], 9) && numeric(p[4], 12) {
			return home + "mylist/" + p[4]
		}
	}
	return ""
}
