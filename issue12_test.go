package main

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestLoginPreflightAndReturnToSearch(t *testing.T) {
	b := browser{navigation: []screenRoute{routeFor(home)}, closed: true}
	b.navigate(login)
	if !b.loginHelp || b.official {
		t.Fatal("login skipped preflight")
	}
	v := ui.NewTester(b.draw, 1260, 850)
	if !v.HasText("Windowsでは Microsoft Edge WebView2 Runtime が必要です。") {
		t.Fatal(v.Texts())
	}
	if err := v.Click("検索へ戻る"); err != nil {
		t.Fatal(err)
	}
	if b.loginHelp || b.official {
		t.Fatal("did not return")
	}
}
