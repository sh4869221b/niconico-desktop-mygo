package main

import "github.com/egoist/mygo/ui"

type catalogVisit struct {
	page                      catalogPage
	hasPage                   bool
	list                      ui.ListState
	focusLink                 string
	restoreFocus, revealFocus bool
	keyedItems                []card
	rowKeys                   []catalogRowKey
}

func (b *browser) nativeVisit() *catalogVisit {
	if missing := len(b.navigation) - len(b.nativeVisits); missing > 0 {
		b.nativeVisits = append(b.nativeVisits, make([]*catalogVisit, missing)...)
	}
	if b.nativeVisits[b.index] == nil {
		b.nativeVisits[b.index] = &catalogVisit{}
	}
	return b.nativeVisits[b.index]
}

func (b *browser) presentNativeResult(p catalogPage, err error) {
	b.result, b.resultErr, b.busy, b.status = p, err, false, p.searchMode
	if err != nil {
		return
	}
	v := b.nativeVisit()
	if len(p.items) == 0 {
		v.dropPage()
		return
	}
	v.page, v.hasPage = p, true
	// Keep nearby pages and their measured row heights. Older history
	// entries keep the selected video but reload the list when revisited.
	const radius = 4
	for i, visit := range b.nativeVisits {
		if visit != nil && visit.hasPage && (i < b.index-radius || i > b.index+radius) {
			visit.dropPage()
		}
	}
}

func (v *catalogVisit) dropPage() {
	v.page, v.hasPage, v.list = catalogPage{}, false, ui.ListState{}
	v.keyedItems, v.rowKeys = nil, nil
	v.revealFocus = true
}

type catalogRowKey struct {
	link       string
	occurrence int
}

func (v *catalogVisit) keysFor(items []card, introRows int) []catalogRowKey {
	// Rows in a catalog response are immutable for its lifetime.
	if len(items) > 0 && len(v.rowKeys) == len(items)+introRows && len(v.keyedItems) == len(items) && &v.keyedItems[0] == &items[0] {
		return v.rowKeys
	}
	seen := make(map[string]int, len(items))
	keys := make([]catalogRowKey, len(items)+introRows)
	for i, item := range items {
		seen[item.link]++
		keys[i+introRows] = catalogRowKey{item.link, seen[item.link]}
	}
	v.keyedItems, v.rowKeys = items, keys
	return keys
}
