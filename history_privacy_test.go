package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
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
	b := &browser{player: p, libraryPath: filepath.Join(dir, "library.json"), historySettingsPath: path, data: library{Pins: []entry{recent}, History: []entry{recent, old, legacy}}}
	if err := saveLibrary(b.libraryPath, b.data); err != nil {
		t.Fatal(err)
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
	newer := e
	newer.LastViewedAt = time.Now().UTC()
	if err := saveLibrary(b.libraryPath, library{History: []entry{newer}}); err != nil {
		t.Fatal(err)
	}
	b.eraseSelectedHistory()
	if len(b.data.History) != 1 || !b.data.History[0].LastViewedAt.Equal(newer.LastViewedAt) {
		t.Fatal("confirmation erased a newer visit")
	}
}

func TestLegacyVisitGetsADateBeforeDeduplication(t *testing.T) {
	const title = "同じタイトル"
	for _, incoming := range []string{title, ""} {
		e, _ := makeEntry(home+"watch/sm9", title)
		p := &playerWindow{}
		b := &browser{player: p, libraryPath: filepath.Join(t.TempDir(), "library.json"), data: library{History: []entry{e}}}
		if err := saveLibrary(b.libraryPath, b.data); err != nil {
			t.Fatal(err)
		}
		b.recordVideo(p, e.URL, incoming)
		got, err := loadLibrary(b.libraryPath)
		if err != nil || len(got.History) != 1 || got.History[0].Title != title || len(historyInPeriod(got.History, "過去24時間", time.Now())) != 1 {
			t.Fatalf("revisited history remained undated or lost its title: %+v (%v)", got.History, err)
		}
		b.recordVideo(p, e.URL, title+" 更新")
		if !b.data.History[0].LastViewedAt.Equal(got.History[0].LastViewedAt) {
			t.Fatal("metadata refresh counted as a new visit")
		}
	}
}

func TestHistoryStopAlsoProtectsAgainstAnotherOpenInstance(t *testing.T) {
	dir := t.TempDir()
	p := &playerWindow{}
	first := &browser{player: p, libraryPath: filepath.Join(dir, "library.json"), historySettingsPath: filepath.Join(dir, "history.json")}
	second := &browser{libraryPath: first.libraryPath, historySettingsPath: first.historySettingsPath}
	second.setHistoryRecording(false)
	first.recordVideo(p, home+"watch/sm9", "履歴に残さない")
	got, err := loadLibrary(first.libraryPath)
	if err != nil || len(got.History) != 0 || !second.historyDisabled {
		t.Fatalf("another instance kept recording after the preference changed: %+v (%v)", got.History, err)
	}
}

func TestHistorySettingReflectsAnotherInstance(t *testing.T) {
	dir := t.TempDir()
	p := &playerWindow{}
	first := &browser{index: -1, closed: true, closeHelpAcknowledged: true, showPlaybackSettings: true, player: p, libraryPath: filepath.Join(dir, "library.json"), historySettingsPath: filepath.Join(dir, "history.json")}
	second := &browser{libraryPath: first.libraryPath, historySettingsPath: first.historySettingsPath}
	const label = "今後の履歴を記録"
	v := ui.NewTester(first.draw, 1000, 850)
	assertChecked := func(want bool) {
		t.Helper()
		box, found := v.Access(label)
		if !found || box.Checked != want {
			t.Fatalf("history setting checked=%v, found=%v, want %v", box.Checked, found, want)
		}
	}
	assertChecked(true)
	second.setHistoryRecording(false)
	v.Advance(time.Second)
	assertChecked(false)
	first.recordVideo(p, home+"watch/sm9", "記録しない")
	second.setHistoryRecording(true)
	v.Advance(time.Second)
	assertChecked(true)
	first.recordVideo(p, home+"watch/sm10", "履歴に記録")
	got, err := loadLibrary(first.libraryPath)
	if err != nil || len(got.History) != 1 || got.History[0].URL != home+"watch/sm10" {
		t.Fatalf("stored history did not follow the shared setting: %+v (%v)", got.History, err)
	}
}
