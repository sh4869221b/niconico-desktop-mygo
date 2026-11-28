# #60: 500 件プロファイルの保存と UI 応答実測

状態: **合成遅延の測定用 A/B を NucBox で実施。通常アプリと実低速ストレージの確認が残っています**。旧基点 `45edceb24d62b7822e050ca19ff6689f0ca44224` では NucBox で専用 500 件の初期書き込みと終了を確認しました。保存ボタン、入力応答、250/1000 ms の操作試行はありません。
[PR #84](https://github.com/sh4869221b/niconico-desktop-mygo/pull/84) の main マージ `5527c6ce5c80ba0382f23b64ca66103d360900a1` 以降の `changeLibrary` に合わせた。実測するこの PR の exact head と、取り込まれている main の exact SHA を記録する。
NucBox では `29718170f578dce699f34f7300e6551c7bfb86a5` の測定用レイアウトで一覧が上部の保存ボタンに重なり、クリック先のサンプル行から未構成のカタログ処理が呼ばれた。測定試料は取得できなかった。修正版は上部の操作領域と一覧のビューポートを分け、サンプル行のフォーカスとスクロールだけを許可する。

Windows / Go 1.27.3+ で実行します。

## 測定用の同期／非同期 A/B

測定用 EXE は `MYGO_STORAGE_MODE=async` を指定すると、同じロック・再読込・Sync・置換を、単一の保存ワーカーで順番に実行します。未指定または `sync` で、製品が同期保存を使っていた時点の測定を再現できます。

ワーカーは受け付けた編集を先頭から処理します。失敗時はその編集と後続を保留し、「保存を再試行」で先頭をやり直します。「保存しました」は永続化の成功を受け取った後に表示します。未確認の編集がある状態で閉じると最大 2 秒だけ UI を動かしたまま待機します。失敗または時間超過時は窓に状態を表示して操作を保持します。

NucBox の基準は head `3bee92712a061fdb7b0904d49ab164080768702d` の測定用 EXE です。同じ SSD の専用 500 件と 1000 ms の合成待ちで、両モード各 3 回の保存を照合しました。同期時の保存と対応する view build の間隔は約 1016–1022 ms でした。非同期では保存区間の最大間隔が 62.8 ms、Tab、ホイール、F8 各 3 回は初回の保存完了前に処理されました。順番に処理された非同期操作の受付から保存完了までは 1027.3 / 1697.5 / 2350.1 ms でした。全 500 件が永続化されました。測定用の操作は再保存とフィクスチャの復元です。[PR #102 の測定条件と結果](https://github.com/sh4869221b/niconico-desktop-mygo/pull/102)

## 通常アプリでの追試

通常ビルドで登録、削除、取り消し、視聴履歴の変更を同じ保存キューに通します。受付中は「保存を待っています」と表示します。登録済み表示や取り消しの対象は、最新ファイルを読み直し Sync と置換を完了した状態から更新します。失敗時は先頭と後続を保持し、「保存を再試行」で同じ順序から再開します。メインウィンドウを通常終了するときは、必要な再生進捗も含めて待ちます。約 2 秒で保存が終わらなければ、ウィンドウは開いたまま保存が続きます。強制終了や OS セッション終了では、メモリーにある未確定の操作が残る保証はありません。

1. 今回の PR の Windows CI artifact の統合コミットと ZIP の SHA-256 を記録します。検証用 PowerShell で `APPDATA` と `LOCALAPPDATA` を新規の隔離領域に設定し、通常 EXE を起動します。実低速領域が使える場合は隔離 `APPDATA` をそこに作成します。
2. 検証用 `NiconicoDesktopMyGo\library.json` に登録済み項目を 499 件用意し、新しい公開動画をツールバーから登録します。受付から成功表示までは Tab とホイールで別の領域を操作します。入力時刻、画面が変わった時刻、成功表示を別々に記録します。
3. 確定した項目を削除し、「元に戻す」が表示されたら押します。保存が重なった場合に UI が応答し、確認表示が結果と一致することを確認します。
4. 保存中にメインウィンドウを通常終了します。成功後の終了か、時間超過で窓が残ることを確認します。再起動後に登録件数、対象 ID、並び順を照合します。

## 実行

このブランチの `git rev-parse HEAD` と `git merge-base HEAD origin/main`、Windows のビルド、CPU、対象ドライブの種類（SSD/USB/共有等）を記録。
本番プロファイルは使わない。テストは指定フォルダー内に `mygo-issue60-ランダム名` を新規作成し、500 件の専用 library.json を生成する。
測定の同期／非同期モードは本番と同じロック、最新ファイルの再読み込み、Sync、置換を使う。一覧のサンプル行には遷移や削除の処理がなく、ネットワーク・WebView・認証は使わない。

```powershell
New-Item -ItemType Directory storage60
$env:MYGO_STORAGE_ROOT = "$PWD/storage60"
$env:MYGO_STORAGE_TRACE = "$PWD/storage60-baseline.jsonl"
$env:MYGO_STORAGE_DELAY_MS = '0'
go test -mod=readonly -tags mygo_measure60 -c -o measure60.test.exe .
.\measure60.test.exe -test.run '^Test(LibrarySaveQueue|LibraryCloseWait|Measurement60)' -test.v
.\measure60.test.exe -test.run '^TestMeasure60$' -test.v -test.timeout=0
```

- ウィンドウを 1000×750 程度にし、「500件を再保存」を 10 回操作。
- 保存と同時に Tab を押してフォーカス移動し、一覧をホイールでスクロール。`tab-delivered` / `scroll-delivered` は UI へ届いた時刻。キーを押す／ホイールを回す時刻と、フォーカス枠／一覧が実際に変化した時刻は録画で記録する。
- 「500件に戻して保存」は同じ専用フィクスチャを置き直し、保存の観測に使える。
- 各試行の操作、保存件数、操作感、エラーを記録し、2 分以内にウィンドウを閉じる。JSONL は閉じると出力される（ディスク上の観測ログによる測定対象 I/O への干渉を避けるためメモリーに蓄積）。
- 終了時に library.json を再読込し、名前・順序を含む登録と履歴がメモリーと異なる場合はテストが失敗する。`fixture-start`～`fixture-end` は初期書き込み。操作試料から除く。`final.events` で保存、F8、Tab、スクロールの記録件数を確認する。初期書き込みと起動だけなら操作応答は未計測。

## 既知の入力を送って応答時間を測る

別の PowerShell で対象 PID を確認し、プローブを開始してから 5 秒以内に測定ウィンドウへフォーカスを戻す。

```powershell
Get-Process measure60.test | Select-Object Id,ProcessName,MainWindowTitle
powershell -NoProfile -File tools/issue60/probe.ps1 -ProcessId <上で確認したPID> -Log probe-baseline.jsonl
```

プローブは F8 を 40 回送信する。測定ウィンドウが前面でなくなれば停止する。最初は保存せず、送信数と `probe-handled` 数が一致することを確認する。IME 等が横取りした場合はその試行を入力遅延測定に使わず、Tab/スクロールの録画を残す。
次に新しいログ名でプローブを動かしながら保存する。
送信スクリプトの start_utc_ms と UI の probe-handled.utc_ns / 1,000,000 を順番に対応付ける。SendWait 自身も待つため、送信間隔を固定レートだと解釈しない。処理順や数が合わない試行は対応付け不可と記録する。

## 低速条件

1. 同じ SSD 上で synthetic delay 0 を baseline とする。
2. **実際に低速な検証用ドライブ/共有が使える場合**、`MYGO_STORAGE_ROOT` だけその専用フォルダーへ変更し delay 0 で再測定。端末・共有・ストレージ種別を記録する。
3. 実ドライブがなければ、`MYGO_STORAGE_DELAY_MS='250'`、次に `'1000'` で同じ操作をする。これは Sync **前**に測定ビルドだけが待つ合成遅延で、遅い実ストレージで再現した証拠とは区別する。実際の Sync 時間は real_elapsed_ns、注入を含む時間は total_elapsed_ns。

ログ名は毎回変える。既存ログの上書きは拒否する。`save-end.elapsed_ns` は UI 操作から保存完了まで、`storage.stage=transaction` はロック取得・再読込・編集・保存を含む。個々の `write` / `sync` / `close` / `rename` と、F8 の入力応答、Tab／スクロールの描画待ちを別集計にする。合成条件の `transaction` と `save-end` には注入した待ち時間が含まれる。

同期測定が終わったら、別のログ名にし、`$env:MYGO_STORAGE_MODE = 'async'` を付けて同じ 0 / 250 / 1000 ms 条件を再実行します。`save-queued.accepted` は受付、`save-end.saved` は永続化結果です。`save-end.queued_utc_ns` と `finished_utc_ns` を、Tab・ホイール・F8 が配信された時刻と照合します。保存直後の閉じる操作では、時間超過後に窓が残ることと、保存成功を確認した後に閉じて終了時の照合が通ることを記録します。

| 条件 | persist 回数 | Write / Sync / Rename ms（中央値・最大） | 最大 frame gap ms | プローブ遅延 ms（中央値・最大） | Tab / scroll の録画結果 |
|---|---:|---:|---:|---:|---|
| SSD / delay 0 | 未測定 | 未測定 | 未測定 | 未測定 | 未測定 |
| 実低速領域 / delay 0 | 未測定 | 未測定 | 未測定 | 未測定 | 未測定 |
| 合成 / 250 ms | 未測定 | 未測定 | 未測定 | 未測定 | 未測定 |
| 合成 / 1000 ms | 未測定 | 未測定 | 未測定 | 未測定 | 未測定 |

frame は約 16 ms 間隔で要求した view build の観測であり、OS の実描画や入力遅延そのものではない。製品画面全体は測定 UI より重い可能性がある。
自動テストは操作順、受付と成功の区別、保存データ、再試行、終了を判定する。応答時間は実機で記録する。

- [x] Cloud: 通常 Go テスト、測定用 Windows テストのクロスコンパイル
- [ ] NucBox: baseline と入力プローブの対応確認
- [x] NucBox: 測定用の同期／非同期 A/B と合成遅延下の入力・描画
- [ ] NucBox: 実低速領域の測定（領域が無い場合は未検証を維持）
- [x] Cloud: 通常アプリの登録操作、保存順、終了待ち、エラー再試行の回帰テスト
- [ ] NucBox: 通常アプリでの保存、入力、終了後の再読み込み

終了後、測定プロセスとログを確認し、設定した環境変数 `MYGO_STORAGE_ROOT`、`MYGO_STORAGE_TRACE`、`MYGO_STORAGE_DELAY_MS`、`MYGO_STORAGE_MODE` を取り除きます。
不要な専用 `mygo-issue60-*` だけ削除する。APPDATA・本番の library.json・OS 設定は変更しない。
