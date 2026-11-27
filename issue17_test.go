package main

import (
	"context"
	"github.com/egoist/mygo/ui"
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
