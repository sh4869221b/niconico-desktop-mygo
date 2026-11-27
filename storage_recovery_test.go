package main

import (
	"github.com/egoist/mygo/ui"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsOfferStorageRecovery(t *testing.T) {
	b := &browser{index: -1, closed: true, showPlaybackSettings: true}
	v := ui.NewTester(b.draw, 1000, 850)
	if _, ok := v.Find("保存先と復旧を表示"); !ok {
		t.Fatal("settings do not expose storage recovery")
	}
}

func TestBackupAndRecoveryKeepOriginalFiles(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "library.json")
	backup := filepath.Join(dir, "backup.json")
	restore := filepath.Join(dir, "recovered.json")
	damaged := []byte(`{"Pins":broken`)
	if err := os.WriteFile(source, damaged, 0600); err != nil {
		t.Fatal(err)
	}
	if err := saveProfileCopy(source, backup, "library", false); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(backup)
	if string(got) != string(damaged) {
		t.Fatal("backup did not preserve damaged bytes")
	}
	if err := saveProfileCopy(source, restore, "library", true); err == nil {
		t.Fatal("invalid recovery input accepted")
	}
	if _, err := os.Stat(restore); !os.IsNotExist(err) {
		t.Fatal("invalid recovery created destination")
	}
	valid := filepath.Join(dir, "valid.json")
	if err := saveLibrary(valid, library{Pins: []entry{{URL: home + "watch/sm9", Title: "kept", Kind: "動画"}}}); err != nil {
		t.Fatal(err)
	}
	if err := saveProfileCopy(valid, restore, "library", true); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadLibrary(restore)
	if err != nil || len(loaded.Pins) != 1 {
		t.Fatal("recovery copy cannot be read")
	}
	if err := saveProfileCopy(valid, source, "library", true); err == nil {
		t.Fatal("existing original overwritten")
	}
	got, _ = os.ReadFile(source)
	if string(got) != string(damaged) {
		t.Fatal("original changed")
	}
	if err := saveProfileCopy(valid, backup, "library", false); err == nil {
		t.Fatal("existing backup overwritten")
	}
}
func TestStorageStatesAndReadFailureAreVisible(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "comment-ng.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	files := inspectProfileFiles(dir)
	if len(files) != 3 || !strings.Contains(files[1].state, "読み込み失敗") {
		t.Fatal("NG read error not exposed", files)
	}
	b := &browser{index: -1, closed: true, ngReadWarning: "NG設定を読み込めません: " + files[1].path}
	v := ui.NewTester(b.draw, 1000, 700)
	if !v.HasText(files[1].path) {
		t.Fatal("startup warning does not name NG file")
	}
	path := filepath.Join(dir, "playback.json")
	original := []byte("broken")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	b.playbackPath = path
	b.playbackBlocked = true
	b.changePlayback(nativePlayback)
	got, _ := os.ReadFile(path)
	if string(got) != string(original) {
		t.Fatal("unreadable playback preference overwritten")
	}
}
