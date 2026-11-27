package main

import (
	"time"

	"github.com/egoist/mygo"
)

const commentLanguageWarning = "日本語コメントの設定に失敗しました。アプリを再起動してください。"

// Niconico's comment language is independent of the page's UI language.
// Set the site's language cookie, not WebView2's locale or HTTP headers.
func setJapaneseComments(set func(mygo.Cookie) error, now time.Time) error {
	return set(mygo.Cookie{
		Name:     "lang",
		Value:    "ja-jp",
		Domain:   ".nicovideo.jp",
		Path:     "/",
		Expires:  now.AddDate(1, 0, 0),
		Secure:   true,
		SameSite: mygo.CookieSameSiteLax,
	})
}
