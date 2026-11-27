package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

const maxLibraryBytes = 2_000_000

type entry struct {
	URL   string
	Title string
	Kind  string
}
type library struct {
	Pins    []entry
	History []entry
}

func makeEntry(raw, title string) (entry, bool) {
	u, k := canonical(raw)
	if u == "" {
		return entry{}, false
	}
	if routeFor(raw).officialBrowse {
		// Preserve the conditions that require an official browse page.
		u = raw
	}
	title = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, title))
	if title == "" {
		title = u
	}
	if runes := []rune(title); len(runes) > 160 {
		title = string(runes[:160])
	}
	return entry{URL: u, Title: title, Kind: k}, true
}
func putFirst(xs []entry, x entry, limit int) []entry {
	xs = slices.DeleteFunc(xs, func(e entry) bool { return e.URL == x.URL })
	return slices.Insert(xs, 0, x)[:min(len(xs)+1, limit)]
}
func normalize(xs []entry, limit int, videosOnly bool) []entry {
	seen := make(map[string]bool)
	out := make([]entry, 0, min(len(xs), limit))
	for _, e := range xs {
		e, ok := makeEntry(e.URL, e.Title)
		if !ok || seen[e.URL] || videosOnly && e.Kind != "動画" {
			continue
		}
		seen[e.URL] = true
		out = append(out, e)
		if len(out) == limit {
			break
		}
	}
	return out
}
func loadLibrary(path string) (library, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return library{}, nil
	}
	if err != nil {
		return library{}, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxLibraryBytes+1))
	if err != nil {
		return library{}, err
	}
	if len(raw) > maxLibraryBytes {
		return library{}, errors.New("library exceeds size limit")
	}
	var l *library
	if err = json.Unmarshal(raw, &l); err != nil {
		return library{}, err
	}
	if l == nil {
		return library{}, errors.New("library is not an object")
	}
	l.Pins = normalize(l.Pins, 500, false)
	l.History = normalize(l.History, 100, true)
	return *l, nil
}
func saveLibrary(path string, l library) error {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > maxLibraryBytes {
		return errors.New("library exceeds size limit")
	}
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".library-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// Only replace generated placeholders; meaningful saved names belong to the user.
func (l *library) fillPinTitle(raw, title string) bool {
	e, ok := makeEntry(raw, title)
	if !ok || e.Title == e.URL {
		return false
	}
	for i := range l.Pins {
		pin := &l.Pins[i]
		if pin.URL == e.URL && pin.Title == pin.URL && pin.Title != e.Title {
			pin.Title = e.Title
			return true
		}
	}
	return false
}
