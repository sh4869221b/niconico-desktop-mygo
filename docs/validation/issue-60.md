# #60: 500 件プロファイルの保存と UI 応答実測

状態: **実機の操作・低速条件は未検証**。旧基点 `45edceb24d62b7822e050ca19ff6689f0ca44224` では NucBox で専用500件の初期書き込みと終了を確認した。保存ボタン、入力応答、250/1000ms の試行はなく、旧基点の値は完了判定に使わない。
[PR #84](https://github.com/sh4869221b/niconico-desktop-mygo/pull/84) の main マージ `5527c6ce5c80ba0382f23b64ca66103d360900a1` 以降の `changeLibrary` に合わせた。実測するこの PR の exact head と、取り込まれている main の exact SHA を記録する。

同期保存を非同期化する修正は入れていない。Windows / Go 1.27.3+ で実行する。

## 実行

このブランチの `git rev-parse HEAD` と `git merge-base HEAD origin/main`、Windows のビルド、CPU、対象ドライブの種類（SSD/USB/共有等）を記録。
本番プロファイルは使わない。テストは指定フォルダー内に `mygo-issue60-ランダム名` を新規作成し、500 件の専用 library.json を生成する。
再保存・復元は `changeLibrary` を経由し、削除は本番の sidebar / `remove` / `changeLibrary` を呼ぶ。ネットワーク・WebView・認証は使わない。

```powershell
New-Item -ItemType Directory storage60
$env:MYGO_STORAGE_ROOT = "$PWD/storage60"
$env:MYGO_STORAGE_TRACE = "$PWD/storage60-baseline.jsonl"
$env:MYGO_STORAGE_DELAY_MS = '0'
go test -mod=readonly -tags mygo_measure60 -c -o measure60.test.exe .
.\measure60.test.exe -test.run '^TestMeasure60$' -test.v -test.timeout=0
```

- ウィンドウを 1000×750 程度にし、「500件を再保存」を 10 回操作。
- 保存と同時に Tab を押してフォーカス移動し、一覧をホイールでスクロール。`tab-delivered` / `scroll-delivered` は UI へ届いた時刻。キーを押す／ホイールを回す時刻と、フォーカス枠／一覧が実際に変化した時刻は録画で記録する。
- 左側の「削除」でも保存する。削除後は「500件に戻して保存」で初期件数へ戻す。
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

| 条件 | persist 回数 | Write / Sync / Rename ms（中央値・最大） | 最大 frame gap ms | プローブ遅延 ms（中央値・最大） | Tab / scroll の録画結果 |
|---|---:|---:|---:|---:|---|
| SSD / delay 0 | 未測定 | 未測定 | 未測定 | 未測定 | 未測定 |
| 実低速領域 / delay 0 | 未測定 | 未測定 | 未測定 | 未測定 | 未測定 |
| 合成 / 250 ms | 未測定 | 未測定 | 未測定 | 未測定 | 未測定 |
| 合成 / 1000 ms | 未測定 | 未測定 | 未測定 | 未測定 | 未測定 |

frame は約 16 ms 間隔で要求した view build の観測であり、OS の実描画や入力遅延そのものではない。製品画面全体は測定 UI より重い可能性がある。
タイミングで unit test の成否は判定しない。低速実領域での再現とデータ永続性を確認した後だけ、順序保証付き非同期化と回帰比較を検討する。

- [x] Cloud: 通常 Go テスト、測定用 Windows テストのクロスコンパイル
- [ ] NucBox: baseline と入力プローブの対応確認
- [ ] NucBox: 合成遅延の値と実 Sync 時間を区別した測定
- [ ] NucBox: 実低速領域の測定（領域が無い場合は未検証を維持）
- [ ] 再現後の製品修正・保存順/終了時 flush/エラー表示の回帰検証

終了後、測定プロセスとログを確認し、`Remove-Item Env:MYGO_STORAGE_ROOT,Env:MYGO_STORAGE_TRACE,Env:MYGO_STORAGE_DELAY_MS`。
不要な専用 `mygo-issue60-*` だけ削除する。APPDATA・本番の library.json・OS 設定は変更しない。
