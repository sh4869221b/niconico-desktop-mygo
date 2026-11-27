//go:build windows && amd64

package video

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestMediaFoundationVolume(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// A short, locally generated audio track exercises the same engine used
	// for a video stream without a network, codec download or user account.
	const rate = 16000
	data := make([]byte, 44+rate*2)
	copy(data, "RIFF\x00\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00")
	binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
	binary.LittleEndian.PutUint32(data[24:], rate)
	binary.LittleEndian.PutUint32(data[28:], rate*2)
	copy(data[32:], "\x02\x00\x10\x00data")
	binary.LittleEndian.PutUint32(data[40:], uint32(len(data)-44))
	for i := range rate {
		pcm := int16(3000 * math.Sin(2*math.Pi*440*float64(i)/rate))
		binary.LittleEndian.PutUint16(data[44+2*i:], uint16(pcm))
	}
	path := filepath.Join(t.TempDir(), "volume.wav")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	engine, err := newEngine(source{path: path, url: fileURL(path)})
	if err != nil {
		t.Fatal(err)
	}
	player := ui.NewEngineVideoPlayer(engine)
	defer player.Close()
	mf := engine.(*mfEngine)
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for mf.Duration() == 0 {
		if _, err := mf.State(); err != nil {
			t.Fatal(err)
		}
		select {
		case <-deadline.C:
			t.Fatal("Media Foundation did not load the WAV audio source")
		case <-tick.C:
		}
	}
	const getVolume = 36 // IMFMediaEngine::GetVolume
	for _, volume := range []float64{0.8, 0.23, 0, 1} {
		player.SetVolume(volume)
		if got := callGetDouble(mf.engine, getVolume); math.IsNaN(got) || math.Abs(got-volume) > 0.0001 {
			t.Fatalf("IMFMediaEngine.GetVolume() = %f after SetVolume(%f)", got, volume)
		}
	}
	t.Log("Windows Media Foundation loaded the audio track and returned all four requested volume levels")
}
