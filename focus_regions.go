package main

import "github.com/egoist/mygo/ui"

func (b *browser) focusRegionShortcut(c *ui.Context) {
	back := c.Shortcut(ui.Shift, ui.KeyF6)
	if !back && !c.Shortcut(0, ui.KeyF6) {
		return
	}
	if b.official {
		b.searchFocus.Focus(c)
		return
	}
	if b.contentRegion.FocusWithin(c) {
		if back && b.showLibrary {
			b.libraryFocus.Focus(c)
		} else {
			b.searchFocus.Focus(c)
		}
	} else if b.showLibrary && b.libraryRegion.FocusWithin(c) {
		if back {
			b.searchFocus.Focus(c)
		} else {
			b.contentFocus.Focus(c)
		}
	} else if back || !b.showLibrary {
		b.contentFocus.Focus(c)
	} else {
		b.libraryFocus.Focus(c)
	}
}
