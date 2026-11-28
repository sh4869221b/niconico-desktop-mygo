package main

import (
	"math"
	"net/url"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"
)

const historyProgressInterval = 15 * time.Second

func validHistorySeconds(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value < 1<<33
}
func copyHistoryProgress(e, old entry) entry {
	if old.PositionKnown && validHistorySeconds(old.PositionSeconds) && validHistorySeconds(old.DurationSeconds) {
		e.PositionKnown = true
		e.PositionSeconds = old.PositionSeconds
		e.DurationSeconds = old.DurationSeconds
		e.Completed = old.Completed
	}
	return e
}
func historyPlaybackURL(e entry, resume bool) string {
	r := routeFor(e.openURL())
	if r.kind != screenVideo {
		return ""
	}
	if !resume {
		return playbackRestoreURL(r.link, 0)
	}
	u, _ := url.Parse(r.target)
	if u.Query().Has("from") {
		return r.target
	}
	if e.PositionKnown && !e.Completed && e.PositionSeconds > 0 && validHistorySeconds(e.PositionSeconds) {
		return playbackRestoreURL(r.link, e.PositionSeconds)
	}
	return r.link
}
func (b *browser) recordPlaybackProgress(p *playerWindow, now time.Time, force bool) bool {
	if p == nil || b.player != p || b.storeWarning != "" {
		return true
	}
	raw, seconds, duration, completed := "", float64(0), float64(0), false
	var stream *ui.VideoPlayer
	if n := p.native; n != nil {
		stream = n.stream
		if stream == nil || !stream.Ready() || stream.Preparing() || stream.Err() != nil {
			return true
		}
		raw, completed = p.target, stream.Ended()
	} else {
		if p.restoring != nil || p.playedPosition == nil || p.playedVideo == "" {
			return true
		}
		raw = p.playedWatch.link
		seconds = p.playedPosition.seconds
		duration = p.playedPosition.duration
		completed = p.playedPosition.ended
	}
	key, kind := canonical(raw)
	visit := p.historyVisitAt
	queuedVisit := p.queuedHistoryVisit
	if kind != "動画" || key != p.historyVisitURL || (visit.IsZero() && queuedVisit == nil) || p.historySuppressed[key] {
		return true
	}
	if !force && p.lastHistoryCompleted == completed && !p.lastHistoryWrite.IsZero() && now.Sub(p.lastHistoryWrite) < historyProgressInterval {
		return true
	}
	if stream != nil {
		position, confirmed := stream.ConfirmedPosition()
		if !confirmed {
			return true
		}
		seconds, duration = position.Seconds(), stream.Duration().Seconds()
	}
	if !validHistorySeconds(seconds) || !validHistorySeconds(duration) {
		return true
	}
	canRecord := func() bool { return !p.historySuppressed[key] && b.canRecordHistory() }
	if b.librarySaves != nil {
		path := b.historySettingsPath
		canRecord = func() bool { return canRecordAt(path) }
	}
	accepted := b.mutateLibrary(func(data *library) bool {
		at := visit
		if queuedVisit != nil {
			at = *queuedVisit
		}
		if at.IsZero() || !canRecord() {
			return false
		}
		i := slices.IndexFunc(data.History, func(e entry) bool { return e.URL == key && e.LastViewedAt.Equal(at) })
		if i < 0 {
			return false
		}
		e := &data.History[i]
		if e.PositionKnown && e.PositionSeconds == seconds && e.DurationSeconds == duration && e.Completed == completed {
			return false
		}
		e.PositionKnown, e.PositionSeconds, e.DurationSeconds, e.Completed = true, seconds, duration, completed
		return true
	}, func(changed, _ bool) {
		if changed && b.win != nil {
			b.win.Invalidate()
		}
	})
	if accepted {
		p.lastHistoryWrite, p.lastHistoryCompleted = now, completed
	}
	return accepted
}
func (b *browser) flushPlayerProgress(p *playerWindow) bool {
	if p.native == nil && p.view != nil {
		p.captureWebPosition(p.view.Page())
	}
	return b.recordPlaybackProgress(p, time.Now(), true)
}
func (b *browser) drawHistoryResume(c *ui.Context, e entry) {
	if e.Kind != "動画" {
		return
	}
	if !e.LastViewedAt.IsZero() {
		ui.Text(c, "最終視聴: "+e.LastViewedAt.Local().Format("2006-01-02 15:04")).FontSize(12).Wrap()
	}
	if e.PositionKnown {
		text := "視聴位置: " + formatPlaybackTime(time.Duration(e.PositionSeconds*float64(time.Second)))
		if e.DurationSeconds > 0 {
			text += " / " + formatPlaybackTime(time.Duration(e.DurationSeconds*float64(time.Second)))
		}
		if e.Completed {
			text = "視聴完了"
		}
		ui.Text(c, text).FontSize(12).Wrap()
	}
	ui.Row(c).Wrap().Gap(4).Children(func() {
		if e.PositionKnown && !e.Completed && e.PositionSeconds > 0 {
			if ui.Button(c, "続きから").Label(e.Title + "を続きから再生").Clicked() {
				b.openPlayerWithHistoryTarget(historyPlaybackURL(e, true), b.playMode, e.openURL())
			}
		}
		if ui.Button(c, "最初から").Label(e.Title + "を最初から再生").Clicked() {
			b.openPlayerWithHistoryTarget(historyPlaybackURL(e, false), b.playMode, e.openURL())
		}
	})
}
