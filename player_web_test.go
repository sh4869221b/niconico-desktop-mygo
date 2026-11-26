package main

import (
	"math"
	"net/url"
	"testing"
	"time"
)

func TestPlaybackRestoreUsesCurrentTimeAndState(t *testing.T) {
	const watch = home + "watch/sm9?from=60&ref=a%20b&%66rom=20&ref=c+d&flag#comments"
	for _, tc := range []struct {
		seconds float64
		paused  bool
		want    string
	}{{90, true, "90"}, {90.75, false, "90"}, {0, true, "0"}} {
		p := &playerWindow{}
		p.notePlayedWatch(routeFor(watch), "")
		p.recordWebSnapshot([]any{watch, "sm9", tc.seconds, tc.paused}, routeFor(watch))
		p.returnToPlayedWatch()
		if p.restoring == nil || p.restoring.position.seconds != tc.seconds || p.restoring.position.paused != tc.paused {
			t.Fatalf("restore request = %+v", p.restoring)
		}
		u, err := url.Parse(p.restoring.target)
		if err != nil || u.RawQuery != "ref=a%20b&ref=c+d&flag&from="+tc.want ||
			u.Query().Get("from") != tc.want || u.Fragment != "comments" {
			t.Fatalf("restore URL = %q", p.restoring.target)
		}
		if entry, ok := makeEntry(p.restoring.target, "動画"); !ok || entry.URL != home+"watch/sm9" {
			t.Fatalf("saved watch identity = %+v", entry)
		}
	}
	for _, sample := range []any{nil, []any{home + "watch/sm12", "sm12", 90.0, true},
		[]any{home + "search/test", "sm9", 90.0, true}, []any{watch, "sm12", 90.0, true},
		[]any{watch, "sm9", math.NaN(), true}, []any{watch, "sm9", -1.0, true}} {
		p := &playerWindow{}
		p.recordWebSnapshot(sample, routeFor(watch))
		if p.playedPosition != nil || p.playedWatch.kind == screenVideo {
			t.Fatalf("accepted invalid sample: %+v", sample)
		}
	}
}

func TestConfirmedWatchIsAvailableBeforeSourcePolling(t *testing.T) {
	a := home + "watch/sm9"
	b := home + "watch/sm12?from=7"
	search := home + "tag/test?l_range=1"
	p := &playerWindow{}
	p.document.begin()
	p.document.commit(a)
	p.finishWebLoad(a)
	p.recordWebSnapshot([]any{a, "sm9", 90.0, true}, routeFor(a))

	p.document.begin()
	p.document.commit(b)
	p.document.stopLoad()
	if p.playedWatch.target != a || p.playedPosition == nil {
		t.Fatal("a failed load displaced the last successful video")
	}

	p.document.begin()
	p.document.commit(b)
	p.finishWebLoad(search) // B completed, but the document changed the URL before the next poll.
	p.recordWebSnapshot([]any{a, "sm9", 90.0, true}, routeFor(b))
	if p.playedWatch.target != b || p.playedPosition != nil {
		t.Fatal("the new committed video inherited a stale target or position")
	}
	p.returnToPlayedWatch()
	if p.restoring != nil {
		t.Fatal("restored playback state without a snapshot for B")
	}
}

