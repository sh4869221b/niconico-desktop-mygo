package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCollectionTitleFilterSearchesPastTheFirstPageInDefaultOrder(t *testing.T) {
	for _, target := range []string{home + "user/123/video", home + "mylist/123", mylists + "#/123"} {
		t.Run(target, func(t *testing.T) {
			r := routeFor(target)
			r.collectionQuery = "NEKO"
			var pages []string
			client := catalog{
				http: &http.Client{Transport: requestFunc(func(req *http.Request) (*http.Response, error) {
					q := req.URL.Query()
					pages = append(pages, q.Get("page"))
					if q.Get("q") != "" || q.Get("sortKey") != "" && r.kind != screenUser {
						t.Fatalf("request changed the existing list order or sent the local query: %v", q)
					}
					rows := []map[string]any{}
					for _, item := range []struct{ id, title string }{{"sm12", "neko 最初"}, {"sm4", "nEko 次"}} {
						if q.Get("page") == "2" {
							rows = append(rows, map[string]any{"video": map[string]any{"id": item.id, "title": item.title}, "essential": map[string]any{"id": item.id, "title": item.title}})
						}
					}
					if q.Get("page") == "1" {
						video := map[string]any{"id": "sm9", "title": "対象外"}
						rows = append(rows, map[string]any{"video": video, "essential": video})
					}
					result := map[string]any{"items": rows, "hasNext": q.Get("page") == "1"}
					if r.kind == screenUser {
						result["totalCount"] = 3
					} else {
						result["totalItemCount"] = 3
						result = map[string]any{"mylist": result}
					}
					return collectionResponse(result), nil
				})},
				session: func(string) (*http.Cookie, error) {
					return &http.Cookie{Name: "user_session", Value: "fixture", Secure: true}, nil
				},
			}
			p, err := client.load(t.Context(), r)
			if err != nil || !slices.Equal(pages, []string{"1", "2"}) || p.more || len(p.items) != 2 || p.items[0].link != home+"watch/sm12" || p.items[1].link != home+"watch/sm4" {
				t.Fatalf("whole-list filtering did not preserve the ordered matches: %v %+v (%v)", pages, p.items, err)
			}
		})
	}
}

func TestCollectionFilterWithIncompleteResponseReturnsNoPartialResults(t *testing.T) {
	for _, metadata := range []map[string]any{{}, {"hasNext": false, "totalItemCount": 20}} {
		metadata["items"] = []map[string]any{{"video": map[string]any{"id": "sm1", "title": "neko"}}}
		client := catalog{http: &http.Client{Transport: requestFunc(func(*http.Request) (*http.Response, error) {
			return collectionResponse(map[string]any{"mylist": metadata}), nil
		})}}
		r := routeFor(home + "mylist/123")
		r.collectionQuery = "neko"
		p, err := client.load(t.Context(), r)
		if !errors.Is(err, errIncompleteCollection) || len(p.items) != 0 {
			t.Fatalf("partial collection appeared complete: %+v (%v)", p.items, err)
		}
	}
}

func TestCollectionFilterKeepsTheOfficialURLAndCanBeCleared(t *testing.T) {
	r := routeFor(home + "mylist/123?page=2")
	b := browser{navigation: []screenRoute{r}, closed: true, closeHelpAcknowledged: true}
	v := ui.NewTester(b.draw, 1260, 850)
	click := func(label string) {
		t.Helper()
		if err := v.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	click("一覧全体を絞り込む")
	if !v.Focused("一覧全体の動画タイトル") {
		t.Fatal("opening the title filter did not focus its input")
	}
	v.Type("  neko  ")
	click("絞り込む")
	filtered := b.navigation[b.index]
	if filtered.collectionQuery != "neko" || filtered.page != 0 || filtered.target != home+"mylist/123" {
		t.Fatalf("filter did not start from the complete collection: %+v", filtered)
	}
	click("絞り込みを解除")
	if cleared := b.navigation[b.index]; cleared.collectionQuery != "" || cleared.page != 0 || cleared.target != filtered.target {
		t.Fatalf("clear did not return to the default collection: %+v", cleared)
	}
	if !v.Focused("一覧全体を絞り込む") {
		t.Fatal("clearing the filter did not restore keyboard focus")
	}
}

func collectionResponse(data map[string]any) *http.Response {
	body, _ := json.Marshal(map[string]any{"meta": map[string]any{"status": 200}, "data": data})
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}
}
