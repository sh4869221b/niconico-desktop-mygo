package main

import (
	"strings"
	"unicode"
)

// The reading list has its own limit. Overlay layout keeps its smaller limit.
func cleanCommentListText(body string) string {
	body = strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n")
	var out []rune
	lines := 1
	for _, r := range body {
		if r == '\t' {
			r = '　'
		} else if r != '\n' && unicode.IsControl(r) {
			continue
		}
		if len(out) >= 4096 || r == '\n' && lines >= 64 {
			return strings.TrimRight(string(out), "\n") + "…"
		}
		if r == '\n' {
			lines++
		}
		out = append(out, r)
	}
	if strings.TrimSpace(string(out)) == "" {
		return ""
	}
	return string(out)
}