func TestWebPlaybackRecoversLastConfirmedMediaAfterSearch(t *testing.T) {
	const a = home + "watch/sm9?from=60"
	const b = home + "watch/sm38468751?playlist=related%3Asm9&from=7&ref=x+y#comments"
	search := routeFor(home + "tag/test?l_range=1")
	for _, tc := range []struct {
		name, confirmed, target string
		seconds                 float64
		paused                  bool
	}{
		{"candidate failed", a, a, 60, true},
		{"B paused", b, home + "watch/sm38468751?playlist=related%3Asm9&ref=x+y&from=85#comments", 85.25, true},
		{"B playing", b, home + "watch/sm38468751?playlist=related%3Asm9&ref=x+y&from=85#comments", 85.25, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &playerWindow{}
			p.notePlayedWatch(routeFor(a), "")
			p.recordWebSnapshot([]any{a, "sm9", 60.0, true}, routeFor(a))
			p.returnToPlayedWatch() // A still has a pending restore when B's URL becomes visible.
			p.recordWebSnapshot([]any{a, "sm9", 61.0, false}, routeFor(b))
			if p.restoring == nil || p.playedWatch.target != a || p.playedPosition.seconds != 60 {
				t.Fatal("unconfirmed B displaced A")
			}
			// The renderer has already reached search and the host has not polled B.
			p.recordWebSnapshot([]any{tc.confirmed, routeFor(tc.confirmed).key, tc.seconds, tc.paused}, search)
			if p.playedWatch.target != tc.confirmed || (p.restoring != nil) != (tc.confirmed == a) {
				t.Fatalf("confirmed watch = %+v, pending restore = %+v", p.playedWatch, p.restoring)
			}
			p.returnToPlayedWatch()
			if state := p.restoring; state == nil || state.target != tc.target ||
				state.position != (webPlaybackPosition{seconds: tc.seconds, paused: tc.paused}) {
				t.Fatalf("restore = %+v", state)
			}
		})
	}
}

func TestWebPlaybackAliasKeepsTheURLAndCanonicalPlaybackID(t *testing.T) {
	alias := routeFor(home + "watch/1173108780?playlist=a%3Ab&from=7#comments")
	canonical := routeFor(home + "watch/sm9")
	other := routeFor(home + "watch/sm38468751")
	p := &playerWindow{}
	p.recordWebSnapshot([]any{alias.target, canonical.key, 85.25, false}, alias)
	if p.playedWatch != alias || p.playedVideo != canonical.key {
		t.Fatalf("lost alias URL or resolved ID: %+v, %q", p.playedWatch, p.playedVideo)
	}
	p.returnToPlayedWatch()
	state := p.restoring
	if state == nil || !state.matches(alias) || !state.matches(canonical) || state.matches(other) ||
		state.video != canonical.key || state.target != home+"watch/1173108780?playlist=a%3Ab&from=85#comments" {
		t.Fatalf("wrong alias restore target or identity: %+v", state)
	}
	p.document.begin()
	p.document.commit(state.target)
	p.finishWebLoad(state.target)
	if p.restoring != state || p.playedPosition != nil {
		t.Fatal("the alias reload lost its pending restore or inherited playback state")
	}
	p.recordWebSnapshot([]any{state.target, canonical.key, 7.0, true}, routeFor(state.target))
	if p.restoring != state || p.playedVideo != canonical.key || p.playedPosition != nil {
		t.Fatal("the alias's transient media state replaced the pending restore")
	}
	p.recordWebSnapshot([]any{state.target, other.key, 15.0, true}, routeFor(state.target))
	if p.restoring != nil || p.playedVideo != other.key {
		t.Fatal("a newly verified alias mapping did not retire the restore for the old ID")
	}
}

func TestWebRestoreExpiresWithoutMatchingTheCurrentRoute(t *testing.T) {
	a := routeFor(home + "watch/sm9")
	b := routeFor(home + "watch/sm38468751")
	deadline := time.Unix(100, 0)
	p := &playerWindow{playedWatch: a, restoring: &webPlaybackRestore{video: a.key, deadline: deadline}}
	p.recordWebSnapshot([]any{a.target, a.key, 60.0, true}, b)
	if p.expireWebRestore(deadline.Add(-time.Second)) || p.restoring == nil {
		t.Fatal("expired before the deadline")
	}
	if !p.expireWebRestore(deadline) || p.restoring != nil || p.expireWebRestore(deadline) {
		t.Fatal("pending A restore survived its deadline")
	}
	p.recordWebSnapshot([]any{b.target, b.key, 85.25, true}, b)
	p.recordWebSnapshot([]any{b.target, b.key, 90.0, false}, b)
	if p.playedWatch != b || p.playedPosition == nil || *p.playedPosition != (webPlaybackPosition{seconds: 90}) {
		t.Fatalf("B could not keep recording: %+v, %+v", p.playedWatch, p.playedPosition)
	}
}
