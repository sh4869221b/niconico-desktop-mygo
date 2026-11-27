package ui

import "time"

// WindowHidden reports whether nothing of the window shows on screen, as
// the web's document.hidden: it is minimized, hidden, on another space or
// desktop, or covered by other windows, as far as the system tells (see
// the platform differences). A view that shows a clock, a price or what
// plays stops working while it is hidden, and refreshes as it shows
// again: the view builds once as it hides, and again as it shows, from
// the first time it asks. Frames that only move what shows wait while it
// is hidden anyway.
//
//	app.ticker.SetPaused(c.WindowHidden())
func (c *context) WindowHidden() bool {
	c.rt.hiddenAsked = true
	return c.rt.host.occluded()
}

// WindowVisibleTime reports the elapsed time while the surface could show
// video frames, and whether it is currently hidden. The clock starts on the
// first call. As with WindowHidden, visibility changes rebuild the view.
func (c *Context) WindowVisibleTime() (elapsed time.Duration, hidden bool) {
	raw := c.build()
	if raw == nil {
		return 0, true
	}
	rt := raw.rt
	rt.hiddenAsked = true
	rt.updateVisibleTime()
	return rt.visibleTime.elapsed, rt.visibleTime.hidden
}

type visibleClock struct {
	updated time.Time
	elapsed time.Duration
	hidden  bool
}

func (rt *engine) updateVisibleTime() {
	clock, now := &rt.visibleTime, rt.now()
	if !clock.updated.IsZero() && !clock.hidden {
		clock.elapsed += now.Sub(clock.updated)
	}
	clock.updated, clock.hidden = now, rt.videoOccluded()
}

// videoOccluded checks the surface itself. The frame that readies a new
// window may draw even when the surface cannot yet show the first picture.
func (rt *engine) videoOccluded() bool {
	if h, ok := rt.host.(*windowHost); ok {
		return h.surfaceOccluded()
	}
	return rt.host.occluded()
}
