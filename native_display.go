package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

type nativeDisplayPreferences struct {
	Danmaku      string `json:"danmaku"`
	HideComments bool   `json:"hideComments"`
	ShowOverview bool   `json:"showOverview"`
}

func (b *browser) nativeDisplayPath() string {
	if b.playbackPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(b.playbackPath), "native-display.json")
}

func loadNativeDisplay(path string) (nativeDisplayPreferences, error) {
	var prefs nativeDisplayPreferences
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return prefs, nil
	}
	if err != nil {
		return prefs, err
	}
	defer f.Close()
	err = json.NewDecoder(io.LimitReader(f, 4096)).Decode(&prefs)
	if err != nil || prefs.Danmaku != "" && prefs.Danmaku != "system" && prefs.Danmaku != "show" && prefs.Danmaku != "hide" {
		return nativeDisplayPreferences{}, errors.New("invalid display preferences")
	}
	return prefs, nil
}

func (b *browser) applyNativeDisplay(n *nativePlayer, reduceMotion bool) {
	n.showComments = b.nativeDisplay.Danmaku == "show" || b.nativeDisplay.Danmaku != "hide" && !reduceMotion
	n.hideCommentPanel, n.showOverview = b.nativeDisplay.HideComments, b.nativeDisplay.ShowOverview
}

func (b *browser) saveNativeDisplay() {
	data, err := json.Marshal(b.nativeDisplay)
	if err == nil && b.nativeDisplayPath() != "" {
		err = writeSettings(b.nativeDisplayPath(), data)
	} else {
		err = errors.New("no settings path")
	}
	if err != nil {
		b.nativeDisplayNotice = "表示設定を保存できませんでした。"
	} else {
		b.nativeDisplayNotice = ""
	}
}

func (b *browser) drawNativeDisplaySettings(c *ui.Context) {
	names := []string{"OSの動きを減らす設定に合わせる", "表示", "非表示"}
	current := names[0]
	if b.nativeDisplay.Danmaku == "show" {
		current = names[1]
	} else if b.nativeDisplay.Danmaku == "hide" {
		current = names[2]
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		ui.Text(c, "次の動画の弾幕")
		if ui.Select(c, &current, names).Label("弾幕の初期表示").Width(290).Changed() {
			b.nativeDisplay.Danmaku = "system"
			if current == names[1] {
				b.nativeDisplay.Danmaku = "show"
			} else if current == names[2] {
				b.nativeDisplay.Danmaku = "hide"
			}
			b.saveNativeDisplay()
		}
	})
	if b.nativeDisplayNotice != "" {
		ui.Text(c, b.nativeDisplayNotice).Wrap()
	}
}
