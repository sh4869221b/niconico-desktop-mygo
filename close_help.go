package main

import (
	"bytes"
	"github.com/egoist/mygo/ui"
	"os"
)

func loadCloseHelp(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var data [65]byte
	n, _ := f.Read(data[:])
	return bytes.Equal(bytes.TrimSpace(data[:n]), []byte(`{"acknowledged":true}`))
}

func (b *browser) drawCloseHelp(c *ui.Context) {
	if b.closeHelpAcknowledged {
		return
	}
	ui.Row(c).FillWidth().Padding(6, 10).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "メインウィンドウを閉じると再生も終了します。動画だけを見たいときはメインを最小化してください。").Grow(1).MinWidth(0).Wrap().Role(ui.RoleStatus)
		if ui.Button(c, "理解しました").Clicked() {
			if b.closeHelpPath != "" && writeSettings(b.closeHelpPath, []byte(`{"acknowledged":true}`)) == nil {
				b.closeHelpAcknowledged = true
			} else {
				b.reportStatus("説明の確認状態を保存できませんでした。")
			}
		}
	})
}
