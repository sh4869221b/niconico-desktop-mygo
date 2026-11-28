package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
)

const maxCommentNGEntries = 2048
const maxCommentNGBytes = 256 << 10

var anonymousCommentID = regexp.MustCompile(`^[A-Za-z0-9_-]{27}$`)

type commentAuthor struct {
	ID    string `json:"id"`
	Video string `json:"video,omitempty"`
}

func (a commentAuthor) valid() bool {
	if a.Video == "" {
		return numeric(a.ID, 20) && a.ID[0] != '0'
	}
	return videoID.MatchString(a.Video) && watchSegment.MatchString(a.Video) &&
		anonymousCommentID.MatchString(strings.TrimPrefix(a.ID, "nvc:"))
}

func authorForComment(video string, raw json.RawMessage, commands []string) commentAuthor {
	var id string
	// Unknown or absent userId values must not affect other comments in the response.
	if json.Unmarshal(raw, &id) != nil || id == "" {
		return commentAuthor{}
	}
	a := commentAuthor{ID: id}
	// The current API supplies decimal public IDs and two forms of opaque
	// 184 IDs. Store the exact opaque token with its video's canonical ID.
	if strings.HasPrefix(id, "nvc:") || slices.Contains(commands, "184") {
		if video == "" {
			return commentAuthor{}
		}
		a.Video = video
	}
	if !a.valid() {
		return commentAuthor{}
	}
	return a
}

type commentNG struct {
	path    string
	blocked map[commentAuthor]bool
	entries []commentAuthor
}

func loadCommentNG(path string) (commentNG, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return commentNG{path: path}, nil
	}
	if err != nil {
		return commentNG{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxCommentNGBytes+1))
	var entries []commentAuthor
	if err != nil || len(data) > maxCommentNGBytes || json.Unmarshal(data, &entries) != nil ||
		entries == nil || len(entries) > maxCommentNGEntries {
		return commentNG{}, errors.New("invalid comment NG settings")
	}
	blocked := make(map[commentAuthor]bool, len(entries))
	for _, a := range entries {
		if !a.valid() {
			return commentNG{}, errors.New("invalid comment author in NG settings")
		}
		blocked[a] = true
	}
	return commentNG{path, blocked, sortedCommentAuthors(blocked)}, nil
}

func sortedCommentAuthors(blocked map[commentAuthor]bool) []commentAuthor {
	out := make([]commentAuthor, 0, len(blocked))
	for a := range blocked {
		out = append(out, a)
	}
	slices.SortFunc(out, func(a, b commentAuthor) int {
		if n := strings.Compare(a.Video, b.Video); n != 0 {
			return n
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

func (ng *commentNG) change(a commentAuthor, block bool) error {
	if ng.path == "" || !a.valid() {
		return errors.New("invalid comment NG change")
	}
	return withSettingsLock(ng.path, func() error {
		latest, err := loadCommentNG(ng.path)
		if err != nil {
			return err
		}
		if latest.blocked[a] != block {
			if block {
				if len(latest.blocked) >= maxCommentNGEntries {
					return errors.New("comment NG limit reached")
				}
				if latest.blocked == nil {
					latest.blocked = make(map[commentAuthor]bool)
				}
				latest.blocked[a] = true
			} else {
				delete(latest.blocked, a)
			}
			latest.entries = sortedCommentAuthors(latest.blocked)
			data, err := json.Marshal(latest.entries)
			if err != nil {
				return err
			}
			if err := writeSettings(ng.path, data); err != nil {
				return err
			}
		}
		*ng = latest
		return nil
	})
}
