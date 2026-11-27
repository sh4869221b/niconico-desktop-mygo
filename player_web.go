package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
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
	video, routeID, target string
	position               webPlaybackPosition
	token                  uint64
	deadline               time.Time
	resume                 bool
}

func (s *webPlaybackRestore) matches(route screenRoute) bool {
	return route.kind == screenVideo && (route.key == s.video || route.key == s.routeID)
}

func playbackRestoreURL(target string, seconds float64) string {
	u, err := url.Parse(target)
	if err != nil {
		return target
	}
	// The page's initial seek should agree with the restore script, which applies fractions of a second.
	query := make([]string, 0, strings.Count(u.RawQuery, "&")+2)
	if u.RawQuery != "" {
		for field := range strings.SplitSeq(u.RawQuery, "&") {
			key, _, _ := strings.Cut(field, "=")
			if name, _ := url.QueryUnescape(key); name != "from" {
				query = append(query, field)
			}
		}
	}
	u.RawQuery = strings.Join(append(query, "from="+strconv.FormatFloat(math.Floor(seconds), 'f', 0, 64)), "&")
	return u.String()
}

func (p *playerWindow) notePlayedWatch(route screenRoute, video string) {
	if route.kind != screenVideo {
		return
	}
	if video == "" || p.playedVideo != video {
		p.playedPosition = nil
	}
	if p.restoring != nil && (!p.restoring.matches(route) || video != "" && p.restoring.video != video) {
		p.restoring = nil
	}
	p.playedWatch, p.playedVideo = route, video
}

func (p *playerWindow) finishWebLoad(raw string) {
	// MyGo publishes this commit and completion only for the same accepted load.
	// The live URL may already be a search route by the completion callback.
	p.notePlayedWatch(routeFor(p.document.committedURL), "")
	p.document.finish(raw)
}

func (p *playerWindow) captureWebPosition(page *mygo.Page) {
	source, revision := page.URL(), p.document.revision
	played, id, position := p.playedWatch, p.playedVideo, p.playedPosition
	ctx, cancel := context.WithTimeout(context.Background(), webSourceInterval)
	defer cancel()
	value, err := page.EvalContext(ctx, `window.__nicoDesktopMedia?.snapshot() ?? null`)
	if err == nil && p.view != nil && p.playedWatch == played && p.playedVideo == id && p.playedPosition == position &&
		p.document.revision == revision && page.URL() == source {
		p.recordWebSnapshot(value, routeFor(source))
	}
}

func (p *playerWindow) recordWebSnapshot(value any, source screenRoute) {
	fields, ok := value.([]any)
	if !ok || len(fields) != 4 {
		return
	}
	raw, urlOK := fields[0].(string)
	video, idOK := fields[1].(string)
	seconds, timeOK := fields[2].(float64)
	paused, pauseOK := fields[3].(bool)
	if !urlOK || !idOK || !watchSegment.MatchString(video) || !timeOK || !pauseOK || !(seconds >= 0 && seconds < 1<<33) {
		return
	}
	watch := routeFor(raw)
	if watch.kind != screenVideo || watch.key != video && !numeric(watch.key, 20) ||
		source.kind == screenVideo && source.link != watch.link {
		return
	}
	// The bridge can still offer B's last sample after a same-document search.
	// Completing a restore for B takes precedence over B's transient initial state.
	p.notePlayedWatch(watch, video)
	if p.restoring == nil {
		p.playedPosition = &webPlaybackPosition{seconds: seconds, paused: paused}
	}
}

func (p *playerWindow) returnToPlayedWatch() {
	pl := p.playedWatch
	if pl.kind != screenVideo {
		return
	}
	video, position := p.playedVideo, p.playedPosition
	if p.restoring != nil && p.restoring.matches(pl) {
		video, position = p.restoring.video, &p.restoring.position
	}
	target := pl.target
	p.restoring = nil
	if video != "" && position != nil {
		target = playbackRestoreURL(target, position.seconds)
		p.restoreToken++
		p.restoring = &webPlaybackRestore{video: video, routeID: pl.key, target: target, position: *position,
			token: p.restoreToken, deadline: time.Now().Add(20 * time.Second)}
	}
	p.load(target)
}

func (p *playerWindow) expireWebRestore(now time.Time) bool {
	if p.restoring == nil || now.Before(p.restoring.deadline) {
		return false
	}
	p.restoring = nil
	return true
}

func (p *playerWindow) showWebRestoreFailure() {
	p.status = "再生状態を復元できませんでした。"
	p.win.Invalidate()
}

func (p *playerWindow) applyWebRestore(page *mygo.Page, route screenRoute) {
	state := p.restoring
	if state == nil || !state.matches(route) {
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
		p.showWebRestoreFailure()
	}
}
