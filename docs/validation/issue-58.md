# #58: ローカル HLS のシーク実測

状態: **未検証**。NucBox v0.3.0 / main `45edceb24d62b7822e050ca19ff6689f0ca44224` では低速 HLS 未実施。
この PR は観測用。Seek の頻度・タイミングは変更していない。実測が揃うまで Issue を完了しない。

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

別の PowerShell（同じ checkout）:

```powershell
$env:MYGO_HLS_URL = 'http://127.0.0.1:8765/index.m3u8'
$env:MYGO_HLS_TRACE = "$PWD/baseline-player.jsonl"
go test -mod=readonly -tags mygo_measure58 -run '^TestMeasure58$' -count=1 -timeout=0 -v .
```

測定専用の native player が開く。通常ビルドにはローカル URL の入力口はない。
公式 API・認証・本番プロファイルは使用しない。音量初期値は 0。
再生が開始したら以下を各 5 回実施し、各試行前後にマーカーを付ける。

```powershell
Invoke-WebRequest 'http://127.0.0.1:8765/__mark?label=playing-drag-1-start' -UseBasicParsing
# 再生位置 10 秒付近から 80 秒付近へ 2 秒ほどかけて一度ドラッグし、離す
Invoke-WebRequest 'http://127.0.0.1:8765/__mark?label=playing-drag-1-end' -UseBasicParsing
```

- 再生中、単一クリック、一時停止中のドラッグ、キーボード左右を別試行にする。
- 同じ fixture でサーバーと測定ウィンドウを閉じ、別名ログで条件を変更する。
- 高遅延: `--latency-ms 500 --kib-s 0`
- 帯域制限: `--latency-ms 500 --kib-s 128`
- 各条件は新しいプレーヤーを開く（デコーダーのキャッシュを混ぜない）。

## 記録と判定

`*-player.jsonl`: 実際の画面が呼んだ `seek`（target_ns）、再生位置、Ready / Preparing / Buffering / Playing / failure。
`*-http.jsonl`: リクエスト・Range・転送バイト・完了/切断・サーバー処理時間・試行マーカー。
UTC ナノ秒で同一端末のログを結合する。再生位置は約 50 ms ごとのフレーム観測であり、画面に映像が提示された時刻そのものではない。

| 条件/試行 | Seek 数 | segment GET 数 / 重複 URL+Range 数 | 最終 Seek →位置が連続進行するまで ms | 目視/音声の再開 ms | エラー |
|---|---:|---:|---:|---:|---|
| baseline / playing-drag-1 | 未測定 | 未測定 | 未測定 | 未測定 | 未測定 |

サーバー帯域は **各 HTTP リクエスト単位**。並列取得の合計帯域は上限を超え得る。
サーバーの response.elapsed_ms はネットワーク取得時間で、再生再開時間ではない。
再取得はマーカー区間の生ログから URL+Range 別に数え、初回取得と区別する。
マーカーは手動なのでドラッグ解放の exact 時刻ではない。Seek の最終時刻を主指標にし、目視は画面録画と併記する。
バッファ枯渇・エラー・最初から再生不能も失敗として残し、成功試行だけを選ばない。

- [x] Cloud: fixture HTTP/Range/ルート外アクセス拒否の自動検証
- [x] Cloud: 通常 Go テスト、Windows 測定テストのクロスコンパイル
- [ ] NucBox: baseline / 高遅延 / 帯域制限の実測、再生中と停止中の比較
- [ ] 問題が再現した場合のみ製品修正と同条件での回帰比較

終了時はサーバーを Ctrl+C、測定ウィンドウを閉じ、環境変数を Remove-Item Env:MYGO_HLS_URL,Env:MYGO_HLS_TRACE で削除する。
システムの回線設定や防御設定は変更しない。
