package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func readSessionCookie(list func(mygo.CookieFilter) ([]mygo.Cookie, error), target string, now time.Time) (*http.Cookie, error) {
	u, err := url.Parse(target)
	if err != nil || !sessionEndpoint(u) {
		return nil, errSignIn
	}
	values, err := list(mygo.CookieFilter{URL: target, Name: "user_session"})
	if err != nil {
		return nil, errors.New("ログイン情報を読み取れませんでした")
	}
	for _, value := range values {
		if value.Name != "user_session" || !value.Secure ||
			value.Domain != ".nicovideo.jp" && value.Domain != apiHost && value.Domain != "."+apiHost ||
			!value.Expires.IsZero() && !value.Expires.After(now) {
			continue
		}
		c := &http.Cookie{Name: value.Name, Value: value.Value, Secure: true, HttpOnly: true}
		if c.Value != "" && c.Valid() == nil {
			return c, nil
		}
	}
	return nil, errSignIn
}

func (b *browser) navigate(link string) {
	if allowedURL(link) == nil {
		return
	}
	r := routeFor(link)
	if r.kind == screenOfficial {
		b.openOfficial(link)
		return
	}
	b.openNative(b.searchDefault.apply(r), true)
}

func (b *browser) openNative(r screenRoute, add bool) {
	b.showSearchRoute(r)
	if b.official {
		b.closeOfficial()
	}
	if add && (b.index == -1 || b.navigation[b.index] != r) {
		if b.index >= 0 {
			b.nativeVisit()
		}
		b.navigation = append(b.navigation[:b.index+1], r)
		clear(b.nativeVisits[b.index+1:])
		b.nativeVisits = append(b.nativeVisits[:b.index+1], nil)
		b.index++
		if len(b.navigation) > 100 {
			b.navigation = b.navigation[1:]
			b.nativeVisits[0] = nil
			b.nativeVisits = b.nativeVisits[1:]
			b.index--
		}
	}
	b.refreshNative()
}

func (b *browser) moveNative(delta int) {
	next := b.index + delta
	if next >= 0 && next < len(b.navigation) {
		b.index = next
		b.showSearchRoute(b.navigation[next])
		visit := b.nativeVisit()
		visit.restoreFocus = visit.focusLink != ""
		if visit.hasPage {
			b.cancelLoad()
			b.result, b.resultErr, b.busy, b.status = visit.page, nil, false, visit.page.searchMode
			if b.win != nil {
				b.win.Invalidate()
			}
			return
		}
		b.refreshNative()
	}
}

func (b *browser) pageNative(delta int) {
	if b.index < 0 {
		return
	}
	r := b.navigation[b.index]
	if page := r.page + delta; page >= 0 && page <= maxCatalogPage {
		b.openNative(r.withPage(page), true)
	}
}

func (b *browser) cancelLoad() {
	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
	}
	if b.thumbnails != nil {
		b.thumbnails.setVisible(nil)
	}
	b.epoch++
}

