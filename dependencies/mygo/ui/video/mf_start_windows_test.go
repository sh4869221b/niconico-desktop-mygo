//go:build windows && amd64

package video

import (
	"math"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMediaFoundationPreparesInitialPicture(t *testing.T) {
	for _, fixture := range []string{"start-video.mp4", "start-av.mp4"} {
		for _, at := range []time.Duration{0, 1200 * time.Millisecond} {
			t.Run(fixture+"/"+at.String(), func(t *testing.T) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				path, err := filepath.Abs(filepath.Join("testdata", fixture))
				if err != nil {
					t.Fatal(err)
				}
				engine, err := newEngine(source{path: path, url: fileURL(path)})
				if err != nil {
					t.Fatal(err)
				}
				defer engine.Close()
				mf := engine.(*mfEngine)
				mf.PrepareStart(at)
				mf.SetMuted(false)
				const getMuted = 34 // IMFMediaEngine::GetMuted
				tick := time.NewTicker(time.Second / 60)
				defer tick.Stop()
				deadline := time.NewTimer(12 * time.Second)
				defer deadline.Stop()
				for {
					if call(mf.engine, getMuted) == 0 {
						t.Fatal("audio gate opened before receiving a frame")
					}
					pic, _, ok := mf.Frame(time.Now())
					if _, err := mf.State(); err != nil {
						if fixture == "start-av.mp4" && strings.Contains(err.Error(), "0xc00d36fa") {
							t.Skipf("no Windows audio sink: %v", err)
						}
						t.Fatal(err)
					}
					if ok {
						pix := pic.RGBA(nil)
						if len(pix) != 64*48*4 || at == 0 && pix[0] < 180 || at != 0 && pix[2] < 180 {
							t.Fatal("decoded picture did not match the initial position")
						}
						position := callGetDouble(mf.engine, engineGetCurrentTime)
						if math.IsNaN(position) || math.Abs(position-at.Seconds()) > 0.016 ||
							call(mf.engine, engineIsPaused) == 0 {
							t.Fatalf("clock moved before presentation: %fs, requested %s", position, at)
						}
						mf.PresentStart()
						if call(mf.engine, getMuted) != 0 || call(mf.engine, engineIsPaused) == 0 {
							t.Fatal("presentation did not restore sound while leaving playback paused")
						}
						return
					}
					select {
					case <-tick.C:
					case <-deadline.C:
						t.Fatalf("no target picture: phase %v, state %d, position %f, target %v",
							mf.start.phase, call(mf.engine, engineGetReadyState), callGetDouble(mf.engine, engineGetCurrentTime), mf.start.target)
					}
				}
			})
		}
	}
}
