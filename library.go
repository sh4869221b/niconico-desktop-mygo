package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const maxLibraryBytes = 2_000_000

type entry struct {
	URL    string
	Title  string
	Kind   string
	Target string `json:",omitempty"`
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
	e := entry{URL: u, Title: truncateEntryTitle(title), Kind: k}
	r := routeFor(raw)
	target, _ := url.Parse(u)
	if r.kind == screenSearch {
		// An explicit empty sort keeps the saved default ranking independent of later preferences.
		q := target.Query()
		if !q.Has("sort") {
			q.Set("sort", "")
		}
		target.RawQuery = q.Encode()
	}
	if r.kind == screenVideo && r.startTime() > 0 {
		q := target.Query()
		q.Set("from", strconv.FormatInt(int64(r.startTime()/time.Second), 10))
		target.RawQuery = q.Encode()
	} else if r.kind != screenOfficial && r.page > 0 {
		q := target.Query()
		q.Set("page", strconv.Itoa(r.page+1))
		target.RawQuery = q.Encode()
	}
	if target.String() != u {
		e.Target = target.String()
	}
	return e, true
}
func putFirst(xs []entry, x entry, limit int) []entry {
	xs = slices.DeleteFunc(xs, func(e entry) bool { return e.URL == x.URL })
	return slices.Insert(xs, 0, x)[:min(len(xs)+1, limit)]
}
func normalize(xs []entry, limit int, videosOnly bool) []entry {
	seen := make(map[string]bool)
	out := make([]entry, 0, min(len(xs), limit))
	for _, e := range xs {
		e, ok := makeEntry(e.openURL(), e.Title)
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

func (e entry) openURL() string {
	if e.Target != "" {
		// Persisted targets must still identify the same resource.
		identity, _ := canonical(e.Target)
		if identity == e.URL {
			return e.Target
		}
	}
	return e.URL
}

func (e entry) hasPlaceholderTitle() bool {
	return e.URL != "" && e.Title == truncateEntryTitle(e.URL)
}

func truncateEntryTitle(title string) string {
	const limit = 160
	if len(title) <= limit {
		return title
	}
	count := 0
	for i := range title {
		if count == limit {
			return title[:i]
		}
		count++
	}
	return title
}

func (l library) pinTitleUpdate(e entry) int {
	if e.hasPlaceholderTitle() {
		return -1
	}
	return slices.IndexFunc(l.Pins, func(pin entry) bool {
		if pin.URL != e.URL || pin.Title == e.Title || !pin.hasPlaceholderTitle() {
			return false
		}
		switch e.Kind {
		case "キーワード検索", "タグ検索":
			return routeFor(pin.openURL()).page == routeFor(e.openURL()).page
		default:
			return true
		}
	})
}

func (l *library) fillPinTitle(e entry) bool {
	i := l.pinTitleUpdate(e)
	if i < 0 {
		return false
	}
	l.Pins[i].Title = e.Title
	return true
}
