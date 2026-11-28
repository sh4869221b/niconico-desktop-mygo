package main

import "time"

const librarySaveQueueCapacity = 16
const libraryCloseWait = 2 * time.Second

type librarySaveRequest struct {
	label      string
	enqueuedAt time.Time
	// An edit owns its inputs and modifies only this transaction's snapshot.
	// It can run more than once when a transaction fails and is retried.
	edit func(*library) bool
}

type librarySaveResult struct {
	request  librarySaveRequest
	library  library
	changed  bool
	err      error
	finished time.Time
}

// librarySaveQueue processes accepted edits in order. One producer owns
// submission, retry and shutdown. The worker alone runs the edits.
type librarySaveQueue struct {
	requests chan librarySaveRequest
	results  chan librarySaveResult
	retry    chan struct{}
	stop     chan struct{}
}

func newLibrarySaveQueue(path string) *librarySaveQueue {
	q := &librarySaveQueue{
		requests: make(chan librarySaveRequest, librarySaveQueueCapacity),
		results:  make(chan librarySaveResult, librarySaveQueueCapacity),
		retry:    make(chan struct{}, 1),
		stop:     make(chan struct{}),
	}
	go q.run(path)
	return q
}

func (q *librarySaveQueue) run(path string) {
	for {
		var request librarySaveRequest
		select {
		case <-q.stop:
			return
		case request = <-q.requests:
		}
		for {
			latest, changed, err := persistLibraryEdits(path, nil, request.edit)
			result := librarySaveResult{request: request, library: latest, changed: changed, err: err, finished: time.Now()}
			select {
			case <-q.stop:
				return
			case q.results <- result:
			}
			if err == nil {
				break
			}
			select {
			case <-q.stop:
				return
			case <-q.retry:
			}
		}
	}
}

// librarySaveSession belongs to the UI thread. A close request waits for
// acknowledgments without blocking that thread. A timed-out or failed close
// leaves the window and its unsaved edits available for another attempt.
type librarySaveSession struct {
	queue   *librarySaveQueue
	pending int
	failed  bool
	closeBy time.Time
	message string
}

func (s *librarySaveSession) submit(request librarySaveRequest) bool {
	if !s.closeBy.IsZero() {
		return false
	}
	if s.pending >= librarySaveQueueCapacity {
		if !s.failed {
			s.message = "保存待ちの操作が上限に達しました。この操作は受け付けていません。"
		}
		return false
	}
	s.queue.requests <- request
	s.pending++
	if !s.failed {
		s.message = "保存を待っています…"
	}
	return true
}

func (s *librarySaveSession) receive(result librarySaveResult) {
	if result.err != nil {
		s.failed = true
		s.closeBy = time.Time{}
		s.message = "保存できませんでした。操作は保留されています。保存を再試行してください。"
		return
	}
	s.pending--
	if s.pending == 0 {
		s.message = "保存しました。"
	}
}

func (s *librarySaveSession) retry() {
	if s.failed {
		s.queue.retry <- struct{}{}
		s.failed = false
		s.message = "保存を再試行しています…"
	}
}

func (s *librarySaveSession) requestClose(now time.Time) bool {
	if s.pending == 0 {
		return true
	}
	if !s.failed && s.closeBy.IsZero() {
		s.closeBy = now.Add(libraryCloseWait)
		s.message = "保存が終わるまで待っています…"
	}
	return false
}

func (s *librarySaveSession) finishClose(now time.Time) bool {
	if s.closeBy.IsZero() {
		return false
	}
	if s.pending == 0 {
		s.closeBy = time.Time{}
		return true
	}
	if !now.Before(s.closeBy) {
		s.closeBy = time.Time{}
		s.message = "保存が続いています。保存完了を確認してから閉じてください。"
	}
	return false
}
