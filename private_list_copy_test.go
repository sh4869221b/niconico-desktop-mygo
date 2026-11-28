package main

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestPrivateListCopyConditionVisible(t *testing.T) {
	for _, raw := range []string{mylists + "#/123", home + "mylist/123"} {
		b := &browser{navigation: []screenRoute{routeFor(raw)}, closed: true}
		v := ui.NewTester(b.draw, 1000, 700)
		if !v.HasText("非公開のリストは外部ツールでは取得できません") {
			t.Fatal("copy condition missing for", raw)
		}
	}
	b := &browser{navigation: []screenRoute{routeFor(home + "user/123/video")}, closed: true}
	v := ui.NewTester(b.draw, 1000, 700)
	if v.HasText("非公開のリスト") {
		t.Fatal("mylist warning shown for user videos")
	}
}
