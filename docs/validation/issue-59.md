# #59: 配布元・SHA-256 と Windows 初回表示

初回 NucBox 報告: v0.3.0 の公式 ZIP は公開 SHA-256 と一致、EXE は未署名。CLI 取得には Mark of the Web（MOTW）が付いておらず、**ブラウザー取得時の Windows 警告は未検証**。
「警告が出なかった」と「警告の検証が成立していない」を区別する。

追加確認: Scoop マニフェストは **0.1.1**、GitHub Release は **v0.3.0**。現状は同一バージョンの比較が成立しないため、**v0.3.0 の Scoop 経路は未検証**。バージョン差がある結果から包装方法の影響を判断しない。マニフェストはこの PR で変更しない。

README に正規配布元とハッシュ・署名の確認手順を追加した。コード署名や製品のダウンロード挙動は変更していない。

## 実機の比較

新規 Windows ユーザーは、検証用の既存アカウントが用意されている場合に利用する。検証のために OS の恒久設定や防御設定を変更しない。
Scoop が未導入の場合はその比較を未実施として記録し、この検証のためだけに実環境へ導入しない。

1. ブラウザーで [GitHub Releases](https://github.com/sh4869221b/niconico-desktop-mygo/releases) の **同じタグ**から ZIP と SHA256SUMS.txt を取得。ブラウザー名/版、URL、タグ、取得日時、Windows ビルドを記録する。
2. README のコマンドで SHA-256 を照合。不一致なら展開・起動しない。
3. 次は既存の MOTW を読むだけ。Zone.Identifier を作成・削除しない。

```powershell
Get-Item .\NiconicoDesktopMyGo-win-x64.zip -Stream Zone.Identifier -ErrorAction SilentlyContinue
Get-Content .\NiconicoDesktopMyGo-win-x64.zip -Stream Zone.Identifier -ErrorAction SilentlyContinue
```

4. 通常のエクスプローラーで新しい専用フォルダーへ展開。EXE の Zone.Identifier と Get-AuthenticodeSignature も記録する。
5. 通常のダブルクリックで起動を試し、表示されたダイアログのタイトル・ファイル名・発行元・文言・時刻・スクリーンショットを保存。実行がブロックされた場合はその時点で中止し、ブロックを回避しない。
6. Scoop が既に利用できる別の検証用アカウントでは `scoop cat mybucket/niconico-desktop-mygo` で URL / hash / version を確認する。同じタグかを記録し、通常のインストール・初回起動で比較する。ZIP 取得時と同一バージョンでない場合は比較不能と記録する。
7. MOTW が存在しない取得経路はその事実を残す。SmartScreen 等の出なかった理由を、MOTW だけに断定しない。

| タグ/ZIP hash | 経路/ブラウザー | ZIP/EXE MOTW | 署名 Status/発行元 | 警告の文言/結果 | 判定 |
|---|---|---|---|---|---|
| v0.3.0 / 公開値と一致（初回報告） | CLI | 無し / 未記録 | 未署名 | ブラウザー警告未実施 | 未検証 |
| exact タグ / hash を追記 | ブラウザー | 未実施 | 未実施 | 未実施 | 未検証 |
| v0.3.0 | Scoop（現マニフェスト 0.1.1） | 未実施 | 未実施 | 同一版を入手できず比較不能 | 未検証 |

- [x] README: 正規配布元、SHA-256、署名の確認手順
- [x] NucBox 初回: ZIP の hash 一致、EXE 未署名の報告
- [ ] ブラウザー取得 ZIP と展開 EXE の MOTW・警告を記録
- [ ] 既存 Scoop 検証環境で同版を比較
- [ ] 必要性を実測から判断してコード署名を検討

ハッシュ一致や未署名だけを根拠に警告表示・安全性の結論は出さない。Issue は実機チェック完了まで開いたまま。
