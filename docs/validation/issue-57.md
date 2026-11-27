# #57: 別窓・公式 WebView からのフォーカスと読み上げ

状態: **一部確認、読み上げは未検証**。初回 NucBox v0.3.0 / main `45edceb24d62b7822e050ca19ff6689f0ca44224` では公式 WebView→独自 UI に戻れたが、画面のフォーカス枠と UIA の差が報告された。タグ遷移と Narrator / NVDA の実際の発話は未実施。

不一致の対象要素・時刻がまだ揃っていないため、推測でフォーカス復帰は変更しない。
自動テストは native tree に正しい名前・role・focused が出ること、タグのキーボード起動後に検索結果を Tab で操作できること、公式モード解除後に native 再生ボタンへ移れることを確認する。WebView 自体・OS の UIA イベント・音声出力は含まない。

```powershell
git rev-parse HEAD
go test -mod=readonly -run 'Test(TagTransitionKeepsNativeResultKeyboardAccessible|NativeControlsReturnAfterOfficialModeIsDismissed)$' -count=1 -v .
go build -mod=readonly -o NiconicoDesktopMyGo-a11y.exe .
```

## NucBox の記録手順

既に使える Narrator、次に NVDA を **別々に**実施。未導入のものは未検証とする。Windows ビルド・読み上げソフト/版・アプリ exact SHA・キー操作を記録する。
読み上げを有効にして次の各遷移を 3 回実施し、画面録画（音声とキー操作付き）に窓名・実際のフォーカス枠・発話の全文・重複発話・待ち時間を残す。

1. メインの動画詳細→タグ→検索結果。キーボードでタグへ移り Enter。どの窓が前面になり、最初に何を読み、Tab でどこへ進むか。
2. 独自プレーヤー別窓→タグ→メインの検索結果。再生継続と、前面窓・フォーカス枠・UIA の一致を確認。
3. メインの公式ページ→「独自UIに戻る」。マウスとキーボードを分け、復帰直後・最初の Tab 後の窓名/要素/発話を記録。
4. 公式 WebView の動画→タグ検索がメインに開く場合も、別窓内の復帰とメインへの移動を別に記録する。

## 読み取り専用 UIA スナップショット

実機でアプリの PID を確認。対象アプリ以外の名前やテキストは記録しない。

```powershell
Get-Process NiconicoDesktopMyGo-a11y | Select-Object Id,MainWindowTitle
powershell -NoProfile -File tools/issue57/snapshot-uia.ps1 -ProcessId <PID> -Label before-tag -Output before-tag.json
# 対象アプリに戻って遷移操作。PowerShell を前面に戻す前に録画で即時フォーカスを残す
powershell -NoProfile -File tools/issue57/snapshot-uia.ps1 -ProcessId <PID> -Label after-tag -Output after-tag.json
```

このコマンドを PowerShell にフォーカスして実行すると、その操作自体がフォーカスを変える。即時フォーカスの証拠にする場合は、別の操作者/タスクから実行するか、次のように **バックグラウンドのプロセス**を事前予約してからアプリへ戻る。主観的な「戻った」と混ぜない。

```powershell
$script = (Resolve-Path tools/issue57/snapshot-uia.ps1).Path
$job = Start-Job -ArgumentList $script,<PID>,"$PWD/after-return.json" -ScriptBlock {
    param($script,$targetProcess,$destination)
    Start-Sleep -Seconds 8
    & $script -ProcessId $targetProcess -Label after-return -Output $destination
}
# 8 秒以内にアプリへ戻り、検証する遷移を実施
Wait-Job $job | Receive-Job
Remove-Job $job
```

`global_focus_process_id`、各窓の UIA `focused` / `focusable` / `bounds` と録画を対応付ける。WebView2 の子要素は別プロセスになる場合があるため、PID だけで一致/不一致を断定しない。truncated=true、要素消失、provider エラーは記録不足とする。
UIA snapshot は 1 時点の tree であり、読み上げの発話やイベント順を保証しない。スクリプトは UI 操作・設定変更を行わない。

| SHA / 読み上げ/版 | 遷移/操作 | 復帰直後の窓と画面枠 | UIA focused の名前/role | 最初の発話 / 重複 | 最初の Tab 後 | 判定 |
|---|---|---|---|---|---|---|
| 45edceb / 初回報告 | 公式→独自 UI | 復帰確認、差の報告あり | 詳細未記録 | 未検証 | 未検証 | 部分確認 |
| 新 exact SHA / Narrator | 各遷移 | 未実施 | 未実施 | 未実施 | 未実施 | 未検証 |
| 新 exact SHA / NVDA | 各遷移 | 未実施 | 未実施 | 未実施 | 未実施 | 未検証 |

- [x] Cloud: native tree / キーボード遷移の回帰テスト
- [ ] NucBox: snapshot スクリプトの動作確認
- [ ] NucBox: Narrator のタグ・別窓・公式復帰と音声記録
- [ ] NucBox: NVDA の同じ遷移と音声記録
- [ ] 差が再現した必要な遷移だけ修正し、同条件で再測定
