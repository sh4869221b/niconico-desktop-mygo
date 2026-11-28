package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
)

func loadSidebarVisibility(path string) (bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return true, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil {
		return true, err
	}
	if len(data) > 4096 {
		return true, errors.New("layout settings exceed size limit")
	}
	var value *struct {
		ShowLibrary *bool `json:"showLibrary"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return true, err
	}
	if value == nil || value.ShowLibrary == nil {
		return true, errors.New("invalid sidebar setting")
	}
	return *value.ShowLibrary, nil
}

func (b *browser) toggleLibrary() {
	b.layoutRevision++
	b.showLibrary = !b.showLibrary
	if b.layoutLoadErr != nil {
		b.reportStatus("配置設定を読み込めなかったため上書きしません。この起動中だけ変更します。")
		return
	}
	data, _ := json.Marshal(struct {
		ShowLibrary bool `json:"showLibrary"`
	}{b.showLibrary})
	if b.layoutPath == "" || writeSettings(b.layoutPath, data) != nil {
		b.reportStatus("登録欄の表示状態を保存できませんでした。")
	}
}
