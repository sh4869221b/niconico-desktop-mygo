package main

import (
	"github.com/egoist/mygo/ui"
	"strings"
	"testing"
)

func TestCollectionSearchGuidanceKeepsDefaultOrder(t *testing.T) {
	for _, kind := range []screenKind{screenUser, screenList, screenOwnList} {
		r := routeFor(home + "mylist/1")
		r.kind = kind
		b := browser{navigation: []screenRoute{r}, closed: true}
		v := ui.NewTester(b.draw, 1260, 850)
		if !v.HasText(listSearchNotice(kind)) {
			t.Fatal(v.Texts())
		}
		if err := v.Click("公式ページで絞り込み・並べ替え"); err != nil {
			t.Fatal(err)
		}
		if b.navigation[0] != r {
			t.Fatal("local subset was reordered")
		}
	}
	if !strings.Contains(listSearchNotice(screenList), "作成者が設定した既定の順序") {
		t.Fatal("default order not explained")
	}
}
