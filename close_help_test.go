package main

import (
	"github.com/egoist/mygo/ui"
	"path/filepath"
	"testing"
)

func TestPlayerExplainsMainClose(t *testing.T) {
	for _, native := range []bool{false, true} {
		b := &browser{closed: true, index: -1}
		p := &playerWindow{}
		if native {
			p.native = &nativePlayer{}
		}
		b.player = p
		v := ui.NewTester(func(c *ui.Context) { b.draw(c) }, 800, 600)
		if !v.HasText("メインウィンドウを閉じると再生も終了します") {
			t.Fatal("close behavior is not explained", native)
		}
	}
}

func TestCloseHelpAcknowledgementSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "close-help.json")
	b := &browser{closeHelpPath: path, index: -1}
	p := &playerWindow{}
	b.player = p
	v := ui.NewTester(func(c *ui.Context) { b.draw(c) }, 800, 600)
	if err := v.Click("理解しました"); err != nil {
		t.Fatal(err)
	}
	if v.HasText("メインウィンドウを閉じると") || !loadCloseHelp(path) {
		t.Fatal("acknowledgement not retained")
	}
	b.closeHelpAcknowledged = loadCloseHelp(path)
	v.Frame()
	if v.HasText("理解しました") {
		t.Fatal("acknowledged help reappeared")
	}
}
