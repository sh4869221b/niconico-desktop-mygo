package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
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

func TestSearchToolbarDismissesLoginHelp(t *testing.T) {
	for _, test := range []struct {
		name, button string
		official     bool
		start, want  int
	}{
		{name: "leave official", button: "独自UIに戻る", official: true, start: 0, want: 0},
		{name: "back from official", button: "戻る", official: true, start: 0, want: 0},
		{name: "back from preflight", button: "戻る", start: 1, want: 0},
		{name: "forward from preflight", button: "進む", start: 0, want: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			b := browser{index: -1, closed: true, closeHelpAcknowledged: true}
			b.navigate(home)
			b.navigate(searchURL("search", "春の動画"))
			if test.start == 0 {
				b.moveNative(-1)
			}
			b.navigate(login)
			if test.official {
				b.official, b.officialTarget = true, login
			}
			v := ui.NewTester(b.draw, 1260, 850)
			if !v.HasText("公式ログインを開く前に") {
				t.Fatal("fixture did not show login help")
			}
			if err := v.Click(test.button); err != nil {
				t.Fatal(err)
			}
			if b.loginHelp || b.official || b.index != test.want || v.HasText("公式ログインを開く前に") {
				t.Fatalf("login help still covers the selected route: index=%d, want %d", b.index, test.want)
			}
			want := "ニコニコ動画"
			if test.want == 1 {
				want = "春の動画"
			}
			if !v.HasText(want) {
				t.Fatalf("native route %q is missing", want)
			}
		})
	}
}
