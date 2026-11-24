package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStorage(t *testing.T) {
	p := filepath.Join(t.TempDir(), "library.json")
	l, err := loadLibrary(p)
	if err != nil || len(l.Pins) != 0 {
		t.Fatal(l, err)
	}
	v, _ := makeEntry(home+"watch/sm9?token=tracking", "テスト")
	s, _ := makeEntry(home+"tag/test?ref=tracking&sort=f", "タグ検索")
	l.Pins = []entry{s, s}
	l.History = []entry{v, s}
	if err := saveLibrary(p, l); err != nil {
		t.Fatal(err)
	}
	got, err := loadLibrary(p)
	if err != nil || len(got.Pins) != 1 || got.Pins[0] != s || len(got.History) != 1 || got.History[0] != v {
		t.Fatal(got, err)
	}
	if err := saveLibrary(p, library{Pins: []entry{v}}); err != nil {
		t.Fatal(err)
	}
	if got, err := loadLibrary(p); err != nil || len(got.Pins) != 1 || got.Pins[0] != v {
		t.Fatal(got, err)
	}
	if err := os.WriteFile(p, []byte("null"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadLibrary(p); err == nil {
		t.Fatal("invalid root accepted")
	}
	res, err := os.ReadFile(p)
	if err != nil || string(res) != "null" {
		t.Fatal("invalid file was overwritten")
	}
}

func TestPutFirst(t *testing.T) {
	a := entry{URL: "a"}
	b := entry{URL: "b"}
	c := entry{URL: "c"}
	for _, tc := range []struct {
		existing []entry
		add      entry
		cap      int
		want     []entry
	}{
		{[]entry{a, b}, b, 2, []entry{b, a}},
		{[]entry{a, b}, c, 2, []entry{c, a}},
		{nil, a, 2, []entry{a}},
	} {
		got := putFirst(tc.existing, tc.add, tc.cap)
		if len(got) != len(tc.want) {
			t.Fatal(got)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatal(got)
			}
		}
	}
}