func (b *browser) refreshNative() {
	b.cancelLoad()
	if b.index < 0 || b.closed || b.official {
		return
	}
	r, epoch := b.navigation[b.index], b.epoch
	b.result, b.resultErr, b.busy, b.status = catalogPage{}, nil, r.kind != screenHome, ""
	if !b.busy {
		b.win.Invalidate()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	go func() {
		defer cancel()
		p, err := b.catalog.load(ctx, r)
		mygo.RunOnMain(func() {
			if b.closed || b.official || b.epoch != epoch {
				return
			}
			b.applyCatalogContext(r, &p)
			b.presentNativeResult(p, err)
			b.win.Invalidate()
		})
	}()
	b.win.Invalidate()
}

// The main WebView handles browsing and login. The playback window owns watch pages.
func (b *browser) playOfficialLink(view *mygo.WebView, raw string, alreadyNavigated bool) bool {
	r := routeFor(raw)
	if r.kind != screenVideo {
		return false
	}
	go mygo.RunOnMain(func() {
		if b.closed || !b.official || b.web != view {
			return
		}
		if alreadyNavigated {
			if view.Page().URL() != raw {
				return
			}
			// A within-document or history navigation can bypass OnWillNavigate.
			// Destroy the view before starting playback so the main window cannot keep playing.
			target := b.source
			if allowedURL(target) == nil {
				target = b.officialTarget
			}
			b.web = nil
			view.Destroy()
			b.openOfficial(target)
		}
		b.openPlayer(r.target)
	})
	return true
}

func (b *browser) openOfficial(target string) {
	if allowedURL(target) == nil || b.closed {
		return
	}
	if routeFor(target).kind == screenVideo {
		b.openPlayer(target)
		return
	}
	b.cancelLoad()
	b.showSearchRoute(routeFor(target))
	b.official, b.officialTarget = true, target
	b.loading()
	b.win.Invalidate()
	go mygo.RunOnMain(func() {
		if b.closed || !b.official || b.officialTarget != target {
			return
		}
		if b.web == nil {
			b.attachOfficial()
		} else {
			b.loadOfficial(target)
		}
	})
}

func (b *browser) closeOfficial() {
	view := b.web
	b.web, b.official, b.loaded = nil, false, false
	b.source, b.status = "", ""
	if view != nil {
		// Destroy the previous view rather than leaving hidden playback running.
		go mygo.RunOnMain(view.Destroy)
	}
	b.checkLogin()
}

func (b *browser) leaveOfficial() {
	b.closeOfficial()
	if b.index >= 0 {
		b.showSearchRoute(b.navigation[b.index])
	}
	b.refreshNative()
}

func (b *browser) drawNative(c *ui.Context) {
	t := c.Theme()
	if b.index < 0 {
		return
	}
	r, p := b.navigation[b.index], b.result
	epoch := b.epoch
	// Keep the controls in the same focus scope while changing sort or page.
	scope := c.Key(r.kind).Key(r.key)
	ui.Column(scope).Bind(&b.contentRegion).Grow(1).MinWidth(0).Padding(14, 16).Gap(10).Children(func() {
		if r.kind == screenHome {
			ui.Text(scope, "ニコニコ動画").FontSize(29).Bold().Focusable().Bind(&b.contentFocus)
			ui.Text(scope, "ログインせずに動画を検索できます。上の入力欄にキーワードやタグを入力してください。").Wrap()
			if ui.PrimaryButton(scope, "例：初音ミクを検索欄に入れる").Clicked() {
				b.searchInput.field, b.focusSearch = "初音ミク", true
			}
			ui.Text(scope, "結果の画像は再生、タイトルは詳細を開きます。気に入ったページは「☆ 登録」で保存できます。").Wrap()
			ui.Row(scope).Gap(8).Wrap().Children(func() {
				if ui.Button(scope, "自分のマイリスト").Clicked() {
					b.navigate(mylists)
				}
				if ui.Button(scope, "ニコニコ公式トップ").Clicked() {
					b.openOfficial(home)
				}
			})
			return
		}
		title := p.title
		if title == "" {
			title = r.key
		}
		if title == "" {
			title = "マイリスト"
		}
		if r.kind == screenSearch {
			_, label := canonical(r.link)
			ui.Text(scope, label).TextColor(t.TextMuted)
		}
		ui.Text(scope, title).FontSize(24).Bold().Wrap().Focusable().Bind(&b.contentFocus)
		ui.Row(scope).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
			switch r.kind {
			case screenSearch:
				b.drawSearchSort(scope, r)
			case screenVideo:
				if ui.PrimaryButton(scope, "再生").Clicked() {
					b.openPlayer(r.target)
				}
			case screenUser:
				if ui.Button(scope, "公開マイリスト").Clicked() {
					b.navigate(home + "user/" + r.key + "/mylist")
				}
				if ui.Button(scope, "シリーズ").Clicked() {
					b.navigate(home + "user/" + r.key + "/series")
				}
			case screenUserSeries:
				if ui.Button(scope, "投稿動画").Clicked() {
					b.navigate(home + "user/" + r.key + "/video")
				}
			case screenSeries:
				if target := p.publisher.seriesURL(); target != "" && ui.Button(scope, "投稿者のシリーズ").Clicked() {
					b.navigate(target)
				}
			case screenUserLists:
				if ui.Button(scope, "投稿動画").Clicked() {
					b.navigate(home + "user/" + r.key + "/video")
				}
			case screenOwnLists, screenOwnList:
				if errors.Is(b.resultErr, errSignIn) && ui.PrimaryButton(scope, "ニコニコにログイン").Clicked() {
					b.openOfficial(login)
				}
				if r.kind == screenOwnList && ui.Button(scope, "自分のマイリスト").Clicked() {
					b.navigate(mylists)
				}
			}
			officialLabel := "公式ページ"
			if r.kind == screenUser || r.kind == screenList || r.kind == screenOwnList {
				officialLabel = "公式ページで絞り込み・並べ替え"
			}
			if r.kind != screenVideo && ui.Button(scope, officialLabel).Clicked() {
				b.openOfficial(r.target)
			}
		})
		if notice := listSearchNotice(r.kind); notice != "" {
			ui.Text(scope, notice).TextColor(t.TextMuted).Wrap()
		}
		if r.page > 0 || p.more || r.kind == screenSearch {
			ui.Row(scope).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
				if ui.Button(scope, "最初のページ").Disabled(r.page == 0).Clicked() {
					b.openNative(r.withPage(0), true)
				}
				if ui.Button(scope, "前へ").Disabled(b.busy || r.page == 0).Clicked() {
					b.pageNative(-1)
				}
				ui.Text(scope, fmt.Sprintf("%d ページ", r.page+1))
				if r.kind == screenSearch {
					pageControls := ui.Row(scope.Key(r.page)).Gap(4)
					value := ui.Local(pageControls, "page number", func() string { return strconv.Itoa(r.page + 1) })
					pageControls.Children(func() {
						input := ui.TextInput(scope, value).Label("移動先ページ").Width(72)
						page, valid := p.searchPageNumber(*value)
						if ui.Button(scope, "指定ページへ").Disabled(b.busy || !valid).Clicked() || input.Submitted() && !b.busy && valid {
							b.openNative(r.withPage(page), true)
						}
					})
				}
				if ui.Button(scope, "次へ").Disabled(b.busy || !p.more || r.page == maxCatalogPage).Clicked() {
					b.pageNative(1)
				}
			})
		}
		if b.epoch != epoch {
			return
		}
		if r.kind == screenSearch && !b.busy && b.resultErr == nil {
			ui.Text(scope, p.searchRange(r)).TextColor(t.TextMuted).Wrap()
			ui.Text(scope, searchSourceNotice(p.searchMode)).TextColor(t.TextMuted).Wrap()
		}
		if b.busy {
			ui.Text(scope, "読み込み中…").TextColor(t.TextMuted)
			return
		}
		if b.resultErr != nil {
			if r.kind == screenSearch {
				ui.Text(scope, "同じ条件で「再試行」するか、「公式ページ」を開いてください。並べ替えを変更する場合は上の選択欄から選べます。").Wrap()
			}
			if (r.kind == screenOwnLists || r.kind == screenOwnList) && !errors.Is(b.resultErr, errSignIn) {
				if ui.Button(scope, "ログイン状態を再確認して再取得").Disabled(b.loginBusy).Clicked() {
					b.checkLogin()
				}
			}
			ui.Text(scope, b.resultErr.Error()).Wrap()
			if !errors.Is(b.resultErr, errSignIn) && ui.Button(scope, "再試行").Clicked() {
				b.refreshNative()
			}
			return
		}
		if len(p.items) != 0 {
			introRows := 0
			if p.author != "" || p.summary != "" || len(p.links) > 0 {
				introRows = 1
			}
			visit := b.nativeVisit()
			listScope := scope.Key(visit)
			pane := ui.Column(listScope).Grow(1).MinHeight(0).MinWidth(0)
			state := &visit.list
			// Links identify a row even if a refresh puts other rows above it.
			// A duplicate link uses its occurrence, rather than sharing focus keys.
			keys := visit.keysFor(p.items, introRows)
			state.Key = func(i int) any { return keys[i] }
			if visit.restoreFocus {
				index := slices.IndexFunc(p.items, func(item card) bool { return item.link == visit.focusLink })
				if index < 0 {
					visit.restoreFocus = false
				} else if visit.revealFocus {
					state.ScrollIntoView(index + introRows)
				}
				visit.revealFocus = false
			}
			if b.thumbnails != nil {
				first, last := state.Visible()
				start := max(0, first-introRows)
				end := min(len(p.items), last+1-introRows)
				if start < end {
					b.thumbnails.setVisible(p.items[start:end])
				} else {
					b.thumbnails.setVisible(nil)
				}
			}
			pane.Children(func() {
				ui.List(listScope, state, len(p.items)+introRows, func(i int) {
					if i < introRows {
						ui.Column(listScope).FillWidth().Gap(8).Children(func() { b.drawCatalogIntro(listScope, p) })
						return
					}
					b.drawCatalogItem(listScope, r.kind, p.items[i-introRows], visit)
				}).Grow(1).MinHeight(0).Gap(6)
			})
			return
		}
		ui.Scroll(scope).Grow(1).MinHeight(0).Children(func() {
			ui.Column(scope).FillWidth().Gap(8).Children(func() {
				b.drawCatalogIntro(scope, p)
				if p.emptyNotice != "" {
					ui.Text(scope, p.emptyNotice).TextColor(t.TextMuted).Wrap()
				} else if r.kind == screenSearch {
					ui.Text(scope, "該当する動画はありません。").TextColor(t.TextMuted)
				} else if r.kind != screenVideo {
					ui.Text(scope, "表示する項目はありません。").TextColor(t.TextMuted)
				}
			})
		})
	})
}

