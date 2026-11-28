package main

import (
	"github.com/egoist/mygo/ui"
	"os"
	"path/filepath"
	"testing"
)

func TestSidebarVisibilityRemembersUserChoice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "layout.json")
	shown, err := loadSidebarVisibility(path)
	if err != nil || !shown {
		t.Fatal("new profiles should keep the visible sidebar default")
	}
	b := &browser{index: -1, closed: true, showLibrary: shown, layoutPath: path}
	v := ui.NewTester(b.draw, 1000, 700)
	if err := v.Click("登録・履歴"); err != nil {
		t.Fatal(err)
	}
	shown, err = loadSidebarVisibility(path)
	if err != nil || shown {
		t.Fatal("hidden sidebar choice not saved")
	}
	if err := v.Click("登録・履歴"); err != nil {
		t.Fatal(err)
	}
	shown, err = loadSidebarVisibility(path)
	if err != nil || !shown {
		t.Fatal("visible sidebar choice not saved")
	}
}
func TestDamagedLayoutSettingsAreProtected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "layout.json")
	original := []byte("broken json")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	shown, err := loadSidebarVisibility(path)
	if err == nil || !shown {
		t.Fatal("damaged settings not detected")
	}
	b := &browser{showLibrary: shown, layoutPath: path, layoutLoadErr: err}
	b.toggleLibrary()
	data, _ := os.ReadFile(path)
	if string(data) != string(original) {
		t.Fatal("damaged file overwritten")
	}
}
