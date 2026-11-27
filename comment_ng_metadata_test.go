package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestNGMetadataMigrationScopeAndSettingsRemoval(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ng.json")
	if err := os.WriteFile(path, []byte(`[{"id":"123"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	ng, err := loadCommentNG(path)
	if err != nil {
		t.Fatal(err)
	}
	a := commentAuthor{ID: strings.Repeat("a", 27), Video: "sm9"}
	date := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	if err := ng.changeWithDetails(a, true, "テスト動画", date); err != nil {
		t.Fatal(err)
	}
	ng, err = loadCommentNG(path)
	if err != nil || !ng.blocked[commentAuthor{ID: "123"}] || ng.metadata[a].Title != "テスト動画" || !ng.metadata[a].AddedAt.Equal(date) {
		t.Fatalf("lost migration or details: %v", err)
	}
	if ng.blocked[commentAuthor{ID: a.ID, Video: "sm12"}] {
		t.Fatal("anonymous identity linked across videos")
	}
	b := &browser{commentNG: ng}
	v := ui.NewTester(b.drawPlaybackSettings, 640, 400)
	if err := v.Click("ローカルNG管理"); err != nil {
		t.Fatal(err)
	}
	if !v.HasText("この動画だけ") || !v.HasText("テスト動画") || !v.HasText("2026-11-27") {
		t.Fatal("settings omitted scope or identifying metadata")
	}
	if err := v.Click("解除 匿名 sm9 / aaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	v.Frame()
	if b.commentNG.blocked[a] || !b.commentNG.blocked[commentAuthor{ID: "123"}] {
		t.Fatal("settings removed wrong identity")
	}
}