func (b *browser) drawSearchSort(scope *ui.Context, r screenRoute) {
	current, ascending := r.searchSort()
	choice := current.label
	order := "降順"
	if ascending {
		order = "昇順"
	}
	ui.Text(scope, "並べ替え").TextColor(scope.Theme().TextMuted)
	changed := ui.Select(scope, &choice, searchSortLabels).Label("並べ替え").Width(160).Changed()
	if current.snapshotKey != "" {
		changed = ui.Select(scope, &order, []string{"降順", "昇順"}).Label("並び順").Width(74).Changed() || changed
	}
	if changed {
		for _, sort := range searchSorts {
			if sort.label == choice {
				b.rememberSearchSort(sort, order == "昇順")
				b.openNative(r.withSearchSort(sort, order == "昇順"), true)
				break
			}
		}
	}
	if current.urlKey == "f" || current.urlKey == "n" {
		direction := "新しい順"
		if ascending {
			direction = "古い順"
		}
		ui.Text(scope, direction).TextColor(scope.Theme().TextMuted)
	}
	if b.searchSortNotice != "" {
		ui.Text(scope, b.searchSortNotice).Wrap()
	}
}

func (b *browser) drawCatalogIntro(scope *ui.Context, p catalogPage) {
	if b.navigation[b.index].kind == screenVideo {
		ui.Text(scope, "タグ").Bold()
		b.drawVideoTags(scope, p.tags, "タグはありません。")
	}
	if p.author != "" {
		ui.Text(scope, p.author).TextColor(scope.Theme().TextMuted).Wrap()
	}
	if len(p.description) != 0 && p.summary != "" {
		b.drawDescription(scope, p.description)
	} else if p.summary != "" {
		ui.Text(scope, p.summary).Wrap()
	}
	for i, link := range p.links {
		if slices.ContainsFunc(p.description, func(part descriptionPart) bool { return part.text != "" && part.link.title == link.title }) {
			continue
		}
		row := scope.Key(fmt.Sprintf("link-%d", i))
		if ui.Button(row, link.title).Clicked() {
			b.followDescription(link.link)
		}
	}
}

