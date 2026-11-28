# Issue #37 — Windows でウィンドウ配置と登録欄を確認する

[PR #91](https://github.com/sh4869221b/niconico-desktop-mygo/pull/91) でメインとプレーヤーに別々の保存キーを設定しました。登録欄は別のファイルに保存します。Cloud の UI テストでは新規表示と、ツールバーで開閉して別の画面モデルに読み直した場合の表示を確認しています。Windows での復元とディスプレイ切断後の復帰は実機で確認します。

## 隔離して起動

main `3b62b2487e996c26a491c8529d0900c51cae5436` の [Windows x64 artifact 15270145465](https://github.com/sh4869221b/niconico-desktop-mygo/actions/runs/46160872980/artifacts/15270145465) で試せます。別のビルドを使う場合も、run、チェックアウトされたコミット、`SHA256SUMS.txt` と照合した ZIP の SHA-256 を記録します。検証用 PowerShell で以下を実行し、同じシェルから各回を起動します。`$exe` には展開した検証用 EXE を指定してください。

```powershell
$profileRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("MyGo-37-" + [guid]::NewGuid())
$env:APPDATA = Join-Path $profileRoot 'Roaming'
$env:LOCALAPPDATA = Join-Path $profileRoot 'Local'
New-Item -ItemType Directory -Path $env:APPDATA, $env:LOCALAPPDATA | Out-Null
Start-Process -FilePath $exe
```

## 位置、寸法、最大化、登録欄

1. 新規プロファイルで登録欄が見えることを確認します。公開動画（例 `sm1097445`）からプレーヤー別窓を開きます。非最大化の主画面と別窓を、区別できる位置と寸法に配置します。ドラッグを終え、0.5 秒以上待ちます。
2. メインの「登録・履歴」で登録欄を閉じます。プレーヤーを閉じてからメインを通常終了します。下のコマンドで `main` と `player` の別々の通常矩形、および `showLibrary: false` を記録します。
3. 同じシェルから再起動し、再び別窓を開きます。両窓がそれぞれの矩形に戻り、登録欄が閉じていることを確認します。
4. 両窓を最大化し、登録欄を開きます。通常終了して再起動し、両窓の最大化と登録欄の表示を確認します。Windows の「元に戻す」で各窓が自身の非最大化時の位置と寸法に戻るかも記録します。

```powershell
$windowState = Join-Path $env:APPDATA 'Niconico Desktop MyGo\window-state.json'
$sidebarState = Join-Path $env:APPDATA 'NiconicoDesktopMyGo\layout.json'
Get-Content -LiteralPath $windowState -Raw | ConvertFrom-Json |
  Select-Object main, player | ConvertTo-Json -Depth 4
Get-Content -LiteralPath $sidebarState -Raw | ConvertFrom-Json |
  Select-Object showLibrary | ConvertTo-Json
```

`pixelX`、`pixelY`、`pixelWidth`、`pixelHeight` は通常時の矩形の物理ピクセル値です。ゼロの値や false のフラグは省略されます。最大化時はタイトルバーと Windows の最大化状態を画面でも確認します。

## ディスプレイが外れた場合

2 画面を使える検証環境で、非最大化のプレーヤーを第 2 画面だけに収めます。通常終了し、保存されたプレーヤーの矩形を記録します。第 2 画面が Windows から外れている間に同じプロファイルで起動し、別窓を開きます。残った画面の作業領域でタイトルバーと操作部にアクセスでき、窓を動かせることを確認します。前後の解像度・拡大率、残存画面上の窓の位置と寸法を記録します。画面が 1 台だけの場合はここを未実施と記録します。保存 JSON や画面設定を書き換えて結果を作る必要はありません。
