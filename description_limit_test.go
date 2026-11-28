package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"strings"
	"testing"
)

func TestDescriptionLinkLimitExplained(t *testing.T) {
	var text strings.Builder
	for i := 1; i <= 25; i++ {
		fmt.Fprintf(&text, "sm%d ", i)
	}
	_, links, parts := plainDescription(text.String())
	if len(links) != 24 {
		t.Fatal("initial unique link limit changed")
	}
	b := &browser{closed: true, navigation: []screenRoute{routeFor(home + "watch/sm9")}}
	v := ui.NewTester(func(c *ui.Context) { b.drawDescription(c, parts) }, 800, 600)
	if !v.HasText("一部のリンクを省略しています") {
		t.Fatal("omission is silent")
	}
	if err := v.Click("公式プレーヤーで概要を確認"); err != nil {
		t.Fatal(err)
	}
	if b.playbackRequest != 1 {
		t.Fatal("official overview entry did not open player")
	}
}

func TestDescriptionOccurrenceLimitAndNoFalseNotice(t *testing.T) {
	for _, n := range []int{64, 65, 1000} {
		_, links, parts := plainDescription(strings.Repeat("sm9 ", n))
		count, omitted := 0, false
		for _, part := range parts {
			if part.link.link != "" {
				count++
			}
			omitted = omitted || part.omitted
		}
		if len(links) != 1 || count != min(n, 64) || omitted != (n > 64) {
			t.Fatalf("limit mismatch n=%d count=%d omitted=%v", n, count, omitted)
		}
	}
	_, _, parts := plainDescription("sm9 sm10")
	for _, part := range parts {
		if part.omitted {
			t.Fatal("short description incorrectly marked")
		}
	}
}
