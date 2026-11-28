package main

import "strings"

var libraryKinds = []string{"すべての種類", "動画", "キーワード検索", "タグ検索", "マイリスト", "ユーザー", "シリーズ"}

type libraryResultKey struct {
	history     bool
	query, kind string
}

func normalizeLibraryQuery(query string) string {
	return strings.ToLower(strings.TrimSpace(query))
}

func filterLibrary(items []entry, query, kind string) []entry {
	query = normalizeLibraryQuery(query)
	out := make([]entry, 0, len(items))
	for _, item := range items {
		if kind != "" && kind != "すべての種類" && item.Kind != kind {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(item.Title+"\n"+item.URL), query) {
			continue
		}
		out = append(out, item)
	}
	return out
}
func libraryEntryLabel(item entry, history bool) string {
	unit := item.Kind + "の登録"
	name := item.Title
	if history {
		unit = "動画の視聴履歴"
	} else if item.hasPlaceholderTitle() {
		name = "仮の名前: " + name
	}
	return "[" + unit + "] " + name
}
