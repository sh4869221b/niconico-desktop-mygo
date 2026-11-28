package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"
)

func loadHistoryRecording(path string) (bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil {
		return true, err
	}
	var value *struct {
		RecordHistory *bool `json:"recordHistory"`
	}
	if len(data) > 4096 || json.Unmarshal(data, &value) != nil || value == nil || value.RecordHistory == nil {
		return true, errors.New("invalid history preference")
	}
	return !*value.RecordHistory, nil
}
func (b *browser) canRecordHistory() bool {
	// Called under the library transaction lock when writing history.
	if b.historySettingsErr != nil {
		return false
	}
	if b.historySettingsPath == "" {
		return !b.historyDisabled
	}
	disabled, err := loadHistoryRecording(b.historySettingsPath)
	return err == nil && !disabled
}

func (b *browser) setHistoryRecording(record bool) {
	if b.historySettingsErr != nil {
		b.historyNotice = "履歴設定を読み込めないため上書きしません。元ファイルを確認してください。"
		return
	}
	data, _ := json.Marshal(struct {
		RecordHistory bool `json:"recordHistory"`
	}{record})
	if err := withSettingsLock(b.libraryPath, func() error {
		if _, err := loadHistoryRecording(b.historySettingsPath); err != nil {
			return err
		}
		return writeSettings(b.historySettingsPath, data)
	}); err != nil {
		b.historyNotice = "履歴設定を保存できませんでした。"
		return
	}
	b.historyDisabled = !record
	b.historyNotice = "記録設定を保存しました。保存済みの履歴は消去していません。"
}
func historyInPeriod(items []entry, period string, now time.Time) []entry {
	days := map[string]time.Duration{"過去24時間": 24 * time.Hour, "過去7日間": 7 * 24 * time.Hour, "過去30日間": 30 * 24 * time.Hour}
	out := []entry{}
	span, known := days[period]
	for _, e := range items {
		if period == "すべて" || known && !e.LastViewedAt.IsZero() && !e.LastViewedAt.Before(now.Add(-span)) && !e.LastViewedAt.After(now) {
			out = append(out, e)
		}
	}
	return out
}
func (b *browser) eraseSelectedHistory() {
	if b.storeWarning != "" {
		b.historyNotice = b.storeWarning
		return
	}
	selected := make(map[string]time.Time, len(b.historyErasePending))
	for _, item := range b.historyErasePending {
		selected[item.URL] = item.LastViewedAt
	}
	var removed []string
	changed, saved := b.changeLibrary(func(data *library) bool {
		removed = removed[:0]
		data.History = slices.DeleteFunc(data.History, func(e entry) bool {
			viewedAt, ok := selected[e.URL]
			if !ok || !e.LastViewedAt.Equal(viewedAt) {
				return false
			}
			removed = append(removed, e.URL)
			return true
		})
		return len(removed) != 0
	})
	if changed {
		if p := b.player; p != nil {
			if p.historySuppressed == nil {
				p.historySuppressed = make(map[string]bool)
			}
			for _, raw := range removed {
				p.historySuppressed[raw] = true
			}
		}
		if b.undoRemoval != nil && b.undoRemoval.history {
			b.undoRemoval = nil
		}
	}
	b.historyErasePending = nil
	if saved {
		b.historyNotice = fmt.Sprintf("視聴履歴を %d 件消去しました。登録済み項目は保持しています。", len(removed))
	} else {
		b.historyNotice = b.status
	}
}
func (b *browser) drawHistorySettings(c *ui.Context) {
	ui.Text(c, "視聴履歴").Bold()
	record := !b.historyDisabled
	if ui.Checkbox(c, &record, "今後の履歴を記録").Changed() {
		b.setHistoryRecording(record)
	}
	if b.historySettingsErr != nil {
		ui.Text(c, "履歴設定を読み込めません。今後の記録を停止し、元ファイルを保護しています。").Wrap()
	}
	period := b.historyPeriod
	if period == "" {
		period = "過去24時間"
	}
	if ui.Select(c, &period, []string{"過去24時間", "過去7日間", "過去30日間", "すべて"}).Label("履歴を消去する期間").Width(180).Changed() {
		b.historyPeriod = period
		b.historyErasePending = nil
	}
	selected := historyInPeriod(b.data.History, period, time.Now())
	ui.Row(c).Wrap().Gap(8).Children(func() {
		ui.Text(c, fmt.Sprintf("消去対象: %d 件", len(selected)))
		if ui.Button(c, "選んだ期間の履歴を消去").Disabled(len(selected) == 0 || b.storeWarning != "").Clicked() {
			b.historyErasePending = slices.Clone(selected)
		}
	})
	ui.Text(c, "記録の停止と保存済み履歴の消去は別操作です。日時不明の既存履歴は「すべて」でのみ消去します。").Wrap()
	if len(b.historyErasePending) > 0 {
		ui.Row(c).Wrap().Gap(8).Children(func() {
			if ui.Button(c, fmt.Sprintf("消去を確定（%d件）", len(b.historyErasePending))).Clicked() {
				b.eraseSelectedHistory()
			}
			if ui.Button(c, "消去をキャンセル").Clicked() {
				b.historyErasePending = nil
			}
		})
	}
	if b.historyNotice != "" {
		ui.Text(c, b.historyNotice).Wrap().Role(ui.RoleStatus)
	}
}
