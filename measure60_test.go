//go:build mygo_measure60

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// TestMain owns the OS main thread locked by MyGo's initialization. Go test
// executes individual tests on worker goroutines, which cannot run App.Run.
var measure60Run = make(chan chan error)

func TestMain(m *testing.M) {
	finished := make(chan int, 1)
	go func() { finished <- m.Run() }()
	select {
	case result := <-measure60Run:
		result <- mygo.App.Run()
		os.Exit(<-finished)
	case code := <-finished:
		os.Exit(code)
	}
}

var storage60 struct {
	sync.Mutex
	rows  []map[string]any
	delay time.Duration
}

func event60(event string, fields map[string]any) {
	fields["event"], fields["utc_ns"] = event, time.Now().UnixNano()
	storage60.Lock()
	storage60.rows = append(storage60.rows, fields)
	storage60.Unlock()
}

func observeLibrary60(stage string, action func() error) error {
	start := time.Now()
	if stage == "sync" && storage60.delay > 0 {
		event60("synthetic-delay", map[string]any{"delay_ns": storage60.delay.Nanoseconds()})
		time.Sleep(storage60.delay)
	}
	realStart := time.Now()
	err := action()
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	event60("storage", map[string]any{"stage": stage, "start_utc_ns": start.UnixNano(), "real_start_utc_ns": realStart.UnixNano(), "real_elapsed_ns": time.Since(realStart).Nanoseconds(), "total_elapsed_ns": time.Since(start).Nanoseconds(), "error": detail})
	return err
}

// Opt-in observation, not a pass/fail performance test. The selected root may
// be a genuinely slow disk/share. Injection only demonstrates UI sensitivity.
func TestMeasure60(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("run the interactive measurement on Windows")
	}
	root := os.Getenv("MYGO_STORAGE_ROOT")
	if root == "" {
		t.Fatal("set MYGO_STORAGE_ROOT to a dedicated test directory/volume")
	}
	ms := 0
	if value := os.Getenv("MYGO_STORAGE_DELAY_MS"); value != "" {
		var err error
		ms, err = strconv.Atoi(value)
		if err != nil || ms < 0 || ms > 5000 {
			t.Fatal("MYGO_STORAGE_DELAY_MS must be 0..5000")
		}
	}
	storage60.delay = time.Duration(ms) * time.Millisecond
	storage60.rows = nil
	dir, err := os.MkdirTemp(root, "mygo-issue60-")
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(os.Getenv("MYGO_STORAGE_TRACE"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	defer func() {
		encoder := json.NewEncoder(f)
		for _, row := range storage60.rows {
			if err := encoder.Encode(row); err != nil {
				t.Error(err)
				break
			}
		}
	}()
	event60("config", map[string]any{"directory": dir, "synthetic_delay_ms": ms, "go": runtime.Version(), "os": runtime.GOOS})
	b := &browser{libraryPath: filepath.Join(dir, "library.json"), showLibrary: true}
	seed := func() {
		b.data = library{}
		for i := 0; i < 500; i++ {
			b.data.Pins = append(b.data.Pins, entry{URL: fmt.Sprintf("https://www.nicovideo.jp/watch/sm%d", i+1), Title: fmt.Sprintf("測定動画 %03d", i+1), Kind: "動画"})
		}
	}
	seed()
	if err := saveLibrary(b.libraryPath, b.data); err != nil {
		t.Fatal(err)
	}
	// Calibration does not contaminate the fixture creation above.
	probe := ""
	var previous time.Time
	mygo.App.SetName("MyGo Issue 60 measurement")
	mygo.App.WhenReady(func() {
		b.win = mygo.NewWindow(mygo.WindowOptions{Title: "Issue 60 / isolated 500 pins / close to finish", Width: 1000, Height: 750, Content: ui.View(func(c *ui.Context) {
			now := time.Now()
			if !previous.IsZero() {
				event60("frame", map[string]any{"gap_ns": now.Sub(previous).Nanoseconds()})
			}
			previous = now
			ui.Column(c).Fill().Padding(8).Gap(8).Children(func() {
				ui.Text(c, fmt.Sprintf("専用プロファイル: %s / synthetic Sync delay: %d ms", dir, ms)).Wrap()
				ui.Row(c).Gap(8).Children(func() {
					if ui.Button(c, "500件を再保存").Clicked() {
						event60("save-start", map[string]any{"operation": "persist", "pins": len(b.data.Pins)})
						b.persist()
						event60("save-end", map[string]any{"operation": "persist", "pins": len(b.data.Pins)})
					}
					if ui.Button(c, "500件に戻して保存").Clicked() {
						seed()
						b.persist()
					}
					ui.TextInput(c, &probe).Label("入力応答プローブ")
				})
				if c.Shortcut(ui.Ctrl, ui.KeySpace) {
					event60("probe-handled", map[string]any{})
				}
				ui.Text(c, "Ctrl+Space は処理時刻を記録。Tab と左の登録一覧スクロールを保存と同時に操作してください。")
				ui.Row(c).Grow(1).MinHeight(0).Children(func() { b.sidebar(c) })
				ui.Text(c, b.status)
			})
			c.After(16 * time.Millisecond)
		})})
		b.win.OnClosed(func() { b.closed = true; mygo.App.Quit() })
	})
	t.Logf("isolated profile: %s", dir)
	result := make(chan error)
	measure60Run <- result
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	got, err := loadLibrary(b.libraryPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Pins) != len(b.data.Pins) {
		t.Fatalf("persisted %d pins, memory has %d", len(got.Pins), len(b.data.Pins))
	}
	event60("final", map[string]any{"pins": len(got.Pins), "path": b.libraryPath})
}
