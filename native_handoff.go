package main

import "time"

// Capture before closing the native window: closing it destroys the stream.
func (p *playerWindow) nativeWebRestore(requested screenRoute, now time.Time) *webPlaybackRestore {
	n := p.native
	current := routeFor(p.target)
	if n == nil || current.kind != screenVideo || requested.kind != screenVideo || current.target != requested.target {
		return nil
	}
	id := n.videoID
	if id == "" && !numeric(current.key, 20) {
		id = current.key
	}
	if id != "" && (!watchSegment.MatchString(id) || !numeric(current.key, 20) && current.key != id) {
		return nil
	}
	position := webPlaybackPosition{seconds: current.startTime().Seconds(), paused: n.pauseRequested}
	if n.handoffKnown {
		position.seconds = n.handoffTime.Seconds()
	}
	if s := n.stream; s != nil && s.Err() == nil {
		position.paused = !s.Playing()
		if s.Ready() && !s.Preparing() {
			position.seconds = s.Position().Seconds()
		}
	}
	target := playbackRestoreURL(requested.target, position.seconds)
	return &webPlaybackRestore{video: id, routeID: current.key, target: target, position: position, token: 1, deadline: now.Add(20 * time.Second)}
}
