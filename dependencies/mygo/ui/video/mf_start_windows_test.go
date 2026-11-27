//go:build windows && amd64

package video

import (
	"math"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/internal/media"
	"github.com/egoist/mygo/internal/wait"
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
				var pic media.Picture
				if !wait.Poll(func() bool {
					// Poll on the test's locked OS thread for Media Foundation and Direct3D.
					if call(mf.engine, engineGetMuted) == 0 {
						t.Fatal("audio gate opened before receiving a frame")
					}
					var ready bool
					pic, _, ready = mf.Frame(time.Now())
					if _, err := mf.State(); err != nil {
						if fixture == "start-av.mp4" && strings.Contains(err.Error(), "0xc00d36fa") {
							t.Skipf("no Windows audio sink: %v", err)
						}
						t.Fatal(err)
					}
					return ready
				}) {
					t.Fatalf("no target picture after %v: phase %v, state %d, position %f, target %v",
						wait.Guard, mf.start.phase, call(mf.engine, engineGetReadyState), callGetDouble(mf.engine, engineGetCurrentTime), mf.start.target)
				}
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
				if call(mf.engine, engineGetMuted) != 0 || call(mf.engine, engineIsPaused) == 0 {
					t.Fatal("presentation did not restore sound while leaving playback paused")
				}
			})
		}
	}
}

func TestMediaFoundationStartRejectsStalePictureAndMovingClock(t *testing.T) {
	at := 2 * time.Second
	if !mfStartMatches(at, 2, 19_900_000) ||
		mfStartMatches(at, 2, 0) ||
		mfStartMatches(at, 2, 25_000_000) ||
		mfStartMatches(at, 2.1, 20_000_000) {
		t.Fatal("startup frame must belong to the requested paused position")
	}
}
