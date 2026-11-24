package main

import "testing"

func TestSearch(t *testing.T) {
	for _, c := range []struct {
		in   string
		tag  bool
		want string
	}{
		{"猫 動画", false, home + "search/%E7%8C%AB%20%E5%8B%95%E7%94%BB"},
		{" 初音ミク ", true, home + "tag/%E5%88%9D%E9%9F%B3%E3%83%9F%E3%82%AF"},
		{"C++ / #", true, home + "tag/C%2B%2B%20%2F%20%23"},
		{"SM9", false, home + "watch/sm9"}, {"SM9", true, home + "tag/SM9"},
		{"user/12/video", false, home + "user/12/video"}, {"nico.ms/sm9", false, "https://nico.ms/sm9"},
		{"https://www.nicovideo.jp/watch/sm9", true, ""}, {"..", true, ""}, {"a\nb", false, ""},
	} {
		if got := resolve(c.in, c.tag); got != c.want {
			t.Errorf("resolve(%q, %v) = %q; want %q", c.in, c.tag, got, c.want)
		}
	}
}

func TestNavigationAllowList(t *testing.T) {
	for _, raw := range []string{home, login, "https://www.nicovideo.jp:443/watch/sm9", "https://nico.ms/sm9"} {
		if allowedURL(raw) == nil {
			t.Errorf("rejected %q", raw)
		}
	}
	for _, raw := range []string{"http://www.nicovideo.jp/", "//www.nicovideo.jp/", "https://www.nicovideo.jp.evil.test/",
		"https://www.nicovideo.jp@evil.test/", "https://evil.test@www.nicovideo.jp/", "https://www.nicovideo.jp:8443/",
		"https://www.nicovideo.jp:/", "https://www.nicovideo.jp\\@evil.test/", "javascript:alert(1)", "file:///tmp/x", "ms-settings:"} {
		if allowedURL(raw) != nil || resolve(raw, false) != "" {
			t.Errorf("accepted %q", raw)
		}
	}
}

func TestCanonical(t *testing.T) {
	if u, k := canonical(home + "tag/C%2B%2B?page=4&ref=tracking&sort=f&order=d#x"); u != home+"tag/C%2B%2B?order=d&sort=f" || k != "タグ検索" {
		t.Errorf("%s %s", u, k)
	}
	if u, k := canonical("https://nico.ms/SM9?ref=abc"); u != home+"watch/sm9" || k != "動画" {
		t.Errorf("%s %s", u, k)
	}
	if u, _ := canonical(login + "?token=test"); u != "" {
		t.Fatal("account route saved")
	}
}

func TestCopyUsesCLIPaths(t *testing.T) {
	for _, c := range []struct{ raw, want string }{
		{home + "user/123/video/?ref=x#top", home + "user/123"},
		{home + "user/123", home + "user/123"},
		{home + "mylist/99?ref=x", home + "mylist/99"},
		{home + "user/123/mylist/99?ref=x", home + "mylist/99"},
		{mylists + "/?ref=x#/123456789012", home + "mylist/123456789012"},
		{mylists + "/#/99/1", ""}, {mylists, ""}, {home + "user/123/mylist", ""},
		{home + "user/123/series/38", ""}, {home + "watch/sm9", ""},
		{home + "user/1234567890/video", ""}, {home + "mylist/1234567890123", ""},
		{home + "mylist/１２３", ""}, {home + "mylist/%31", ""}, {home + "search/test", ""},
		{"https://account.nicovideo.jp/user/123", ""}, {"https://www.nicovideo.jp.evil.test/user/123", ""},
	} {
		if got := goNicoListURL(c.raw); got != c.want {
			t.Errorf("%q = %q, want %q", c.raw, got, c.want)
		}
	}
}
