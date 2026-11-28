//go:build mygo_measure60

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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

func drawMeasurement60(c *ui.Context, pins []entry, delay int, probe *string, resave, reset func(), observe func(ui.InputEvent)) {
	ui.Column(c).Fill().Padding(8).Gap(8).ObserveInput(observe).Children(func() {
		ui.Column(c).FillWidth().Shrink(0).Gap(8).Children(func() {
			ui.Text(c, fmt.Sprintf("隔離プロファイル: %d 件 / 合成 Sync 遅延: %d ms", len(pins), delay)).Wrap()
			ui.Row(c).FillWidth().Wrap().Gap(8).Children(func() {
				if ui.Button(c, "500件を再保存").Clicked() {
					resave()
				}
				if ui.Button(c, "500件に戻して保存").Clicked() {
					reset()
				}
				ui.TextInput(c, probe).Label("入力応答プローブ")
			})
			ui.Text(c, "保存と同時に Tab で移動し、下の登録一覧をスクロールしてください。").Wrap()
		})
		scroll := ui.Scroll(c.Key("measurement-library")).FillWidth().Grow(1).MinHeight(0)
		scroll.HandleInput(func(ev ui.InputEvent) bool {
			if ev.Kind == ui.InputScroll {
				observe(ev)
			}
			return false // the actual scroll container still receives the wheel
		})
		scroll.Children(func() {
			ui.Column(c).FillWidth().Gap(4).Children(func() {
				for _, pin := range pins {
					// Inert fixture rows keep input inside the measurement window.
					ui.Text(c.Key(pin.URL), pin.Title).Padding(5).Focusable()
				}
			})
		})
	})
}

func TestMeasurement60ControlsAreAboveTheFixtureList(t *testing.T) {
	pins := make([]entry, 500)
	for i := range pins {
		pins[i] = entry{URL: fmt.Sprint(i), Title: fmt.Sprintf("測定動画 %03d", i+1)}
	}
	for _, size := range [][2]int{{1000, 750}, {720, 480}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			var probe string
			var saves, resets, tabs, scrolls int
			v := ui.NewTester(func(c *ui.Context) {
				drawMeasurement60(c, pins, 250, &probe, func() { saves++ }, func() { resets++ }, func(ev ui.InputEvent) {
					if ev.Kind == ui.InputKeyDown && ev.Key == ui.KeyTab {
						tabs++
					}
					if ev.Kind == ui.InputScroll {
						scrolls++
					}
				})
			}, size[0], size[1])
			click := func(label string) {
				t.Helper()
				if err := v.Click(label); err != nil {
					t.Fatal(err)
				}
			}
			click("500件を再保存")
			click("500件に戻して保存")
			v.Key(0, ui.KeyTab)
			if !v.Focused("入力応答プローブ") {
				t.Fatal("Tab did not reach the control above the list")
			}
			first, ok := v.Access(pins[0].Title)
			if !ok || first.Offscreen {
				t.Fatal("fixture is outside its viewport")
			}
			v.Scroll(first.Bounds.X+10, first.Bounds.Y+5, 0, 500)
			after, _ := v.Access(pins[0].Title)
			if !after.Offscreen {
				t.Fatal("fixture viewport did not scroll")
			}
			click("500件を再保存")
			if saves != 2 || resets != 1 || tabs == 0 || scrolls == 0 {
				t.Fatalf("input missed controls: saves=%d resets=%d tabs=%d scrolls=%d", saves, resets, tabs, scrolls)
			}
		})
	}
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
	var fixture []entry
	for i := 0; i < 500; i++ {
		fixture = append(fixture, entry{URL: fmt.Sprintf("https://www.nicovideo.jp/watch/sm%d", i+1), Title: fmt.Sprintf("測定動画 %03d", i+1), Kind: "動画"})
	}
	b := &browser{libraryPath: filepath.Join(dir, "library.json"), data: library{Pins: slices.Clone(fixture)}}
	event60("fixture-start", map[string]any{"pins": len(fixture)})
	if err := saveLibrary(b.libraryPath, b.data); err != nil {
		t.Fatal(err)
	}
	event60("fixture-end", map[string]any{"pins": len(fixture)})
	save := func(operation string, edit func(*library) bool) {
		start := time.Now()
		event60("save-start", map[string]any{"operation": operation, "pins": len(b.data.Pins)})
		changed, saved := b.changeLibrary(edit)
		event60("save-end", map[string]any{"operation": operation, "pins": len(b.data.Pins), "elapsed_ns": time.Since(start).Nanoseconds(), "changed": changed, "saved": saved})
		if !saved {
			t.Error("the measured transaction did not persist the fixture")
		}
	}
	// UI probes start after the seed has been durably written.
	probe := ""
	var previous time.Time
	mygo.App.SetName("MyGo Issue 60 measurement")
	mygo.App.WhenReady(func() {
		b.win = mygo.NewWindow(mygo.WindowOptions{Title: "Issue 60 / isolated 500 pins / close to finish", Width: 1000, Height: 750, Content: ui.View(func(c *ui.Context) {
			now := time.Now()
			if !previous.IsZero() {
				event60("frame", map[string]any{"gap_ns": now.Sub(previous).Nanoseconds()})
			} else {
				event60("ui-ready", map[string]any{})
			}
			previous = now
			if c.Shortcut(0, ui.KeyF8) {
				event60("probe-handled", map[string]any{})
			}
			drawMeasurement60(c, b.data.Pins, ms, &probe, func() {
				save("resave", func(*library) bool { return true })
			}, func() {
				save("reset", func(data *library) bool {
					data.Pins = slices.Clone(fixture)
					return true
				})
			}, func(ev ui.InputEvent) {
				switch {
				case ev.Kind == ui.InputKeyDown && ev.Key == ui.KeyTab:
					event60("tab-delivered", map[string]any{"repeat": ev.Repeat})
				case ev.Kind == ui.InputScroll:
					event60("scroll-delivered", map[string]any{"dx": ev.DX, "dy": ev.DY})
				}
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
	if !slices.Equal(got.Pins, b.data.Pins) || !slices.Equal(got.History, b.data.History) {
		t.Fatalf("persisted library differs from the measurement UI (%d versus %d pins)", len(got.Pins), len(b.data.Pins))
	}
	counts := map[string]int{"saved": 0, "probe-handled": 0, "tab-delivered": 0, "scroll-delivered": 0}
	storage60.Lock()
	for _, row := range storage60.rows {
		name, _ := row["event"].(string)
		if name == "save-end" && row["saved"] == true {
			counts["saved"]++
		} else if _, tracked := counts[name]; tracked {
			counts[name]++
		}
	}
	storage60.Unlock()
	event60("final", map[string]any{"pins": len(got.Pins), "events": counts})
}
