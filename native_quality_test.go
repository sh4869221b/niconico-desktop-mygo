package main

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"
)

func TestQualityOnlyUsesAvailableTracks(t *testing.T) {
	v := []mediaTrack{{ID: "high", IsAvailable: true, QualityLevel: 9}, {ID: "denied", QualityLevel: 99}, {ID: "low", IsAvailable: true, QualityLevel: 1}, {IsAvailable: true}, {ID: "low", IsAvailable: true, QualityLevel: 1}}
	a := []mediaTrack{{ID: "a-high", IsAvailable: true, QualityLevel: 5}, {ID: "a-low", IsAvailable: true, QualityLevel: 1}}
	for _, tc := range []struct {
		quality streamQuality
		want    [][2]string
	}{
		{qualityEconomy, [][2]string{{"low", "a-low"}}},
		{qualityHigh, [][2]string{{"high", "a-high"}}},
		{qualityAuto, [][2]string{{"low", "a-low"}, {"low", "a-high"}, {"high", "a-low"}, {"high", "a-high"}}},
	} {
		if got := qualityOutputs(v, a, tc.quality); !slices.Equal(got, tc.want) {
			t.Fatalf("quality %d: %v", tc.quality, got)
		}
	}
	if qualityOutputs(nil, a, qualityAuto) != nil {
		t.Fatal("accepted missing video tracks")
	}
}

func TestQualityAccessRequestUsesSelectedOutputs(t *testing.T) {
	var watch watchData
	watch.Video.ID, watch.Video.Title = "sm9", "fixture"
	watch.Client.WatchTrackID = "fixture-track"
	watch.Media.Domand.AccessRightKey = "fixture-key"
	watch.Media.Domand.Videos = []mediaTrack{{ID: "low", IsAvailable: true, QualityLevel: 1}, {ID: "high", IsAvailable: true, QualityLevel: 9}, {ID: "unavailable", QualityLevel: 99}}
	watch.Media.Domand.Audios = []mediaTrack{{ID: "audio", IsAvailable: true, QualityLevel: 1}}
	body, _ := json.Marshal(map[string]any{"meta": map[string]int{"status": 200}, "data": watch})
	var requested [][2]string
	prior := http.DefaultTransport
	http.DefaultTransport = streamTransportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == watchHost {
			return testResponse(200, string(body)), nil
		}
		if r.URL.Host != apiHost || r.Method != http.MethodPost {
			t.Fatalf("unexpected request %s", r.URL)
		}
		var payload struct{ Outputs [][2]string }
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		requested = payload.Outputs
		response := testResponse(201, `{"meta":{"status":201},"data":{"contentUrl":"https://delivery.domand.nicovideo.jp/fixture/master.m3u8"}}`)
		response.Header.Set("Set-Cookie", "domand_bid=fixture; Domain=.nicovideo.jp; Path=/; Secure")
		return response, nil
	})
	t.Cleanup(func() { http.DefaultTransport = prior })
	for _, quality := range []streamQuality{qualityAuto, qualityEconomy, qualityHigh} {
		if _, err := nativeStreamQuality(t.Context(), "sm9", nil, quality); err != nil {
			t.Fatal(err)
		}
		if want := qualityOutputs(watch.Media.Domand.Videos, watch.Media.Domand.Audios, quality); !slices.Equal(requested, want) {
			t.Fatalf("permission request ignored selected quality: %v", requested)
		}
	}
}

func TestQualityChangeCapturesIndependentPlaybackState(t *testing.T) {
	s, err := testNativePlaybackStream()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Seek(55500 * time.Millisecond)
	s.Pause()
	s.SetVolume(0.31)
	s.SetMuted(true)
	n := &nativePlayer{stream: s}
	state := captureNativeQuality(n, routeFor(home+"watch/sm9?from=3"))
	if state.at != 55500*time.Millisecond || !state.paused || !state.muted || state.volume != 0.31 {
		t.Fatalf("lost quality-change state: %+v", state)
	}
	if s.Position() != state.at || s.Playing() {
		t.Fatal("capture disturbed original playback")
	}
}
