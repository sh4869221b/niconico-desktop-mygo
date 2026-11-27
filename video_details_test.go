package main

import (
	"github.com/egoist/mygo/ui"
	"net/http"
	"testing"
)

func TestVideoDetailsShowPublicIdentification(t *testing.T) {
	c := newCatalog(nil)
	c.http.Transport = requestFunc(func(*http.Request) (*http.Response, error) {
		return testResponse(200, `<nicovideo_thumb_response status="ok"><thumb><video_id>sm9</video_id><title>作品</title><length>2:30</length><first_retrieve>2020-01-02T03:04:05+09:00</first_retrieve><view_counter>123</view_counter><comment_num>0</comment_num><ch_id>123</ch_id><ch_name>公式名</ch_name></thumb></nicovideo_thumb_response>`), nil
	})
	r := routeFor(home + "watch/sm9")
	page, err := c.video(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	b := &browser{navigation: []screenRoute{r}, result: page, closed: true}
	v := ui.NewTester(b.draw, 1000, 700)
	for _, text := range []string{"動画ID: sm9", "長さ: 2:30", "投稿日: 2020-01-02", "再生: 123", "コメント: 0", "投稿者の種別: チャンネル"} {
		if !v.HasText(text) {
			t.Fatal("detail fact lost", text)
		}
	}
}

func TestVideoDetailsCarryOnlySelectedKnownFacts(t *testing.T) {
	item, _ := makeVideo("sm9", "一覧で見た作品", 120, 50, 7)
	b := &browser{navigation: []screenRoute{routeFor(home)}, result: catalogPage{items: []card{item}}, closed: true}
	b.navigate(item.link)
	fallback := b.knownVideoDetails[item.link]
	if fallback.duration != "2:00" || fallback.views != "50" {
		t.Fatal("selected list facts not carried")
	}
	public := videoDetails{views: "123", posted: "2026-01-01"}.withFallback(fallback)
	if public.views != "123" || public.duration != "2:00" || public.comments != "7" {
		t.Fatal("known facts did not fill only absent values", public)
	}
	if b.knownVideoDetails[home+"watch/sm10"] != (videoDetails{}) {
		t.Fatal("facts reused for another video")
	}
}
