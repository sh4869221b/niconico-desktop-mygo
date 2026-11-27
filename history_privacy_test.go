package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryRecordingChoiceIsVisible(t *testing.T) {
	b := &browser{index: -1, showPlaybackSettings: true, closed: true}
	v := ui.NewTester(b.draw, 1000, 850)
	if _, ok := v.Find("今後の履歴を記録"); !ok {
		t.Fatal("no history recording preference")
	}
}

func TestHistoryRecordingDefaultStopAndPeriodErase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	disabled, err := loadHistoryRecording(path)
	if err != nil || disabled {
		t.Fatal("existing recording default changed")
	}
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	recent, _ := makeEntry(home+"watch/sm1", "recent")
	recent.LastViewedAt = now.Add(-time.Hour)
	old, _ := makeEntry(home+"watch/sm2", "old")
	old.LastViewedAt = now.Add(-40 * 24 * time.Hour)
	legacy, _ := makeEntry(home+"watch/sm3", "legacy")
	p := &playerWindow{}
	b := &browser{win: new(mygo.Window), player: p, libraryPath: filepath.Join(dir, "library.json"), historySettingsPath: path, data: library{Pins: []entry{recent}, History: []entry{recent, old, legacy}}}
	if !b.persist() {
		t.Fatal("fixture not saved")
	}
	b.setHistoryRecording(false)
	disabled, err = loadHistoryRecording(path)
	if err != nil || !disabled || len(b.data.History) != 3 {
		t.Fatal("recording stop erased history or was not saved")
	}
	b.recordVideo(p, home+"watch/sm4", "private")
	if len(b.data.History) != 3 {
		t.Fatal("history recorded after stop")
	}
	selected := historyInPeriod(b.data.History, "過去7日間", now)
	if len(selected) != 1 || selected[0].URL != recent.URL {
		t.Fatal("period includes old or unknown dates")
	}
	b.historyErasePending = selected
	b.eraseSelectedHistory()
	saved, err := loadLibrary(b.libraryPath)
	if err != nil || len(saved.Pins) != 1 || len(saved.History) != 2 || !saved.History[0].LastViewedAt.Equal(old.LastViewedAt) {
		t.Fatal("erase damaged other data", saved, err)
	}
	if !p.historySuppressed[recent.URL] {
		t.Fatal("playing video would reappear after erase")
	}
	if len(historyInPeriod(saved.History, "すべて", now)) != 2 {
		t.Fatal("legacy history cannot be explicitly erased")
	}
}
func TestEraseConfirmationDoesNotDeleteNewerVisit(t *testing.T) {
	e, _ := makeEntry(home+"watch/sm9", "video")
	e.LastViewedAt = time.Now().UTC().Add(-time.Hour)
	b := &browser{libraryPath: filepath.Join(t.TempDir(), "library.json"), data: library{History: []entry{e}}, historyErasePending: []entry{e}}
	b.data.History[0].LastViewedAt = time.Now().UTC()
	b.eraseSelectedHistory()
	if len(b.data.History) != 1 {
		t.Fatal("confirmation erased a newer visit")
	}
}
