package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"time"

	"github.com/egoist/mygo"
)

//go:embed player_web.js
var webPlayerPreload string

type webPlaybackPosition struct {
	seconds float64
	paused  bool
}

type webPlaybackRestore struct {
	video, target string
	position      webPlaybackPosition
	token         uint64
	deadline      time.Time
	resume        bool
}

func playbackPosition(value any, videoID string) *webPlaybackPosition {
	fields, ok := value.([]any)
	if !ok || len(fields) != 3 || videoID == "" || fields[0] != videoID {
		return nil
	}
	seconds, timeOK := fields[1].(float64)
	paused, pauseOK := fields[2].(bool)
	if !timeOK || !pauseOK || !(seconds >= 0 && seconds < 1<<33) {
		return nil
	}
	return &webPlaybackPosition{seconds: seconds, paused: paused}
}

func playbackRestoreURL(target string, seconds float64) string {
	u, err := url.Parse(target)
	if err != nil {
		return target
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return target
	}
	// The page's initial seek should agree with the restore script, which applies fractions of a second.
	query.Set("from", strconv.FormatFloat(math.Floor(seconds), 'f', 0, 64))
	u.RawQuery = query.Encode()
	return u.String()
}

func (p *playerWindow) notePlayedWatch(route screenRoute, newDocument bool) {
	if route.kind != screenVideo {
		return
	}
	if newDocument || p.playedWatch.link != route.link {
		p.playedPosition = nil
	}
	p.playedWatch = route
}

func (p *playerWindow) finishWebLoad(raw string) {
	// MyGo publishes this commit and completion only for the same accepted load.
	// The live URL may already be a search route by the completion callback.
	p.notePlayedWatch(routeFor(p.document.committedURL), true)
	p.document.finish(raw)
}

func (p *playerWindow) captureWebPosition(page *mygo.Page) {
	played, revision := p.playedWatch, p.document.revision
	if played.kind != screenVideo || p.restoring != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), webSourceInterval)
	defer cancel()
	value, err := page.EvalContext(ctx, `window.__nicoDesktopMedia?.snapshot() ?? null`)
	if err == nil && p.view != nil && p.playedWatch == played && p.document.revision == revision {
		if snapshot := playbackPosition(value, played.key); snapshot != nil {
			p.playedPosition = snapshot
		}
	}
}

func (p *playerWindow) returnToPlayedWatch() {
	pl := p.playedWatch
	if pl.kind != screenVideo {
		return
	}
	position := p.playedPosition
	if p.restoring != nil && p.restoring.video == pl.key {
		position = &p.restoring.position
	}
	target := pl.target
	p.restoring = nil
	if position != nil {
		target = playbackRestoreURL(target, position.seconds)
		p.restoreToken++
		p.restoring = &webPlaybackRestore{video: pl.key, target: target, position: *position,
			token: p.restoreToken, deadline: time.Now().Add(20 * time.Second)}
	}
	p.load(target)
}

func (p *playerWindow) applyWebRestore(page *mygo.Page, route screenRoute) {
	state := p.restoring
	if state == nil || route.kind != screenVideo || route.key != state.video {
		return
	}
	if time.Now().After(state.deadline) {
		p.restoring = nil
		p.status = "再生状態を復元できませんでした。"
		p.win.Invalidate()
		return
	}
	encodedVideo, _ := json.Marshal(state.video)
	code := fmt.Sprintf(`window.__nicoDesktopMedia?.restore(%d, %s, %s, %t, %t) ?? "wait"`,
		state.token, encodedVideo, strconv.FormatFloat(state.position.seconds, 'g', -1, 64), state.position.paused, state.resume)
	ctx, cancel := context.WithTimeout(context.Background(), webSourceInterval)
	defer cancel()
	revision := p.document.revision
	eval := page.EvalContext
	if state.resume {
		// Resume one previously playing video without changing WebView2's autoplay policy.
		state.resume = false
		eval = page.EvalUserGestureContext
	}
	result, err := eval(ctx, code)
	if err != nil || p.restoring != state || p.view == nil || p.document.revision != revision ||
		routeFor(page.URL()).link != route.link {
		return
	}
	switch result {
	case "play":
		state.resume = !state.position.paused
	case "applied":
		p.restoring = nil
	case "blocked":
		p.restoring = nil
		p.status = "再生状態を復元できませんでした。"
		p.win.Invalidate()
	}
}
