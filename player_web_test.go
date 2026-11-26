package main

import (
	"math"
	"net/url"
	"testing"
)

func TestPlaybackRestoreUsesCurrentTimeAndState(t *testing.T) {
	const watch = home + "watch/sm9?from=60&ref=a&from=20#comments"
	for _, tc := range []struct {
		seconds float64
		paused  bool
		want    string
	}{{90, true, "90"}, {90.75, false, "90"}, {0, true, "0"}} {
		p := &playerWindow{}
		p.notePlayedWatch(routeFor(watch), true)
		p.playedPosition = playbackPosition([]any{"sm9", tc.seconds, tc.paused}, "sm9")
		p.returnToPlayedWatch()
		if p.restoring == nil || p.restoring.position.seconds != tc.seconds || p.restoring.position.paused != tc.paused {
			t.Fatalf("restore request = %+v", p.restoring)
		}
		u, err := url.Parse(p.restoring.target)
		if err != nil || u.Query().Get("from") != tc.want || len(u.Query()["from"]) != 1 ||
			u.Query().Get("ref") != "a" || u.Fragment != "comments" {
			t.Fatalf("restore URL = %q", p.restoring.target)
		}
		if entry, ok := makeEntry(p.restoring.target, "動画"); !ok || entry.URL != home+"watch/sm9" {
			t.Fatalf("saved watch identity = %+v", entry)
		}
	}
	for _, sample := range []any{nil, []any{"sm12", 90.0, true}, []any{"sm9", math.NaN(), true}, []any{"sm9", -1.0, true}} {
		if position := playbackPosition(sample, "sm9"); position != nil {
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
	p.playedPosition = playbackPosition([]any{"sm9", 90.0, true}, "sm9")

	p.document.begin()
	p.document.commit(b)
	p.document.stopLoad()
	if p.playedWatch.target != a || p.playedPosition == nil {
		t.Fatal("a failed load displaced the last successful video")
	}

	p.document.begin()
	p.document.commit(b)
	p.finishWebLoad(search) // B completed, but the document changed the URL before the next poll.
	if p.playedWatch.target != b || p.playedPosition != nil || playbackPosition([]any{"sm9", 90.0, true}, p.playedWatch.key) != nil {
		t.Fatal("the new committed video inherited a stale target or position")
	}
	p.returnToPlayedWatch()
	if p.restoring != nil {
		t.Fatal("restored playback state without a snapshot for B")
	}
}
