//go:build mygo_measure58

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"
)

// TestMain owns the OS main thread locked by MyGo's initialization. Go test
// executes individual tests on worker goroutines, which cannot run App.Run.
var measure58Run = make(chan chan error)

func TestMain(m *testing.M) {
	finished := make(chan int, 1)
	go func() { finished <- m.Run() }()
	select {
	case result := <-measure58Run:
		result <- mygo.App.Run()
		os.Exit(<-finished)
	case code := <-finished:
		os.Exit(code)
	}
}

var seek58Log *json.Encoder

func traceSeek58(target time.Duration) {
	if seek58Log != nil {
		_ = seek58Log.Encode(map[string]any{"event": "seek", "utc_ns": time.Now().UnixNano(), "target_ns": target.Nanoseconds()})
	}
}

func traceNativeFailure58(message string) {
	if seek58Log != nil {
		_ = seek58Log.Encode(map[string]any{"event": "native-stream-failure", "utc_ns": time.Now().UnixNano(), "message": message})
	}
}

// This opt-in interactive test opens the production native player view. It
// records observations; closing the window does not assert that #58 is fixed.
func TestMeasure58(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Media Foundation measurement needs Windows")
	}
	u, err := url.Parse(os.Getenv("MYGO_HLS_URL"))
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil {
		t.Fatal("set MYGO_HLS_URL to the loopback fixture")
	}
	f, err := os.OpenFile(os.Getenv("MYGO_HLS_TRACE"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	seek58Log = json.NewEncoder(f)
	defer func() { seek58Log = nil }()
	_ = seek58Log.Encode(map[string]any{"event": "config", "utc_ns": time.Now().UnixNano(), "go_version": runtime.Version()})
	mygo.App.SetName("MyGo Issue 58 measurement")
	var openErr error
	mygo.App.WhenReady(func() {
		s, err := video.OpenURL(u.String())
		if err != nil {
			openErr = err
			mygo.App.Quit()
			return
		}
		s.SetVolume(0)
		s.PrepareStart(0)
		_, cancel := context.WithCancel(context.Background())
		n := &nativePlayer{stream: s, cancel: cancel, recorded: true}
		p := &playerWindow{native: n}
		b := &browser{player: p}
		var last string
		var startupReported bool
		var trial int
		var inTrial bool
		const windowTitle = "Issue 58 / local HLS"
		mark := func(event string) {
			_ = seek58Log.Encode(map[string]any{
				"event": event, "utc_ns": time.Now().UnixNano(), "trial": trial,
				"paused": s.Paused(), "ready": s.Ready() && !s.Preparing(),
			})
		}
		p.win = mygo.NewWindow(mygo.WindowOptions{Title: windowTitle + " / F8 start, F9 end", Width: 1000, Height: 700, Content: ui.View(func(c *ui.Context) {
			if c.Shortcut(0, ui.KeyF8) && !c.ShortcutRepeated() && !inTrial {
				trial++
				inTrial = true
				mark("trial-start")
				p.win.SetTitle(fmt.Sprintf("%s / trial %d recording / F9 end", windowTitle, trial))
			}
			b.drawPlayer(p, c)
			if !startupReported && (n.status != "" || s.Ready() && !s.Preparing()) {
				visible, _ := c.WindowVisibleTime()
				_ = seek58Log.Encode(map[string]any{"event": "startup-result", "utc_ns": time.Now().UnixNano(),
					"window_visible_ns": visible.Nanoseconds(), "ready": s.Ready(), "preparing": s.Preparing(),
					"failure": n.status})
				startupReported = true
			}
			state := map[string]any{"event": "sample", "position_ns": s.Position().Nanoseconds(), "ready": s.Ready(), "preparing": s.Preparing(), "buffering": s.Buffering(), "playing": s.Playing(), "paused": s.Paused(), "failure": n.status}
			if position, confirmed := s.ConfirmedPosition(); confirmed {
				state["confirmed_position_ns"] = position.Nanoseconds()
			}
			encoded, _ := json.Marshal(state)
			if string(encoded) != last {
				last = string(encoded)
				state["utc_ns"] = time.Now().UnixNano()
				_ = seek58Log.Encode(state)
			}
			if c.Shortcut(0, ui.KeyF9) && !c.ShortcutRepeated() && inTrial {
				mark("trial-end")
				inTrial = false
				p.win.SetTitle(fmt.Sprintf("%s / trial %d recorded / F8 start", windowTitle, trial))
			}
			c.After(50 * time.Millisecond)
		})})
		p.win.OnClosed(func() {
			if inTrial {
				mark("trial-interrupted")
			}
			cancel()
			s.Close()
			mygo.App.Quit()
		})
	})
	result := make(chan error)
	measure58Run <- result
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if openErr != nil {
		t.Fatal(openErr)
	}
}
