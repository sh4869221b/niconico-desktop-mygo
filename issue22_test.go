package main

import (
	"path/filepath"
	"testing"
)

func TestFillPinTitle(t *testing.T) {
	raw := home + "watch/sm9"
	for _, old := range []string{raw, "sm9", "自分の名前"} {
		e, _ := makeEntry(raw, old)
		l := library{Pins: []entry{e}}
		changed := l.fillPinTitle(raw, "取得した作品名")
		if changed != (old == raw) {
			t.Fatalf("old %q changed %v", old, changed)
		}
		path := filepath.Join(t.TempDir(), "library.json")
		if err := saveLibrary(path, l); err != nil {
			t.Fatal(err)
		}
		got, err := loadLibrary(path)
		if err != nil {
			t.Fatal(err)
		}
		want := "取得した作品名"
		if old != raw {
			want = old
		}
		if got.Pins[0].Title != want {
			t.Fatal(got)
		}
	}
}