func (b *browser) drawCatalogItem(scope *ui.Context, kind screenKind, item card, visit *catalogVisit) {
	t := scope.Theme()
	ui.Row(scope).FillWidth().AlignItems(ui.Center).Gap(10).Padding(6, 8).Radius(6).Background(t.Surface).Children(func() {
		b.drawThumbnail(scope.Key(item.link), item)
		ui.Column(scope).Grow(1).MinWidth(0).Gap(4).AlignItems(ui.Start).Children(func() {
			title := ui.Button(scope, "").Label(item.title).Tooltip(item.title).FillWidth().MinWidth(0).Justify(ui.Start).Children(func() {
				ui.Text(scope, item.title).Wrap().MaxLines(2)
			})
			if visit.restoreFocus && visit.focusLink == item.link {
				visit.restoreFocus = false
				title.Focus()
			}
			if title.Clicked() {
				visit.focusLink = item.link
				b.navigate(item.link)
			}
			if item.summary != "" {
				ui.Text(scope, strings.TrimSpace(item.summary)).TextColor(t.TextMuted).FontSize(12).Wrap().MaxLines(2)
			}
			if b.thumbnails != nil && !item.masked && b.thumbnails.failed(item.thumbnail) {
				if ui.Button(scope, "画像を再試行").Clicked() {
					b.thumbnails.retry(item.thumbnail)
				}
			}
			if kind == screenSearch {
				p := videoPublisher{item.ownerName, item.owner}
				if p.videos != "" {
					name := p.name
					if name == "" {
						name = "投稿者"
					}
					if metadataButton(scope.Key("publisher"), name, p.linkLabel(), 240).Clicked() {
						b.browseInMain(p.videos)
					}
				} else if p.name != "" {
					ui.Text(scope, p.name).SingleLine().Ellipsis("…").MaxWidth(240).MinWidth(0).Focusable().Tooltip(p.name)
				}
				if metric := item.searchMetric(b.navigation[b.index]); metric != "" {
					ui.Text(scope, metric).TextColor(t.TextMuted).FontSize(12).Wrap()
				}
				b.drawVideoTags(scope, item.tags, "タグ情報なし")
			}
		})
		if item.owner != "" && kind != screenSearch && kind != screenUser && ui.Button(scope, "投稿者").Clicked() {
			b.navigate(item.owner)
		}
	})
}

