package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

type playbackMode string

const (
	webPlayback    playbackMode = "webview"
	nativePlayback playbackMode = "native"
)

func loadPlayback(path string) (playbackMode, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return webPlayback, nil
	}
	if err != nil {
		return webPlayback, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil || len(b) > 4096 {
		return webPlayback, errors.New("invalid settings file")
	}
	var prefs struct {
		Player playbackMode `json:"player"`
	}
	if err = json.Unmarshal(b, &prefs); err != nil {
		return webPlayback, err
	}
	if prefs.Player != nativePlayback && prefs.Player != webPlayback {
		return webPlayback, errors.New("invalid player preference")
	}
	return prefs.Player, nil
}

func savePlayback(path string, m playbackMode) error {
	if path == "" || m != webPlayback && m != nativePlayback {
		return errors.New("invalid playback settings")
	}
	data, err := json.MarshalIndent(struct {
		Player playbackMode `json:"player"`
	}{m}, "", "  ")
	if err != nil {
		return err
	}
	return writeSettings(path, data)
}

func writeSettings(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".settings-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	cerr := f.Close()
	if err != nil {
		return err
	}
	if cerr != nil {
		return cerr
	}
	return os.Rename(f.Name(), path)
}

func (b *browser) changePlayback(m playbackMode) {
	if b.playbackBlocked {
		b.playbackNotice = "読めない再生設定は上書きしません。保存先と復旧を確認してください。"
		return
	}
	if b.playMode == m {
		return
	}
	if err := withSettingsLock(b.playbackPath, func() error {
		if _, err := loadPlayback(b.playbackPath); err != nil {
			b.playbackBlocked = true
			return err
		}
		return savePlayback(b.playbackPath, m)
	}); err != nil {
		b.playbackNotice = "再生設定を保存できませんでした。"
		if b.playbackBlocked {
			b.playbackNotice = "読めない再生設定は上書きしません。保存先と復旧を確認してください。"
		}
		return
	}
	b.playMode = m
	b.playbackNotice = "保存しました。次に動画を開くと反映されます。"
}

func (b *browser) drawPlaybackSettings(c *ui.Context) {
	ui.Column(c).FillWidth().Padding(8, 14).Gap(6).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "再生プレーヤー").Bold()
			names := []string{"公式 WebView（既定）", "独自プレーヤー（試験版）"}
			current := names[0]
			if b.playMode == nativePlayback {
				current = names[1]
			}
			if ui.Select(c, &current, names).Label("再生プレーヤー").Width(235).Changed() {
				mode := webPlayback
				if current == names[1] {
					mode = nativePlayback
				}
				b.changePlayback(mode)
			}
			if b.playbackNotice != "" {
				ui.Text(c, b.playbackNotice).Wrap()
			}
		})
		ui.Text(c, "公式：公式の弾幕・コメント投稿・対応配信を利用できます。").Wrap()
		ui.Text(c, "独自：日本語弾幕と端末内のローカルNGを利用できます。コメント投稿や一部の配信には対応していません。").Wrap()
		ui.Text(c, "どちらも別ウィンドウで開きます。ここで選ぶ既定は次に開く動画から適用します。再生中の「公式 WebView で開く」はその動画だけの切替で、既定は変わりません。").Wrap()
		b.drawNativeDisplaySettings(c)
		b.drawStorageSettings(c)
	})
}
