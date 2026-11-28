package main

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestAccountButtonRetainedDuringLoginCheck(t *testing.T) {
	b := browser{index: -1, loginState: loginAuthenticated, loginBusy: true, closed: true}
	view := ui.NewTester(b.draw, 1260, 850)
	if err := view.Click("アカウント"); err != nil {
		t.Fatal(err)
	}
	if b.loginState != loginAuthenticated {
		t.Fatal("checking changed the last confirmed login result")
	}
}
