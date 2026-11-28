package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestCommentListPreservesLongTextSeparatelyFromOverlay(t *testing.T) {
	for _, body := range []string{strings.Repeat("文", 240) + "終端", strings.Repeat("行\n", 13) + "終端"} {
		list := cleanCommentListText(body)
		if !strings.HasSuffix(list, "終端") {
			t.Fatalf("list lost tail: %q", list)
		}
		cues := makeDanmakuCues([]timedComment{{Body: list}})
		if len(cues) != 1 || strings.Contains(cues[0].body, "終端") {
			t.Fatal("overlay no longer uses its bounded text")
		}
	}
	if got := cleanCommentListText(strings.Repeat("字", 5000)); len([]rune(got)) != 4097 || !strings.HasSuffix(got, "…") {
		t.Fatal("list truncation was silent or unbounded")
	}
	if got := cleanCommentListText("a\r\nb\x00\t終端"); got != "a\nb　終端" {
		t.Fatalf("unsafe or broken list text: %q", got)
	}
}

func TestNativeCommentResponseKeepsListTail(t *testing.T) {
	body := strings.Repeat("文", 240) + "終端"
	payload, _ := json.Marshal(map[string]any{"meta": map[string]any{"status": 200}, "data": map[string]any{"threads": []any{map[string]any{"comments": []timedComment{{Body: body}}}}}})
	prior := http.DefaultTransport
	http.DefaultTransport = streamTransportFunc(func(r *http.Request) (*http.Response, error) { return testResponse(200, string(payload)), nil })
	t.Cleanup(func() { http.DefaultTransport = prior })
	var watch watchData
	watch.Video.ID = "sm9"
	watch.Comment.NVComment.Server = "https://nvcomment.nicovideo.jp"
	watch.Comment.NVComment.ThreadKey = "fixture"
	watch.Comment.NVComment.Params = json.RawMessage(`{}`)
	rows, err := nativeComments(t.Context(), watch)
	if err != nil || len(rows) != 1 || rows[0].Body != body {
		t.Fatalf("ingestion truncated list body: %v %+v", err, rows)
	}
}
