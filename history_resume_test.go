package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"image"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryOffersPlaybackChoice(t *testing.T) {
	e, _ := makeEntry(home+"watch/sm9", "作品")
	b := &browser{index: -1, closed: true, showLibrary: true, history: true, data: library{History: []entry{e}}}
	v := ui.NewTester(b.draw, 1000, 700)
	if _, ok := v.Find("作品を最初から再生"); !ok {
		t.Fatal("history has no explicit playback choice")
	}
}

func TestVerifiedHistoryProgressPersistsAndResumes(t *testing.T) {
	r := routeFor(home + "watch/sm9")
	e, _ := makeEntry(r.link, "作品")
	p := &playerWindow{}
	p.recordWebSnapshot([]any{r.target, r.key, 90.75, true, 300.0, false}, r)
	b := &browser{win: new(mygo.Window), player: p, libraryPath: filepath.Join(t.TempDir(), "library.json"), data: library{History: []entry{e}}}
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	b.recordPlaybackProgress(p, now, false)
	saved, err := loadLibrary(b.libraryPath)
	if err != nil {
		t.Fatal(err)
	}
	got := saved.History[0]
	if !got.PositionKnown || got.PositionSeconds != 90.75 || got.DurationSeconds != 300 || !got.LastViewedAt.Equal(now) {
		t.Fatal("verified position not retained", got)
	}
	if historyPlaybackURL(got, true) != r.link+"?from=90" || historyPlaybackURL(got, false) != r.link+"?from=0" {
		t.Fatal("resume choices incorrect")
	}
	got.URL = r.link + "?from=12"
	if historyPlaybackURL(got, true) != got.URL {
		t.Fatal("explicit starting time overridden")
	}
	p.playedPosition.seconds = 101
	b.recordPlaybackProgress(p, now.Add(14*time.Second), false)
	if b.data.History[0].PositionSeconds != 90.75 {
		t.Fatal("position written more often than 15 seconds")
	}
	b.recordPlaybackProgress(p, now.Add(15*time.Second), false)
	if b.data.History[0].PositionSeconds != 101 {
		t.Fatal("interval sample not saved")
	}
	p.playedPosition.seconds = 105
	b.recordPlaybackProgress(p, now.Add(16*time.Second), true)
	if b.data.History[0].PositionSeconds != 105 {
		t.Fatal("final close sample lost")
	}
	b.recordVideo(p, r.link, "更新されたタイトル")
	if b.data.History[0].PositionSeconds != 105 {
		t.Fatal("title update erased progress")
	}
	b.history = true
	b.remove(b.data.History[0])
	b.recordPlaybackProgress(p, now.Add(time.Minute), true)
	if len(b.data.History) != 0 {
		t.Fatal("deleted history recreated by sample")
	}
}
func TestLegacyHistoryAndCompletion(t *testing.T) {
	e, _ := makeEntry(home+"watch/sm9", "legacy")
	path := filepath.Join(t.TempDir(), "library.json")
	if err := saveLibrary(path, library{History: []entry{e}}); err != nil {
		t.Fatal(err)
	}
	saved, err := loadLibrary(path)
	if err != nil || len(saved.History) != 1 || saved.History[0].PositionKnown {
		t.Fatal("old history changed or invented a position")
	}
	r := routeFor(e.URL)
	p := &playerWindow{}
	p.recordWebSnapshot([]any{r.target, r.key, 300.0, true, 300.0, true}, r)
	b := &browser{player: p, libraryPath: path, data: saved}
	b.recordPlaybackProgress(p, time.Now(), true)
	if !b.data.History[0].Completed || historyPlaybackURL(b.data.History[0], true) != e.URL {
		t.Fatal("completed video did not restart at beginning")
	}
	p.playedPosition = nil
	b.recordPlaybackProgress(p, time.Now(), true)
	if !b.data.History[0].Completed {
		t.Fatal("unverified sample replaced completion")
	}
}
func TestNativeHistoryUsesActualPlayerPosition(t *testing.T) {
	stream, err := ui.NewVideoPlayer(danmakuTestVideo{image.NewRGBA(image.Rect(0, 0, 40, 30))})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	stream.Pause()
	stream.Seek(10 * time.Second)
	v := ui.NewTester(func(c *ui.Context) { ui.Video(c, stream).Fill() }, 640, 400)
	v.Frame()
	e, _ := makeEntry(home+"watch/sm9", "native")
	p := &playerWindow{target: e.URL, native: &nativePlayer{stream: stream}}
	b := &browser{player: p, libraryPath: filepath.Join(t.TempDir(), "library.json"), data: library{History: []entry{e}}}
	b.recordPlaybackProgress(p, time.Now(), true)
	got := b.data.History[0]
	if !got.PositionKnown || got.PositionSeconds != stream.Position().Seconds() || got.DurationSeconds != stream.Duration().Seconds() {
		t.Fatal("native playback did not supply confirmed position", got)
	}
}

func TestHistoryResumeChoiceAndBadSnapshot(t *testing.T) {
	e, _ := makeEntry(home+"watch/sm9", "作品")
	e.PositionKnown = true
	e.PositionSeconds = 90
	e.DurationSeconds = 300
	b := &browser{index: -1, closed: true, showLibrary: true, history: true, data: library{History: []entry{e}}}
	v := ui.NewTester(b.draw, 1000, 700)
	if err := v.Click("作品を続きから再生"); err != nil {
		t.Fatal(err)
	}
	if b.playbackRequest != 1 {
		t.Fatal("resume did not use selected player")
	}
	for _, sample := range []any{[]any{e.URL, "sm9", 90.0, true, -1.0, false}, []any{e.URL, "sm9", 90.0, true, 300.0, "ended"}} {
		p := &playerWindow{}
		p.recordWebSnapshot(sample, routeFor(e.URL))
		if p.playedPosition != nil {
			t.Fatal("invalid history snapshot accepted")
		}
	}
}
