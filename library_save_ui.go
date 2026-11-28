package main

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (b *browser) startLibrarySaves(dispatch func(func())) {
	s := &librarySaveSession{}
	b.librarySaves = s
	s.queue = newLibrarySaveQueue(b.libraryPath, func() {
		dispatch(func() { b.receiveLibrarySave(s, <-s.queue.results) })
	})
}

// mutateLibrary publishes state only after the worker has reread and saved it.
// The synchronous path also serves the isolated baseline measurement.
func (b *browser) mutateLibrary(edit func(*library) bool, complete func(changed, saved bool)) bool {
	if b.librarySaves == nil {
		changed, saved := b.changeLibrary(edit)
		if complete != nil {
			complete(changed, saved)
		}
		return true
	}
	return b.submitLibrarySave(librarySaveRequest{edit: edit, complete: complete})
}

func (b *browser) submitLibrarySave(request librarySaveRequest) bool {
	if b.closed || b.librarySessionEnding {
		return false
	}
	accepted := b.librarySaves.submit(request)
	if b.win != nil {
		b.win.Invalidate()
	}
	return accepted
}

func (b *browser) receiveLibrarySave(s *librarySaveSession, result librarySaveResult) {
	if b.closed || b.librarySaves != s {
		return
	}
	wasFailed := s.failed
	s.receive(result)
	if result.err != nil {
		if !wasFailed {
			b.statusAnnouncements = append(b.statusAnnouncements, s.message)
		}
	} else {
		if result.request.edit != nil {
			b.data = result.library
		}
		if complete := result.request.complete; complete != nil {
			complete(result.changed, true)
		}
	}
	b.finishLibraryClose(s)
	if b.win != nil && !b.closed {
		b.win.Invalidate()
	}
}

func (b *browser) finishLibraryClose(s *librarySaveSession) {
	if s.finishClose(time.Now()) {
		b.libraryCloseReady = true
		if b.win != nil {
			b.win.Close()
		}
	}
}

func (b *browser) requestLibraryClose() bool {
	s := b.librarySaves
	if s == nil || b.closed || b.libraryCloseReady || b.librarySessionEnding {
		return true
	}
	if b.libraryClosePreparing {
		return false
	}
	if !s.failed && s.closeBy.IsZero() {
		b.libraryClosePreparing = true
		defer func() { b.libraryClosePreparing = false }()
		if p := b.player; p != nil && !b.flushPlayerProgress(p) {
			return false
		}
		if s.requestClose(time.Now()) {
			b.libraryCloseReady = true
			return true
		}
		deadline := s.closeBy
		if b.win != nil {
			time.AfterFunc(time.Until(deadline), func() {
				mygo.RunOnMain(func() {
					if !b.closed && s.closeBy == deadline {
						b.finishLibraryClose(s)
						if !b.closed {
							b.win.Invalidate()
						}
					}
				})
			})
		}
	}
	if b.win != nil {
		b.win.Invalidate()
	}
	return false
}

func (b *browser) drawLibrarySaveStatus(c *ui.Context) {
	s := b.librarySaves
	if s == nil || s.pending == 0 {
		return
	}
	ui.Text(c, s.message).Role(ui.RoleStatus).Wrap()
	if s.failed && ui.Button(c, "保存を再試行").Clicked() {
		s.retry()
		c.Invalidate()
	}
}
