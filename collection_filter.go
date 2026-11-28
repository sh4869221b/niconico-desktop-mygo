package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

var errIncompleteCollection = errors.New("一覧全体を確認できなかったため、絞り込み結果を表示できません。再試行するか、公式ページを開いてください")

func (c catalog) filterCollection(ctx context.Context, r screenRoute) (catalogPage, error) {
	const maxPages = 100
	query := strings.ToLower(r.collectionQuery)
	var result catalogPage
	var matches []card
	var total *int
	loaded := 0
	for page := range maxPages {
		if err := ctx.Err(); err != nil {
			return catalogPage{}, err
		}
		p, err := c.list(ctx, r.withPage(page))
		if err != nil {
			return catalogPage{}, err
		}
		if page == 0 {
			result, total = p, p.total
		} else if (p.total == nil) != (total == nil) || total != nil && *p.total != *total {
			return catalogPage{}, errIncompleteCollection
		}
		loaded += p.rawCount
		if !p.collectionPaginationKnown || p.more && p.rawCount == 0 || total != nil && loaded > *total {
			return catalogPage{}, errIncompleteCollection
		}
		for _, item := range p.items {
			if !item.masked && item.link != "" && strings.Contains(strings.ToLower(item.title), query) {
				matches = append(matches, item)
			}
		}
		if !p.more {
			if total != nil && loaded != *total {
				return catalogPage{}, errIncompleteCollection
			}
			result.items, result.more = matches, false
			result.emptyNotice = "条件に一致する動画はありません。"
			return result, nil
		}
	}
	return catalogPage{}, errIncompleteCollection
}

type collectionFilterKey struct {
	kind       screenKind
	key, query string
}

type collectionFilterControls struct {
	open, focusInput, focusButton bool
}

func (b *browser) drawCollectionFilter(c *ui.Context, r screenRoute, p catalogPage, state *collectionFilterControls) {
	controls := ui.Row(c.Key(collectionFilterKey{r.kind, r.key, r.collectionQuery})).Gap(8).Wrap().AlignItems(ui.Center)
	query := ui.Local(controls, "collection title", func() string { return r.collectionQuery })
	controls.Children(func() {
		input := ui.TextInput(c, query).Label("一覧全体の動画タイトル").Placeholder("動画タイトル").Width(230)
		if state.focusInput {
			state.focusInput = false
			input.Focus()
		}
		term := strings.TrimSpace(*query)
		canFilter := term != "" && term != r.collectionQuery
		if ui.Button(c, "絞り込む").Disabled(!canFilter).Clicked() || input.Submitted() && canFilter {
			next := r.withPage(0)
			next.collectionQuery = term
			b.openNative(next, true)
		}
		if r.collectionQuery != "" && ui.Button(c, "絞り込みを解除").Clicked() {
			next := r.withPage(0)
			next.collectionQuery = ""
			state.open, state.focusButton = false, true
			b.openNative(next, true)
		}
		if r.collectionQuery == "" && ui.Button(c, "閉じる").Label("絞り込み欄を閉じる").Clicked() {
			state.open, state.focusButton = false, true
		}
	})
	if r.collectionQuery != "" && !b.busy && b.resultErr == nil {
		ui.Text(c, fmt.Sprintf("一覧全体から %d 件", len(p.items))).Role(ui.RoleStatus)
	}
}
