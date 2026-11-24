package main

import (
	"time"

	"github.com/egoist/mygo"
)

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
