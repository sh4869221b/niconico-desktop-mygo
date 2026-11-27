package main

import (
	"slices"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type streamQuality uint8

const (
	qualityAuto streamQuality = iota
	qualityEconomy
	qualityHigh
)

// Auto exposes all permitted combinations to the HLS player. Manual modes
// request one pair, ordered by the service's qualityLevel, not by its name.
func qualityOutputs(videos, audios []mediaTrack, quality streamQuality) [][2]string {
	available := func(tracks []mediaTrack) []mediaTrack {
		var out []mediaTrack
		seen := map[string]bool{}
		for _, track := range tracks {
			if track.IsAvailable && track.ID != "" && !seen[track.ID] {
				out = append(out, track)
				seen[track.ID] = true
			}
		}
		slices.SortStableFunc(out, func(a, b mediaTrack) int {
			if a.QualityLevel < b.QualityLevel {
				return -1
			}
			if a.QualityLevel > b.QualityLevel {
				return 1
			}
			return 0
		})
		return out
	}
	v, a := available(videos), available(audios)
	if len(v) == 0 || len(a) == 0 {
		return nil
	}
	if quality == qualityEconomy {
		return [][2]string{{v[0].ID, a[0].ID}}
	}
	if quality == qualityHigh {
		return [][2]string{{v[len(v)-1].ID, a[len(a)-1].ID}}
	}
	var outputs [][2]string
	for _, video := range v {
		for _, audio := range a {
			outputs = append(outputs, [2]string{video.ID, audio.ID})
		}
	}
	return outputs
}

type nativeQualityState struct {
	at            time.Duration
	paused, muted bool
	volume        float64
}

func captureNativeQuality(n *nativePlayer, route screenRoute) *nativeQualityState {
	state := &nativeQualityState{at: route.startTime(), paused: n.pauseRequested, volume: defaultNativeVolume / 100.0}
	if n.qualityRestore != nil {
		*state = *n.qualityRestore
	}
	if s := n.stream; s != nil {
		state.paused, state.muted, state.volume = !s.Playing(), s.Muted(), s.Volume()
		if s.Ready() && !s.Preparing() {
			state.at = s.Position()
		}
	}
	return state
}

func (b *browser) drawNativeQuality(c *ui.Context, p *playerWindow, n *nativePlayer) {
	names := []string{"自動", "通信量を抑える", "高画質"}
	current := names[n.quality]
	if ui.Select(c, &current, names).Label("配信画質").Tooltip("自動は利用可能な画質からHLSプレーヤーが選択します").Width(156).Disabled(n.loadingStream).Changed() {
		quality := streamQuality(slices.Index(names, current))
		state := captureNativeQuality(n, routeFor(p.target))
		go mygo.RunOnMain(func() {
			if !b.closed && b.player == p && p.native == n {
				b.loadNativePlayerQuality(p, routeFor(p.target), quality, state)
			}
		})
	}
}
