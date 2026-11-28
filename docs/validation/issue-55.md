# #55: 日本語 IME の確定と検索

NucBox 初回結果（親タスクからの実機報告）: v0.3.0 / main `45edceb24d62b7822e050ca19ff6689f0ca44224`、Microsoft IME で最初の Enter は確定のみ、2 回目で検索。**その試行では再現せず**。全 IME / Windows バージョンで問題がないとは判断していない。

製品挙動は変更しない。`TestSearchWaitsForIMECommitAndNextEnter` は実際の toolbar を使い、合成中 Enter →確定テキスト→次の Enter、確定と合成終了の両順序、検索先・並べ替え条件・二重実行しないこと・タグボタンを確認する。取消も別テストで確認する。
`ui.Tester` はプラットフォームが合成中キーを除外する前提をモデル化するため、Windows の IMM32 通知順そのものは検証できない。

```powershell
git rev-parse HEAD
go test -mod=readonly -run 'Test(SearchWaitsForIMECommitAndNextEnter|CancelledIMECompositionDoesNotSearch)$' -count=1 -v .
go build -mod=readonly -o NiconicoDesktopMyGo-ime.exe .
```

実機で既定の Microsoft IME と Windows ビルド、IME バージョン、アプリ SHA、設定した検索の並べ替えを記録する。

1. Ctrl+L で入力欄へ移動。「はつねみく」を入力し Space で「初音ミク」を選ぶ。
2. Enter を 1 回だけ押す。入力欄が「初音ミク」になり、検索結果・読み込み表示が変わらないことを確認。
3. キーを離してから Enter をもう 1 回。キーワード検索が 1 回だけ始まること、既定の並べ替えが保たれることを確認。
4. タグ検索ボタンでも確定済み文字列がタグとして検索されることを確認。Enter は通常のキーワード検索に割り当てられており、直前がタグ画面でも同じ。
5. 候補を Escape で取り消した試行、直接入力の Enter、候補選択し直し、長押しを別に記録。各 5 回。画面録画でキー押下と遷移を対応付ける。

| 環境/SHA | 操作/回数 | 1 回目の確定 | 2 回目の検索 | 二重実行 | 判定 |
|---|---|---|---|---|---|
| NucBox / v0.3.0 / 45edceb | 初回報告 | 確定のみ | 検索 | 報告なし | 再現せず（報告された試行） |
| 新ブランチ exact SHA | 未実施 | 未実施 | 未実施 | 未実施 | 未検証 |

- [x] 初回 NucBox: 基本の 2 Enter 操作で再現せず
- [x] Cloud: headless の toolbar 回帰テスト
- [ ] 新ブランチ: Windows IME 通知と実際の検索回数、取消・長押しの再確認

再現が得られたときだけ、その通知順と条件に基づき修正する。未実施欄を自動的に合格にしない。
