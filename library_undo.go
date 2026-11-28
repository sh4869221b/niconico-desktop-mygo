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
	history := b.history
	undo := &libraryRemoval{history: history}
	capture := true
	changed, saved := b.changeLibrary(func(data *library) bool {
		items := &data.Pins
		if history {
			items = &data.History
		}
		i := slices.IndexFunc(*items, func(x entry) bool { return x.URL == e.URL })
		if i < 0 {
			return false
		}
		if capture {
			undo.item, undo.index = (*items)[i], i
			undo.previous, undo.next = "", ""
			if i > 0 {
				undo.previous = (*items)[i-1].URL
			}
			if i+1 < len(*items) {
				undo.next = (*items)[i+1].URL
			}
		}
		*items = slices.Delete(*items, i, i+1)
		return true
	})
	// A delayed save may replay this edit without changing the undo record.
	capture = false
	if history && b.player != nil {
		p := b.player
		undo.player, undo.suppressed = p, p.historySuppressed[e.URL]
		if p.historySuppressed == nil {
			p.historySuppressed = make(map[string]bool)
		}
		p.historySuppressed[e.URL] = true
	}
	if !changed {
		return
	}
	b.undoRemoval = undo
	if saved {
		b.status = libraryDeleteLabel(undo.item, history) + "しました。"
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
	var full bool
	_, saved := b.changeLibrary(func(data *library) bool {
		items, limit := &data.Pins, 500
		if u.history {
			items, limit = &data.History, 100
		}
		if slices.ContainsFunc(*items, func(x entry) bool { return x.URL == u.item.URL }) {
			full = false
			return false
		}
		full = len(*items) >= limit
		if full {
			return false
		}
		i := len(*items)
		if u.next != "" {
			i = slices.IndexFunc(*items, func(x entry) bool { return x.URL == u.next })
			if i < 0 && u.previous != "" {
				if previous := slices.IndexFunc(*items, func(x entry) bool { return x.URL == u.previous }); previous >= 0 {
					i = previous + 1
				}
			}
			if i < 0 {
				i = min(u.index, len(*items))
			}
		}
		*items = slices.Insert(*items, i, u.item)
		return true
	})
	if full {
		if saved {
			b.status = "件数が上限のため元に戻せません。"
		}
		return
	}
	if u.history && b.player == u.player && u.player != nil && !u.suppressed {
		delete(u.player.historySuppressed, u.item.URL)
	}
	b.undoRemoval = nil
	if saved {
		b.status = u.item.Title + "を元に戻しました。"
	}
}
