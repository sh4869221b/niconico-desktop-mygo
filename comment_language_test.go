package main

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

func TestJapaneseCommentsOnlySetsNiconicoLanguage(t *testing.T) {
	now := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
	var calls int
	want := mygo.Cookie{
		Name: "lang", Value: "ja-jp", Domain: ".nicovideo.jp", Path: "/",
		Expires: now.AddDate(1, 0, 0), Secure: true, SameSite: mygo.CookieSameSiteLax,
	}
	err := setJapaneseComments(func(got mygo.Cookie) error {
		calls++
		if got != want {
			t.Errorf("language cookie = %#v; want %#v", got, want)
		}
		return nil
	}, now)
	if err != nil || calls != 1 {
		t.Fatalf("cookie writes = %d, error = %v", calls, err)
	}
}

func TestJapaneseCommentsReportsCookieStoreFailure(t *testing.T) {
	storeErr := errors.New("cookie store unavailable")
	if err := setJapaneseComments(func(mygo.Cookie) error { return storeErr }, time.Now()); !errors.Is(err, storeErr) {
		t.Fatalf("error = %v; want %v", err, storeErr)
	}
}
