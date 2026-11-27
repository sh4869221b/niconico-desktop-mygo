# #56: 異なる DPI のモニター間移動

状態: **未検証**。NucBox 初回はモニター 1 台のみ。単一モニターでの正常動作を、異なる DPI 間の合格として扱わない。
基準製品: v0.3.0 / main `45edceb24d62b7822e050ca19ff6689f0ca44224`。

回帰テスト `TestPlayerHitTargetsAcrossDeviceScaleTransitions` は実際の player view に device scale 1→1.5→2→1.5→1 を繰り返し適用し、2 種類の DIP サイズで映像の縦横比・映像中央クリック・黒帯クリック無視・シークの入力順・コメント行が残ることを確認する。
headless の DIP / device pixel を検証しており、Windows の WM_DPICHANGED、Media Foundation 面、画面座標のイベント変換そのものは含まない。

```powershell
git rev-parse HEAD
go test -mod=readonly -run '^TestPlayerHitTargetsAcrossDeviceScaleTransitions$' -count=1 -v .
go build -mod=readonly -o NiconicoDesktopMyGo-dpi.exe .
```

## NucBox 実機条件

表示倍率が既に 100% と 150%（または 200%）の 2 画面を接続できる場合に実施する。恒久設定の変更はしない。2 画面が用意できなければ未検証を維持。
両画面の解像度・倍率・配置（右/左、主画面）・Windows ビルド・GPU とドライバー・アプリ exact SHA を記録。

1. 独自プレーヤーで同じ動画を再生し、コメント一覧と弾幕を表示。
2. 再生中に画面 A→B→A を各 5 往復。境界をまたいで止める場合と、ウィンドウを全体移動する場合を分ける。
3. 各移動直後に映像中央を 1 回クリック（停止）、もう 1 回（再生）。黒帯は状態を変えない。シークの 25% / 75% をクリックし、時刻とポインター位置を記録。
4. 一時停止中も同じ移動を実施。コメント一覧の行選択/スクロール、概要欄リンク位置を確認。
5. 異常があれば「最後に正常だった画面/倍率、最初にズレた画面/倍率、操作、映像とクリックの画面座標差」を録画と画面写真に残す。映像だけ、入力だけ、コメントだけの差を区別する。

| SHA / A→B の倍率 | 再生/停止 | 回数 | 中央/黒帯 | Seek 25/75% | コメント/弾幕 | 判定 |
|---|---|---:|---|---|---|---|
| NucBox 初回 | 両 DPI 環境なし | 0 | 未実施 | 未実施 | 未実施 | 未検証 |

- [x] Cloud: device scale 遷移の描画・hit target 回帰テスト
- [ ] NucBox: 異なる DPI の 2 画面を用意
- [ ] NucBox: 再生中・停止中の移動と物理入力座標の実測
- [ ] 再現した境界だけ修正し、同じ画面配置で再測定
