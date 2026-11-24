# Niconico Desktop MyGo

Windows用の非公式ニコニコブラウザです。[C#版](https://github.com/sh4869221b/niconico-desktop)とは別プロジェクトです。Goでアプリの操作部を実装し、[MyGo](https://github.com/egoist/mygo)のネイティブUIにMicrosoft Edge WebView2を埋め込んで、公式サイトを同じウィンドウに表示します。

## 使い方

- **検索 / タグ検索**：検索語を入力してボタンを選びます。「検索」では動画IDや公式URLも開けます。
- **ログイン / マイリスト**：ニコニコ公式ページで操作します。起動時にMyGo版の専用セッションでニコニコのコメント言語を日本語に設定します。ログイン情報や保存済み項目は消去しません。視聴条件、コメント弾幕の表示・非表示、公開範囲は公式プレーヤーとアカウントの権限に従います。外部事業者によるログインは対象外です。
- **☆ 登録 / 視聴履歴**：検索条件と並び順、動画、ユーザーやマイリストのリンクを端末内に登録できます。検索結果の固定保存やニコニコ側のマイリスト変更は行いません。開いた動画は最大100件まで記録します。
- **URLをコピー**：ユーザーの投稿動画一覧や個別のマイリストを開いたとき、[go-nico-list](https://github.com/sh4869221b/go-nico-list)用の正規URLをコピーできます。「自分のマイリスト」は先に個別のリストを選択してください。認証状態は共有しないため非公開リストはgo-nico-listでは取得できません。
- 認証ページを履歴や登録先に保存しません。表示ページにはGoの操作を公開せず、外部ページへの移動、新規ウィンドウ、ダウンロード、サイト権限は許可しません。ユーザー操作で開いた公式リンクは同じ表示欄へ移動します。

登録済み項目と履歴は `%APPDATA%\NiconicoDesktopMyGo\library.json` に保存します。WebView2もMyGo版専用のセッションを用いるため、C#版からログイン状態や保存済み項目を自動移行しません。

## ビルド

Windows 10/11 x64と [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/) が必要です。ビルドにはGo 1.27.3以降を使います。

```powershell
go test ./...
go build -trimpath -ldflags=-H=windowsgui -o NiconicoDesktopMyGo.exe .
```

GitHub Actionsの「Windows build」はテスト、静的チェック、Windows x64のビルドを実行し、試用ZIPとSHA-256を成果物として保存します。タグを push するか、`main`で「Windows build」を開いて`vMAJOR.MINOR.PATCH`を指定すると、上記のチェックに合格したファイルだけをGitHub Releasesへ公開します。

公開版はScoopの`mybucket/niconico-desktop-mygo`で導入できます。C#版とは別パッケージです。
