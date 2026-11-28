package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type profileFile struct{ kind, name, path, state string }

func validateProfileBytes(kind string, data []byte) error {
	if len(data) > maxLibraryBytes {
		return errors.New("ファイルが大きすぎます")
	}
	switch kind {
	case "library":
		var value *library
		if json.Unmarshal(data, &value) != nil || value == nil {
			return errors.New("登録・履歴のJSON形式が不正です")
		}
	case "ng":
		var value []commentAuthor
		if len(data) > maxCommentNGBytes || json.Unmarshal(data, &value) != nil || value == nil || len(value) > maxCommentNGEntries {
			return errors.New("NG設定のJSON形式が不正です")
		}
		for _, author := range value {
			if !author.valid() {
				return errors.New("NG投稿者IDが不正です")
			}
		}
	case "playback":
		var value struct {
			Player playbackMode `json:"player"`
		}
		if len(data) > 4096 || json.Unmarshal(data, &value) != nil || value.Player != nativePlayback && value.Player != webPlayback {
			return errors.New("再生設定のJSON形式が不正です")
		}
	default:
		return errors.New("未対応のファイル種別です")
	}
	return nil
}
func openRegularProfileFile(path string) (*os.File, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("通常のファイルを選んでください")
	}
	return os.Open(path)
}

func readProfileBytes(path string) ([]byte, error) {
	f, err := openRegularProfileFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxLibraryBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxLibraryBytes {
		return nil, errors.New("ファイルが大きすぎます")
	}
	return data, nil
}
func inspectProfileFiles(root string) []profileFile {
	if root == "" || root == "." {
		return nil
	}
	files := []profileFile{{kind: "library", name: "登録・視聴履歴", path: filepath.Join(root, "library.json")}, {kind: "ng", name: "コメントNG", path: filepath.Join(root, "comment-ng.json")}, {kind: "playback", name: "再生設定", path: filepath.Join(root, "playback.json")}}
	for i := range files {
		data, err := readProfileBytes(files[i].path)
		if errors.Is(err, os.ErrNotExist) {
			files[i].state = "未作成（既定値を使用）"
			continue
		}
		if err == nil {
			err = validateProfileBytes(files[i].kind, data)
		}
		if err != nil {
			files[i].state = "読み込み失敗（元ファイルを保護）: " + err.Error()
		} else {
			files[i].state = "読み込み可能"
		}
	}
	return files
}

// Always creates a new copy. The source and all existing destinations survive failures.
func saveProfileCopy(source, destination, kind string, validate bool) error {
	var contents io.Reader
	if validate {
		data, err := readProfileBytes(source)
		if err != nil {
			return err
		}
		if err := validateProfileBytes(kind, data); err != nil {
			return err
		}
		contents = bytes.NewReader(data)
	} else {
		original, err := openRegularProfileFile(source)
		if err != nil {
			return err
		}
		defer original.Close()
		contents = original
	}
	f, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		f.Close()
		if !complete {
			os.Remove(destination)
		}
	}()
	if _, err = io.Copy(f, contents); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	complete = true
	return nil
}
func (b *browser) chooseProfileCopy(file profileFile, recovery bool) {
	if b.storageBusy {
		return
	}
	b.storageBusy = true
	go func() {
		source := file.path
		var destination, notice string
		mygo.RunOnMain(func() {
			if b.closed {
				return
			}
			suffix, title := ".backup.json", "元ファイルのバックアップを別名で保存"
			if recovery {
				paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: b.win, Title: "復元に使うJSONファイルを選択"})
				if err != nil {
					notice = "ファイルを選べませんでした。"
					return
				}
				if len(paths) == 0 {
					return
				}
				source = paths[0]
				suffix, title = ".recovered.json", "検証済みの復元用コピーの保存先を選択"
			}
			path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: b.win, Title: title, DefaultPath: file.path + suffix})
			if err != nil {
				notice = "保存先を選べませんでした。"
				return
			}
			destination = path
		})
		if destination != "" {
			if err := saveProfileCopy(source, destination, file.kind, recovery); err != nil {
				notice = "コピーを保存できませんでした。元ファイルは変更していません: " + err.Error()
			} else {
				notice = "コピーを保存しました: " + destination
				if recovery {
					notice += "。アプリを終了し、元ファイルのバックアップを確認してから、このコピーを " + file.path + " に配置して再起動してください。"
				}
			}
		}
		mygo.RunOnMain(func() {
			b.storageBusy = false
			if notice != "" {
				b.storageNotice = notice
			}
			if !b.closed && b.win != nil {
				b.win.Invalidate()
			}
		})
	}()
}
func (b *browser) drawStorageSettings(c *ui.Context) {
	label := "保存先と復旧を表示"
	if b.storageOpen {
		label = "保存先と復旧を閉じる"
	}
	if ui.Button(c, label).Clicked() {
		b.storageOpen = !b.storageOpen
		if b.storageOpen && !b.storageBusy {
			root := filepath.Dir(b.libraryPath)
			b.storageBusy = true
			go func() {
				files := inspectProfileFiles(root)
				mygo.RunOnMain(func() {
					b.storageFiles, b.storageBusy = files, false
					if !b.closed && b.win != nil {
						b.win.Invalidate()
					}
				})
			}()
		}
	}
	if !b.storageOpen {
		return
	}
	ui.Text(c, "保存先・読み込み状態").Bold()
	ui.Text(c, "復旧手順: 元ファイルを別名でバックアップ → 手元の復元用JSONを検証して別名保存 → アプリ終了後にコピーを配置 → 再起動。既存ファイルは自動で上書きしません。").Wrap()
	if len(b.storageFiles) == 0 && b.storageBusy {
		ui.Text(c, "保存ファイルを確認しています…").Role(ui.RoleStatus)
	} else if len(b.storageFiles) == 0 {
		ui.Text(c, "保存先を取得できません。")
	}
	for i, file := range b.storageFiles {
		row := c.Key(fmt.Sprintf("storage-%d", i))
		ui.Column(row).FillWidth().Gap(4).Children(func() {
			ui.Text(row, file.name+": "+file.state).Wrap()
			ui.Text(row, "フォルダー: "+filepath.Dir(file.path)).Wrap()
			ui.Text(row, "ファイル: "+file.path).Wrap()
			ui.Row(row).Wrap().Gap(6).Children(func() {
				if ui.Button(row, "保存先をコピー").Label(file.name + "の保存先をコピー").Clicked() {
					row.WriteClipboard(file.path)
				}
				if ui.Button(row, "バックアップを別名保存").Label(file.name + "のバックアップを別名保存").Disabled(b.storageBusy).Clicked() {
					b.chooseProfileCopy(file, false)
				}
				if ui.Button(row, "復元用コピーを保存").Label(file.name + "の復元用コピーを保存").Disabled(b.storageBusy).Clicked() {
					b.chooseProfileCopy(file, true)
				}
			})
		})
	}
	if b.storageNotice != "" {
		ui.Text(c, b.storageNotice).Wrap().Role(ui.RoleStatus)
	}
}
