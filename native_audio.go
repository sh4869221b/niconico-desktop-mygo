package main

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

type nativeAudioPreferences struct {
	Volume float64 `json:"volume"`
	Muted  bool    `json:"muted"`
}

func (b *browser) nativeAudioPath() string {
	if b.playbackPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(b.playbackPath), "native-audio.json")
}

func loadNativeAudio(path string) (nativeAudioPreferences, error) {
	prefs := nativeAudioPreferences{Volume: defaultNativeVolume}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return prefs, nil
	}
	if err != nil {
		return prefs, err
	}
	defer f.Close()
	var saved nativeAudioPreferences
	err = json.NewDecoder(io.LimitReader(f, 4096)).Decode(&saved)
	if err != nil || math.IsNaN(saved.Volume) || math.IsInf(saved.Volume, 0) || saved.Volume < 0 || saved.Volume > 100 {
		return prefs, errors.New("invalid native audio settings")
	}
	return saved, nil
}

func (b *browser) applyNativeAudio(s *ui.VideoPlayer) {
	s.SetVolume(b.nativeVolume / 100)
	s.SetMuted(b.nativeMuted)
}

func (b *browser) saveNativeAudio() {
	path := b.nativeAudioPath()
	if path == "" {
		b.nativeAudioNotice = "音量・ミュート設定の保存先を利用できません。"
		return
	}
	data, err := json.Marshal(nativeAudioPreferences{b.nativeVolume, b.nativeMuted})
	if err == nil {
		err = writeSettings(path, data)
	}
	if err != nil {
		b.nativeAudioNotice = "音量・ミュート設定を保存できませんでした。"
	} else {
		b.nativeAudioNotice = ""
	}
}