type thumbnailPress struct {
	x, y     float32
	canceled bool
}

func (press *thumbnailPress) input(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.Button == 0 {
			*press = thumbnailPress{x: ev.X, y: ev.Y, canceled: ev.Clicks > 1}
		}
	case ui.InputPointerMove, ui.InputPointerUp:
		if ev.Button == 0 {
			dx, dy := ev.X-press.x, ev.Y-press.y
			press.canceled = press.canceled || ev.Canceled || dx*dx+dy*dy > 36
		}
	case ui.InputScroll:
		press.canceled = true
	case ui.InputKeyDown:
		press.canceled = false
	}
	return false
}

func (b *browser) drawThumbnail(scope *ui.Context, item card) {
	width := float32(112)
	if w, _ := scope.Size(); w < 900 {
		width = 84
	}
	height := width * 9 / 16
	t := scope.Theme()
	r := routeFor(item.link)
	label := item.title + " を開く"
	if r.kind == screenVideo {
		label = item.title + " を再生"
	}
	button := ui.ButtonBase(scope).Size(width, height).Label(label).Tooltip(label).Radius(4).
		Background(t.Background).Border(1, t.Border).Cursor(ui.CursorPointer)
	press := ui.Local(button, "thumbnail press", func() thumbnailPress { return thumbnailPress{} })
	button.HandleInput(press.input).Children(func() {
		var img *ui.Bitmap
		if b.thumbnails != nil {
			img = b.thumbnails.image(item.thumbnail)
		}
		if img != nil {
			ui.Image(scope, img).Fill().Fit(ui.Contain).Radius(4).IgnoreInput(true).AccessHidden(true)
		} else {
			symbol, note := thumbnailFallback(item, b.thumbnails)
			ui.Column(scope).Fill().AlignItems(ui.Center).Justify(ui.Center).IgnoreInput(true).AccessHidden(true).Children(func() {
				ui.Text(scope, symbol).FontSize(18)
				ui.Text(scope, note).FontSize(10).TextColor(t.TextMuted)
			})
		}
	})
	if button.Clicked() && !press.canceled {
		if r.kind == screenVideo {
			b.openPlayer(r.target)
		} else {
			b.navigate(item.link)
		}
	}
}

func listSearchNotice(kind screenKind) string {
	switch kind {
	case screenUser:
		return "投稿日時の新しい順で、一覧全体をページごとに取得しています。名前の絞り込みや別の並び順は「公式ページで絞り込み・並べ替え」から選んでください。"
	case screenList:
		return "作成者が設定した既定の順序で表示しています。名前の絞り込みや別の並び順は「公式ページで絞り込み・並べ替え」から選んでください。"
	case screenOwnList:
		return "一覧の既定の順序で表示しています。名前の絞り込みや別の並び順は「公式ページで絞り込み・並べ替え」から選んでください。"
	}
	return ""
}

func searchSourceNotice(mode string) string {
	if mode == "スナップショット検索" {
		return "スナップショット検索に切り替えました。選択した並び順を保持しています。検索インデックスの更新には時差があり、最新の結果と異なる場合があります。更新時刻は取得できません。"
	}
	return mode + "の結果です。結果の更新時刻は取得できません。最新の状況は公式ページでも確認できます。"
}

// Keep the known author when moving between their collections.
func (b *browser) applyCatalogContext(r screenRoute, p *catalogPage) {
	if r.kind != screenUser && r.kind != screenUserLists {
		return
	}
	if p.publisher.name != "" {
		if b.userNames == nil {
			b.userNames = make(map[string]string)
		}
		b.userNames[r.key] = p.publisher.name
	}
	name := b.userNames[r.key]
	if name == "" {
		name = "ユーザー" + r.key
	}
	if r.kind == screenUserLists {
		p.title = name + " の公開マイリスト"
	} else {
		p.title = name + " の投稿動画"
	}
}
