//go:build windows && (amd64 || arm64)

package video

import (
	"errors"
	"math"
	"time"
	"unsafe"

	"github.com/egoist/mygo/internal/media"
)

type mfStartPhase uint8

const (
	startLoad mfStartPhase = iota
	startPaused
	startSeeking
	startPreroll
	startReady
)

type mfStart struct {
	target     time.Duration
	phase      mfStartPhase
	seekBefore uint64
	prerolled  bool
	stepped    bool
	reseeked   bool
}

func (e *mfEngine) PrepareStart(at time.Duration) {
	e.start = &mfStart{target: at}
	e.SetMuted(e.userMuted)
	const preloadAutomatic = 4 // MF_MEDIA_ENGINE_PRELOAD_AUTOMATIC
	call(e.engine, engineSetPreload, preloadAutomatic)
}

func (e *mfEngine) PresentStart() {
	if e.start == nil || e.start.phase != startReady {
		return
	}
	e.start = nil
	e.SetMuted(e.userMuted)
}

func (e *mfEngine) startError(reason string) (media.Picture, bool, bool) {
	call(e.engine, enginePause)
	e.mu.Lock()
	e.state, e.err = media.Failed, errors.New(reason)
	e.mu.Unlock()
	return media.Picture{}, false, false
}

func (e *mfEngine) seekStart(s *mfStart) bool {
	e.mu.Lock()
	s.seekBefore, e.landed = e.seekEvents, false
	e.mu.Unlock()
	s.phase = startSeeking
	return !failed(callSetDouble(e.engine, engineSetCurrentTime, s.target.Seconds()))
}

func (e *mfEngine) playPreroll(s *mfStart) bool {
	if call(e.engine, engineGetMuted) == 0 &&
		(failed(call(e.engine, engineSetMuted, 1)) || call(e.engine, engineGetMuted) == 0) {
		return false
	}
	s.prerolled, s.phase = true, startPreroll
	return !failed(call(e.engine, enginePlay))
}

func (e *mfEngine) startFrame() (media.Picture, bool, bool) {
	s := e.start
	if s.phase == startReady {
		return e.picture(false)
	}
	const haveMetadata, haveCurrentData = 1, 2
	ready := call(e.engine, engineGetReadyState)
	if state, _ := e.State(); state == media.Failed {
		return media.Picture{}, false, false
	}
	if ready < haveMetadata {
		return media.Picture{}, false, false
	}
	if call(e.engine, engineHasVideo) == 0 && ready >= haveCurrentData {
		return e.startError("Media Foundation found no video track")
	}
	if s.phase == startLoad {
		if duration := e.Duration(); duration > 0 && s.target >= duration {
			s.target = max(0, duration-time.Second/30)
		}
		s.phase = startPaused
		if s.target > 0 {
			if !e.seekStart(s) {
				return e.startError("Media Foundation could not seek to the initial position")
			}
			return media.Picture{}, false, false
		}
	}
	var pts int64
	fresh := call(e.engine, engineOnVideoStreamTick, uintptr(unsafe.Pointer(&pts))) == 0
	seeking := call(e.engine, engineIsSeeking) != 0
	if s.phase == startPreroll {
		if !fresh || seeking {
			return media.Picture{}, false, false
		}
		if failed(call(e.engine, enginePause)) {
			return e.startError("Media Foundation could not stop preroll")
		}
		// Avoid seeking to the same position if preroll produced the target
		// picture before the clock moved. A no-op seek may never notify us.
		if e.captureStart(s, pts) {
			return e.picture(true)
		}
		// Preroll advanced beyond the target. Restore it while still muted.
		if !e.seekStart(s) {
			return e.startError("Media Foundation could not restore the initial position")
		}
		return media.Picture{}, false, false
	}
	// The target frame can arrive before the async seeked notification.
	// Transfer it on this tick once the seek, clock and frame agree.
	if fresh && !seeking && e.captureStart(s, pts) {
		return e.picture(true)
	}
	if s.phase == startSeeking {
		e.mu.Lock()
		seeked := e.seekEvents != s.seekBefore
		e.mu.Unlock()
		if seeked && !seeking {
			s.phase = startPaused
		} else if s.prerolled {
			return media.Picture{}, false, false
		}
	}
	if !s.prerolled {
		if !e.playPreroll(s) {
			return e.startError("Media Foundation could not prepare the initial frame")
		}
	} else if !fresh && !s.stepped && ready >= haveCurrentData && e.ex != 0 && call(e.engine, engineIsPaused) != 0 {
		// Some engines need a paused frame-step after a seek. Its result
		// still has to match both the requested clock and the frame time.
		s.stepped = true
		call(e.ex, engineExFrameStep, 1)
	} else if fresh && s.stepped && !s.reseeked {
		// FrameStep can advance the clock. With the decoder now warm, seek
		// again and require a new picture from the initial position.
		s.reseeked = true
		if !e.seekStart(s) {
			return e.startError("Media Foundation could not restore the initial position after stepping")
		}
	}
	return media.Picture{}, false, false
}

func (e *mfEngine) captureStart(s *mfStart, pts int64) bool {
	if call(e.engine, engineIsSeeking) != 0 || call(e.engine, engineIsPaused) == 0 ||
		!s.acceptPicture(callGetDouble(e.engine, engineGetCurrentTime), pts, e.captureFrame) {
		return false
	}
	e.mu.Lock()
	e.landed = false
	e.mu.Unlock()
	return true
}

func (s *mfStart) acceptPicture(clock float64, pts int64, transfer func() bool) bool {
	if !mfStartMatches(s.target, clock, pts) || !transfer() {
		return false
	}
	s.phase = startReady
	return true
}

func mfStartMatches(target time.Duration, seconds float64, pts int64) bool {
	// GetCurrentTime is the paused clock; the decoded frame can begin
	// before it (the requested time can lie inside that frame).
	nearClock := math.Abs(seconds-target.Seconds()) <= 0.016
	offset := float64(pts)/10_000_000 - target.Seconds()
	return nearClock && pts >= 0 && offset >= -0.25 && offset <= 0.10
}
