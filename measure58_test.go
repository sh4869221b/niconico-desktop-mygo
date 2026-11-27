//go:build mygo_measure58

package main

import (
	"context"
	"encoding/json"
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
		ctx, cancel := context.WithCancel(context.Background())
		_ = ctx
		n := &nativePlayer{stream: s, cancel: cancel, recorded: true}
		p := &playerWindow{native: n}
		b := &browser{player: p}
		var last string
		p.win = mygo.NewWindow(mygo.WindowOptions{Title: "Issue 58 / local HLS / close to finish", Width: 1000, Height: 700, Content: ui.View(func(c *ui.Context) {
			b.drawPlayer(p, c)
			state := map[string]any{"event": "sample", "position_ns": s.Position().Nanoseconds(), "ready": s.Ready(), "preparing": s.Preparing(), "buffering": s.Buffering(), "playing": s.Playing(), "failure": n.status}
			encoded, _ := json.Marshal(state)
			if string(encoded) != last {
				last = string(encoded)
				state["utc_ns"] = time.Now().UnixNano()
				_ = seek58Log.Encode(state)
			}
			c.After(50 * time.Millisecond)
		})})
		p.win.OnClosed(func() { cancel(); s.Close(); mygo.App.Quit() })
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
