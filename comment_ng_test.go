package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommentAuthorsUseExactIDAndScope(t *testing.T) {
	token := strings.Repeat("a", 27)
	author := func(video, id string, commands ...string) commentAuthor {
		raw, _ := json.Marshal(id)
		return authorForComment(video, raw, commands)
	}
	if author("sm9", "987") != author("sm12", "987") || author("sm9", "987") == author("sm9", "123") {
		t.Fatal("public user IDs did not match by account")
	}
	for _, id := range []string{token, "nvc:" + token} {
		a := author("sm9", id, "184")
		if !a.valid() || a != author("sm9", id, "184") || a == author("sm12", id, "184") {
			t.Fatal("anonymous ID was not limited to the video")
		}
	}
	for _, a := range []commentAuthor{
		author("sm9", ""), author("sm9", "0"), author("sm9", "unknown"), author("sm9", "77", "184"),
		author("", "nvc:"+token), author("123", "nvc:"+token),
		authorForComment("sm9", json.RawMessage(`null`), nil),
		authorForComment("sm9", json.RawMessage(`{"unexpected":"77"}`), nil),
	} {
		if a.ID != "" {
			t.Fatalf("unavailable ID was treated as a user: %+v", a)
		}
	}
}

func TestCommentNGPersistsChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "comment-ng.json")
	ng, err := loadCommentNG(path)
	if err != nil {
		t.Fatal(err)
	}
	a := commentAuthor{ID: "123"}
	b := commentAuthor{ID: "nvc:" + strings.Repeat("b", 27), Video: "sm9"}
	if err := ng.change(a, true); err != nil {
		t.Fatal(err)
	}
	if err := ng.change(b, true); err != nil {
		t.Fatal(err)
	}
	reloaded, err := loadCommentNG(path)
	if err != nil || !reloaded.blocked[a] || !reloaded.blocked[b] {
		t.Fatalf("NG settings lost after reopening: %v", err)
	}
	if err := reloaded.change(a, false); err != nil {
		t.Fatal(err)
	}
	reloaded, err = loadCommentNG(path)
	if err != nil || reloaded.blocked[a] || !reloaded.blocked[b] {
		t.Fatalf("NG removal did not persist independently: %v", err)
	}
}
