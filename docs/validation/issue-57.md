# #57: 別窓・公式 WebView からのフォーカスと読み上げ

状態: **NucBox で UIA フォーカスの不一致を再現。修正候補の再試験と読み上げは未実施**。
Windows x64 artifact `15267190837`、main `5d178f00d545b2ae57c5a1c955f631d8e3cab3ec`（[Windows CI](https://github.com/sh4869221b/niconico-desktop-mygo/actions/runs/46153497683)）を専用プロファイルで起動した。公開動画 `sm1097445` の詳細から公式 WebView 別窓を開き、Alt+F4 で閉じてメインで Tab を一度押すと「音楽」タグに枠が付き、Enter でタグ検索へ遷移した。UIA が返した focused 要素は検索欄のままだった。Alt+Left での戻りと F6 の領域移動でも、描画と UIA の差が見られた。

Windows の UIA プロバイダーは仮想要素間でフォーカスが動いたとき、UIA イベントの購読がなくても Windows のフォーカス通知を送る。これまでフォーカス移動の通知は購読状態に従っていた。アプリのフォーカス移動先は変えない。
自動テストは、ウィンドウのフォーカス復帰、Tab・F6 が選んだ要素と native accessibility tree の一致、および購読がない場合の Windows プロバイダーの通知経路を検証する。実際の Windows UIA クライアント、別窓の WebView2、Narrator / NVDA の発話は以下で確認する。

```powershell
git rev-parse HEAD
go test -mod=readonly -run 'Test(TagTransitionKeepsNativeResultKeyboardAccessible|NativeControlsReturnAfterOfficialModeIsDismissed|PlaybackWindowFocusReturnKeepsTabAndF6InNativeRegions)$' -count=1 -v .
go test -mod=readonly -run '^TestUIAFocusPublishesWinEventWithoutUIAEventSubscription$' -count=1 -v github.com/egoist/mygo/internal/windows
go build -mod=readonly -o NiconicoDesktopMyGo-a11y.exe .
```

## NucBox の記録手順

既に使える Narrator、次に NVDA を **別々に**実施。未導入のものは未検証とする。Windows ビルド・読み上げソフト/版・アプリ exact SHA・キー操作を記録する。
読み上げを有効にして次の各遷移を 3 回実施し、画面録画（音声とキー操作付き）に窓名・実際のフォーカス枠・発話の全文・重複発話・待ち時間を残す。

1. メインの動画詳細→タグ→検索結果。キーボードでタグへ移り Enter。どの窓が前面になり、最初に何を読み、Tab でどこへ進むか。
2. 独自プレーヤー別窓→タグ→メインの検索結果。再生継続と、前面窓・フォーカス枠・UIA の一致を確認。
3. メインの公式ページ→「独自UIに戻る」。マウスとキーボードを分け、復帰直後・最初の Tab 後の窓名/要素/発話を記録。
4. 公式 WebView の動画→タグ検索がメインに開く場合も、別窓内の復帰とメインへの移動を別に記録する。
5. 再現に使った動画で公式別窓を Alt+F4 で閉じ、メインで Tab を1回。画面の枠、`global_focus`、同じ窓の `nodes[].focused` を記録してから Enter でタグ検索を確認する。Alt+Left と F6 もそれぞれ試す。Narrator / NVDA を起動する前の UIA スナップショットと、発話を記録する試行は別に残す。

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

`global_focus`（対象プロセス内の名前と role）、`global_focus_process_id`、各窓の UIA `focused` / `focusable` / `bounds` と録画を対応付ける。WebView2 の子要素は別プロセスになる場合があるため、PID だけで一致/不一致を断定しない。truncated=true、要素消失、provider エラーは記録不足とする。
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
