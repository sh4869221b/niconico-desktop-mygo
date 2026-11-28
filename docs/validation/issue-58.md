# #58: ローカル HLS のシーク実測

状態は **NucBox で部分測定済み**。head `595807d2753653eedd94e2a7625040e11cd0a60b` の 120 秒フィクスチャでは、0/100 ms 加算時に目標フレームへの移動を確認しました。試行中のセグメント GET は 0 件で、先読みの影響が考えられます。500 ms 加算時は最初の映像が制限時間内に出ず、シークは未実施でした。[PR #95 の条件・実測値](https://github.com/sh4869221b/niconico-desktop-mygo/pull/95)
測定ビルドは現行プレーヤーの Seek 呼び出しを記録します。

## 準備（NucBox の別タスクで実行）

Python 3.10+、FFmpeg（H.264/AAC encoder 付き）、Go 1.27.3+ を使用。
リポジトリのこのブランチの exact SHA を `git rev-parse HEAD` で記録する。
専用フォルダーの PowerShell で実行。既存ファイルを上書きしない。

```powershell
New-Item -ItemType Directory fixture58
ffmpeg -n -f lavfi -i 'testsrc2=size=640x360:rate=30' -f lavfi -i 'sine=frequency=440:sample_rate=48000' -t 120 -c:v libx264 -preset ultrafast -b:v 700k -maxrate 700k -bufsize 1400k -g 60 -sc_threshold 0 -pix_fmt yuv420p -c:a aac -b:a 96k -f hls -hls_time 2 -hls_playlist_type vod -hls_segment_filename 'fixture58/seg%03d.ts' fixture58/index.m3u8
Get-FileHash fixture58/* -Algorithm SHA256 | Export-Csv fixture58-hashes.csv -NoTypeInformation
python tools/issue58/hls_server.py fixture58 --log baseline-http.jsonl
```

別の PowerShell で、同じ checkout から測定用 EXE をコンパイルして起動します。最初に HTTP 200 と Range の HTTP 206 が確認できれば、プレーヤー起動前の準備が完了です。

```powershell
$env:MYGO_HLS_URL = 'http://127.0.0.1:8765/index.m3u8'
$env:MYGO_HLS_TRACE = "$PWD/baseline-player.jsonl"
curl.exe -I $env:MYGO_HLS_URL
curl.exe -sS -D - -H 'Range: bytes=0-15' -o NUL http://127.0.0.1:8765/seg000.ts
go test -mod=readonly -tags mygo_measure58 -c -o measure58.test.exe .
.\measure58.test.exe -test.run '^TestMeasure58$' -test.v -test.timeout=0
```

測定専用の native player が開く。通常ビルドにはローカル URL の入力口はない。
公式 API・認証・本番プロファイルは使用しない。音量初期値は 0。
1. 再生位置 10 秒付近で対象ウィンドウを前面にし、**F8** で新しい試行を記録します。タイトルに試行番号が出ます。
2. 一度だけ、10 秒付近から 80 秒付近へ約 2 秒かけてドラッグし、離します。再生が進むか 10 秒経過するまで待ち、同じ窓で **F9** を押します。PowerShell へ切り替える必要はありません。
3. 各条件で再生中のドラッグ、単一クリック、一時停止中のドラッグ、キーボード右矢印を分け、各 5 回実施します。試行番号、操作と目視結果を記録します。開始位置の調整は F8 の前に済ませます。一時停止試行では F9 まで再生を再開しません。
4. 同じ fixture を使い、試行条件ごとにプレーヤーとサーバーを閉じ、新しいログ名で起動します。基準はオプションなし、高遅延は `--latency-ms 500 --kib-s 0`、帯域制限は `--latency-ms 500 --kib-s 128` です。

## 初回フレームと未取得区間へのシークを分ける

初回フレームの追試では 120 秒の短いフィクスチャを使い、全リクエストの加算を 0、100、250、500 ms で変えます。プレーヤーを各条件で新しく起動します。`startup-result` に出る窓の表示時間、準備状態、失敗の文言を記録します。製品の再生ビューはストリームを表示した状態で 20 秒の初回フレーム期限を使います。失敗時はストリームを閉じる直前に測定ビルドが `native-stream-failure` を記録します。その時刻をマニフェストと最初の数セグメントの 206・転送バイト・切断時刻に照合します。

シークの追試では、上の FFmpeg コマンドの `-t 120` を `-t 600` に変更し、新規の `fixture58-long` に出力します。同じ 2 秒 GOP とセグメント設定を使い、マニフェストと開始から 60 秒分には遅延や帯域制限を加えません。

```powershell
python tools/issue58/hls_server.py fixture58-long --latency-ms 500 --kib-s 0 --delay-from-segment 30 --log cold-http.jsonl
```

再生開始約 5–10 秒で F8 を押し、約 540 秒へ単一クリックします。次に新しいサーバー／プレーヤー／ログで、同じ開始位置と目標に約 2 秒かけてドラッグし、再生中と一時停止中を分けます。500 ms 加算に帯域制限も加える場合は `--kib-s 128` を指定します。初回フレームの失敗も記録します。

```powershell
python tools/issue58/summarize.py cold-player.jsonl cold-http.jsonl --playlist fixture58-long/index.m3u8
```

最終目標の `target_segment` に `target_gets_before_trial: 0` かつ `target_gets_in_trial` が 1 件以上あれば、試行中に対象が初めてサーバーから要求されたケースとして扱えます。シーク直前に既に取得されていた場合は、試行の開始を早める、目標を後ろへずらす、または新規プロセスで再試行します。マーカー内の GET が 0 件の試行から未取得区間の挙動は判定できません。

## 記録と判定

`*-player.jsonl` は、試行マーカー、スライダーが呼んだ Seek、要求中の位置と確定した位置、再生・バッファ・一時停止を記録します。`*-http.jsonl` はリクエスト・Range・転送バイト・完了/切断・サーバー処理時間を記録します。両方のプロセスを閉じた後に集計します。

```powershell
python tools/issue58/summarize.py baseline-player.jsonl baseline-http.jsonl
```

同一端末の UTC ナノ秒で照合します。確定位置と進行は約 50 ms ごとのフレーム観測です。最後の Seek の目標から約 2 秒以内の確定位置を探し、再生中は確認位置から 250 ms 進んだ時刻も集計します。実際に映像が画面へ提示された時刻は目視または録画で別に記録します。未確認値は `null`、F9 前に閉じた試行は `closed_by_marker: false` です。

| 条件/試行 | Seek 数 | segment GET 数 / 重複 URL+Range 数 | 最終 Seek →位置が連続進行するまで ms | 目視/音声の再開 ms | エラー |
|---|---:|---:|---:|---:|---|
| baseline / playing-drag-1 | 未測定 | 未測定 | 未測定 | 未測定 | 未測定 |

サーバー帯域は **各 HTTP リクエスト単位**。並列取得の合計帯域は上限を超え得る。
サーバーの response.elapsed_ms はネットワーク取得時間で、再生再開時間ではない。
同一 URL+Range の追加取得は F8〜F9 区間内で数えます。ドラッグ解放の時刻には録画、デコーダーへ指示した時刻には最後の Seek のログを使います。
バッファ枯渇・エラー・最初から再生不能も失敗として残し、成功試行だけを選ばない。

- [x] Cloud: fixture HTTP/Range/ルート外アクセス拒否の自動検証
- [x] Cloud: 通常 Go テスト、Windows 測定テストのクロスコンパイル
- [x] NucBox: 0/100 ms の短いドラッグと 500 ms の初回フレーム失敗を記録
- [ ] NucBox: 未取得区間の約 2 秒ドラッグ、帯域制限、再生中と停止中の比較
- [ ] 問題が再現した場合のみ製品修正と同条件での回帰比較

終了時はサーバーを Ctrl+C、測定ウィンドウを閉じ、環境変数を Remove-Item Env:MYGO_HLS_URL,Env:MYGO_HLS_TRACE で削除する。
システムの回線設定や防御設定は変更しない。
