package main

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The transport returns synthetic response bodies in memory. Load measures the
// catalog's request construction, decoding and normalization without network I/O.
func browseFixture(kind screenKind, n int) (screenRoute, catalog) {
	var target, body string
	var rows []string
	for i := range n {
		video := fmt.Sprintf(`{"id":"sm%d","title":"動画 %03d のタイトル","duration":125,"owner":{"id":"7","ownerType":"user","name":"投稿者名"},"count":{"view":12345,"comment":42}}`, i+1, i+1)
		switch kind {
		case screenSearch:
			rows = append(rows, fmt.Sprintf(`{"contentId":"sm%d","title":"動画 %03d のタイトル","lengthSeconds":125,"viewCounter":12345,"commentCounter":42}`, i+1, i+1))
		case screenUser:
			rows = append(rows, `{"essential":`+video+`}`)
		case screenList:
			rows = append(rows, `{"video":`+video+`}`)
		}
	}
	items := strings.Join(rows, ",")
	switch kind {
	case screenSearch:
		target = "search/test"
		body = `{"meta":{"status":200,"totalCount":101},"data":[` + items + `]}`
	case screenUser:
		target = "user/7/video"
		body = `{"meta":{"status":200},"data":{"totalCount":101,"items":[` + items + `]}}`
	case screenList:
		target = "mylist/7"
		body = `{"meta":{"status":200},"data":{"mylist":{"name":"一覧","description":"説明 https://nico.ms/sm1","totalItemCount":101,"items":[` + items + `]}}}`
	case screenVideo:
		target = "watch/sm1"
		body = `<nicovideo_thumb_response status="ok"><thumb><video_id>sm1</video_id><title>動画</title><description>テスト動画 https://nico.ms/sm2</description><user_id>7</user_id><user_nickname>投稿者名</user_nickname></thumb></nicovideo_thumb_response>`
	}
	c := newCatalog(nil)
	c.http.Transport = requestFunc(func(*http.Request) (*http.Response, error) { return testResponse(200, body), nil })
	return routeFor(home + target), c
}

func BenchmarkNativeBrowse(b *testing.B) {
	for _, tc := range []struct {
		name string
		kind screenKind
		n    int
	}{{"Search24", screenSearch, 24}, {"Uploads100", screenUser, 100}, {"Mylist100", screenList, 100}, {"Details", screenVideo, 0}} {
		b.Run(tc.name, func(b *testing.B) {
			r, c := browseFixture(tc.kind, tc.n)
			p, err := c.load(b.Context(), r)
			if err != nil || len(p.items) != tc.n {
				b.Fatalf("items %d, err %v", len(p.items), err)
			}
			app := &browser{navigation: []screenRoute{r}, result: p}
			b.Run("Load", func(b *testing.B) {
				for b.Loop() {
					if _, err := c.load(b.Context(), r); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("Frame", func(b *testing.B) {
				rec := ui.NewRecorder(app.draw, ui.RecorderOptions{Width: 1260, Height: 850})
				defer rec.Close()
				rec.Frame(0)
				var at time.Duration
				for b.Loop() {
					at += time.Second / 60
					rec.Frame(at)
				}
			})
			if tc.n > 0 {
				b.Run("Scroll", func(b *testing.B) {
					view := ui.NewTester(app.draw, 1260, 850)
					step := 0
					for b.Loop() {
						dy := float32(32)
						if step%24 >= 12 {
							dy = -dy
						}
						view.Scroll(700, 550, 0, dy)
						step++
					}
				})
			}
		})
	}
}

// Replay the work of revisiting a posting list and its details. Each leg loads
// a fresh response and draws the resulting screen. There is no event-loop or
// network wait in this measurement.
func BenchmarkNativeBrowseBackForward(b *testing.B) {
	list, lists := browseFixture(screenUser, 100)
	detail, details := browseFixture(screenVideo, 0)
	app := browser{navigation: []screenRoute{list, detail}, index: 1}
	rec := ui.NewRecorder(app.draw, ui.RecorderOptions{Width: 1260, Height: 850})
	defer rec.Close()
	requests := 0
	for _, c := range []catalog{lists, details} {
		transport := c.http.Transport
		c.http.Transport = requestFunc(func(req *http.Request) (*http.Response, error) {
			requests++
			return transport.RoundTrip(req)
		})
	}
	var at time.Duration
	for b.Loop() {
		for i, c := range []catalog{lists, details} {
			app.index = i
			p, err := c.load(b.Context(), app.navigation[i])
			if err != nil {
				b.Fatal(err)
			}
			app.result = p
			at += time.Second / 60
			rec.Frame(at)
		}
	}
	b.ReportMetric(float64(requests)/float64(b.N), "fixture-requests/op")
}
