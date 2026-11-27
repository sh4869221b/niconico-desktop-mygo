package main

import "slices"

type libraryRemoval struct {
	item       entry
	index      int
	history    bool
	next       string
	previous   string
	player     *playerWindow
	suppressed bool
}

func libraryDeleteLabel(e entry, history bool) string {
	kind := "登録"
	if history {
		kind = "視聴履歴"
	}
	return e.Title + "を" + kind + "から削除"
}

func (b *browser) remove(e entry) {
	if b.storeWarning != "" {
		b.status = b.storeWarning
		return
	}
	items := &b.data.Pins
	if b.history {
		items = &b.data.History
	}
	i := slices.IndexFunc(*items, func(x entry) bool { return x.URL == e.URL })
	if i < 0 {
		return
	}
	undo := &libraryRemoval{item: (*items)[i], index: i, history: b.history}
	if i > 0 {
		undo.previous = (*items)[i-1].URL
	}
	if i+1 < len(*items) {
		undo.next = (*items)[i+1].URL
	}
	if b.history && b.player != nil {
		p := b.player
		undo.player, undo.suppressed = p, p.historySuppressed[e.URL]
		if p.historySuppressed == nil {
			p.historySuppressed = make(map[string]bool)
		}
		p.historySuppressed[e.URL] = true
	}
	*items = slices.Delete(*items, i, i+1)
	b.undoRemoval = undo
	if b.persist() {
		b.status = libraryDeleteLabel(e, b.history) + "しました。"
	}
}

func (b *browser) undoLibraryRemoval() {
	u := b.undoRemoval
	if u == nil {
		return
	}
	if b.storeWarning != "" {
		b.status = b.storeWarning
		return
	}
	items, limit := &b.data.Pins, 500
	if u.history {
		items, limit = &b.data.History, 100
	}
	if !slices.ContainsFunc(*items, func(x entry) bool { return x.URL == u.item.URL }) {
		if len(*items) >= limit {
			b.status = "件数が上限のため元に戻せません。"
			return
		}
		i := slices.IndexFunc(*items, func(x entry) bool { return x.URL == u.next })
		if i < 0 && u.previous != "" {
			if previous := slices.IndexFunc(*items, func(x entry) bool { return x.URL == u.previous }); previous >= 0 {
				i = previous + 1
			}
		}
		if i < 0 {
			i = min(u.index, len(*items))
		}
		*items = slices.Insert(*items, i, u.item)
	}
	if u.history && b.player == u.player && u.player != nil && !u.suppressed {
		delete(u.player.historySuppressed, u.item.URL)
	}
	b.undoRemoval = nil
	if b.persist() {
		b.status = u.item.Title + "を元に戻しました。"
	}
}
