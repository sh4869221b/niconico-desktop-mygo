package main

import "strings"

type searchInput struct {
	field, term, draft string
	active, byTag      bool
}

func (b *browser) showSearchRoute(r screenRoute) {
	input := &b.searchInput
	active := r.isSearchPage()
	byTag := active && strings.HasPrefix(r.link, home+"tag/")
	term := ""
	if active {
		term = r.key
	}
	if input.active == active && input.term == term && input.byTag == byTag {
		return
	}
	if !input.active || input.field != input.term {
		input.draft = input.field
	}
	input.active, input.byTag, input.term = active, byTag, term
	if active {
		input.field = term
	} else {
		input.field = input.draft
	}
}

func (b *browser) search(byTag bool) {
	input := &b.searchInput
	target := resolve(input.field, byTag)
	if byTag && input.active && input.byTag && input.field == input.term {
		// A tag opened from a video can contain significant spaces.
		target = exactSearchURL("tag", input.term)
	}
	if target == "" {
		b.status = "検索語・タグ・動画ID、またはニコニコ公式のHTTPS URLを入力してください。"
		return
	}
	input.draft, input.field = "", input.term
	b.navigate(target)
}
