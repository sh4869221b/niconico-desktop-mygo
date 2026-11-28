package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSidebarVisibilityRemembersUserChoice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "layout.json")
	shown, err := loadSidebarVisibility(path)
	if err != nil || !shown {
		t.Fatal("new profiles should keep the visible sidebar default")
	}
	b := &browser{index: -1, closed: true, showLibrary: shown, layoutPath: path}
	v := ui.NewTester(b.draw, 1000, 700)
	if _, visible := v.Find("登録・履歴領域"); !visible {
		t.Fatal("new profiles should show the sidebar")
	}
	if err := v.Click("登録・履歴"); err != nil {
		t.Fatal(err)
	}
	shown, err = loadSidebarVisibility(path)
	if err != nil || shown {
		t.Fatal("hidden sidebar choice not saved")
	}
	b = &browser{index: -1, closed: true, showLibrary: shown, layoutPath: path}
	v = ui.NewTester(b.draw, 1000, 700)
	if _, visible := v.Find("登録・履歴領域"); visible {
		t.Fatal("a new window should keep the sidebar hidden")
	}
	if err := v.Click("登録・履歴"); err != nil {
		t.Fatal(err)
	}
	shown, err = loadSidebarVisibility(path)
	if err != nil || !shown {
		t.Fatal("visible sidebar choice not saved")
	}
	b = &browser{index: -1, closed: true, showLibrary: shown, layoutPath: path}
	v = ui.NewTester(b.draw, 1000, 700)
	if _, visible := v.Find("登録・履歴領域"); !visible {
		t.Fatal("a new window should show the restored sidebar")
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
