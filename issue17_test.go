package main

import (
	"context"
	"github.com/egoist/mygo/ui"
	"net/http"
	"testing"
)

func TestThumbnailFallbackAndRetry(t *testing.T) {
	target := "https://nicovideo.cdn.nimg.jp/thumbnails/9/9"
	item := card{link: home + "watch/sm9", title: "作品"}
	s := newThumbnails(func() {})
	if symbol, note := thumbnailFallback(item, s); symbol != "▶" || note != "画像なし" {
		t.Fatal(symbol, note)
	}
	item.thumbnail = target
	s.store(target, nil)
	if _, note := thumbnailFallback(item, s); note != "取得失敗" {
		t.Fatal(note)
	}
	item.masked = true
	if _, note := thumbnailFallback(item, s); note != "マスク表示" {
		t.Fatal(note)
	}
	item.masked = false
	s.wanted = []string{target}
	for _, k := range []string{"a", "b", "c", "d"} {
		s.active[k] = context.CancelFunc(func() {})
	}
	s.retry(target)
	if len(s.active) != thumbnailWorkers || len(s.wanted) != 1 {
		t.Fatal("worker limit or viewport changed")
	}
	if _, ok := s.cache[target]; ok {
		t.Fatal("manual retry retained failed backoff")
	}
	s.store(target, new(ui.Bitmap))
	s.retry(target)
	if s.cache[target].image == nil {
		t.Fatal("successful image retried")
	}
}

func TestMaskedCollectionCover(t *testing.T) {
	c := newCatalog(nil)
	c.http.Transport = requestFunc(func(*http.Request) (*http.Response, error) {
		return testResponse(200, `{"meta":{"status":200},"data":{"mylists":[{"id":1,"isPublic":true,"name":"表紙","sampleItems":[{"video":{"requireSensitiveMasking":true,"thumbnail":{"url":"https://nicovideo.cdn.nimg.jp/thumbnails/9/9"}}}]}]}}`), nil
	})
	p, err := c.load(t.Context(), routeFor(home+"user/123/mylist"))
	if err != nil || len(p.items) != 1 || !p.items[0].masked || p.items[0].thumbnail != "" {
		t.Fatal(p, err)
	}
	if symbol, note := thumbnailFallback(p.items[0], nil); symbol != "▤" || note != "マスク表示" {
		t.Fatal(symbol, note)
	}
}
