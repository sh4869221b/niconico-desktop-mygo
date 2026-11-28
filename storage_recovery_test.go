package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSettingsOfferStorageRecovery(t *testing.T) {
	b := &browser{index: -1, closed: true, showPlaybackSettings: true}
	v := ui.NewTester(b.draw, 1000, 850)
	if _, ok := v.Find("保存先と復旧を表示"); !ok {
		t.Fatal("settings do not expose storage recovery")
	}
}

func TestOversizedDamagedProfileCanBeBackedUpWithoutValidation(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "library.json")
	backup := filepath.Join(dir, "backup.json")
	recovery := filepath.Join(dir, "recover.json")
	damaged := bytes.Repeat([]byte{0xa5}, maxLibraryBytes+64)
	if err := os.WriteFile(source, damaged, 0600); err != nil {
		t.Fatal(err)
	}
	if files := inspectProfileFiles(dir); len(files) == 0 || !strings.Contains(files[0].state, "読み込み失敗") {
		t.Fatalf("oversized profile passed inspection: %+v", files)
	}
	if err := saveProfileCopy(source, backup, "library", false); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{source, backup} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, damaged) {
			t.Fatalf("damaged bytes were not preserved: %d bytes (%v)", len(got), err)
		}
	}
	if err := saveProfileCopy(source, recovery, "library", true); err == nil {
		t.Fatal("oversized profile passed recovery validation")
	}
	if _, err := os.Stat(recovery); !os.IsNotExist(err) {
		t.Fatalf("validation created a destination: %v", err)
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
	b.changePlayback(nativePlayback)
	got, _ := os.ReadFile(path)
	if !b.playbackBlocked || string(got) != string(original) {
		t.Fatal("unreadable playback preference overwritten")
	}
}
