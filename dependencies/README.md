# MyGo

`mygo/` のベースは [MyGo v0.10.0](https://github.com/egoist/mygo/tree/ebde9399baf2734b3361856a53f4036859973d1d) です。このアプリが使うパッケージを `go mod vendor` で抽出し、上流の `go.mod` とともに収録しています。ルートの `go.mod` でローカルのソースを指定しています。

Windows では WebView2 の `NavigationId` で開始・コミット・完了を照合します。継続中のIDへリダイレクト先を記録し、失敗した場合はそのリクエスト先を返します。公式別窓は、受理した開始と現在のキャンセルを通知する追加イベントを使います。

Windows x64 の回帰テストはルートで `go test ./... github.com/egoist/mygo/internal/windows` を実行します。
